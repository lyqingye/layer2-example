package main

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/iden3/go-iden3-crypto/babyjub"
	"github.com/iden3/go-iden3-crypto/poseidon"
	"github.com/iden3/go-merkletree"
	"github.com/iden3/go-merkletree/db"
	"github.com/iden3/go-merkletree/db/pebble"
	"io/ioutil"
	"math/big"
	"os"
)

var (
	KeyLatestIdx = []byte("l")
	PrefixAcc    = []byte("a")
)

const nLevels = 10

func AccountKey(idx Idx) []byte {
	var bytes = make([]byte, 9)
	binary.PutUvarint(bytes[1:], uint64(idx))
	bytes[0] = PrefixAcc[0]
	return bytes
}

type Idx uint64
type Nonce uint64

type Account struct {
	Idx     Idx
	EthAddr ethCommon.Address
	Nonce   Nonce
	Balance *big.Int
	Ax      *big.Int
	Ay      *big.Int
}

func (a *Account) Bytes() ([]byte, error) {
	return json.Marshal(a)
}

func AccountFromJsonBytes(bytes []byte) (*Account, error) {
	var acc = Account{}
	return &acc, json.Unmarshal(bytes, &acc)
}

func (a *Account) BigInts() ([6]*big.Int, error) {
	e := [6]*big.Int{}
	e[0] = big.NewInt(int64(a.Idx))
	e[1] = big.NewInt(int64(a.Nonce))
	e[2] = a.Balance
	e[3] = new(big.Int).SetBytes(a.EthAddr.Bytes())
	e[4] = a.Ax
	e[5] = a.Ay
	return e, nil
}

func (a *Account) HashValue() (*big.Int, error) {
	bigInts, err := a.BigInts()
	if err != nil {
		return nil, err
	}
	return poseidon.Hash(bigInts[:])
}

type StateDB struct {
	MT      *merkletree.MerkleTree
	Storage db.Storage
}

func LoadState() (*StateDB, error) {
	storage, err := pebble.NewPebbleStorage("state.db", false)
	if err != nil {
		return nil, err
	}
	mt, err := merkletree.NewMerkleTree(storage, nLevels)
	if err != nil {
		return nil, err
	}
	return &StateDB{
		MT:      mt,
		Storage: storage,
	}, nil
}

func (s *StateDB) LastAccountIdx() (Idx, error) {
	idx, err := s.Storage.Get(KeyLatestIdx)
	if err != nil {
		return 0, nil
	}
	i, _ := binary.Uvarint(idx)
	return Idx(i), nil
}

func (s *StateDB) SetLastAccountIdx(idx Idx) (db.Tx, error) {
	tx, err := s.Storage.NewTx()
	if err != nil {
		return nil, err
	}
	var bytes = make([]byte, 8)
	binary.PutUvarint(bytes, uint64(idx))
	err = tx.Put(KeyLatestIdx, bytes)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func (s *StateDB) CreateAccount(acc *Account) (Idx, *merkletree.CircomProcessorProof, error) {
	idx, err := s.LastAccountIdx()
	if err != nil {
		return 0, nil, err
	}
	nextIdx := idx + 1
	acc.Idx = nextIdx
	tx, err := s.SetLastAccountIdx(nextIdx)
	if err != nil {
		return 0, nil, err
	}
	bytes, err := acc.Bytes()
	if err != nil {
		return 0, nil, err
	}
	err = tx.Put(AccountKey(nextIdx), bytes)
	if err != nil {
		return 0, nil, err
	}
	err = tx.Commit()
	if err != nil {
		return 0, nil, err
	}
	hashBigInt, err := acc.HashValue()
	if err != nil {
		return 0, nil, err
	}
	proof, err := s.MT.AddAndGetCircomProof(big.NewInt(int64(nextIdx)), hashBigInt)
	if err != nil {
		return 0, nil, err
	}
	println(proof.OldRoot.BigInt().String())
	println(proof.NewRoot.BigInt().String())
	return nextIdx, proof, nil
}

func (s *StateDB) GetAccount(idx Idx) (*Account, error) {
	key := AccountKey(idx)
	accBytes, err := s.Storage.Get(key)
	if err != nil {
		return nil, err
	}
	return AccountFromJsonBytes(accBytes)
}

func (s *StateDB) UpdateAccount(acc *Account) (*merkletree.CircomProcessorProof, error) {
	key := AccountKey(acc.Idx)
	tx, err := s.Storage.NewTx()
	if err != nil {
		return nil, err
	}
	bytes, err := acc.Bytes()
	if err != nil {
		return nil, err
	}
	err = tx.Put(key, bytes)
	if err != nil {
		return nil, err
	}
	err = tx.Commit()
	if err != nil {
		return nil, err
	}
	hashBigInt, err := acc.HashValue()
	if err != nil {
		return nil, err
	}
	return s.MT.Update(big.NewInt(int64(acc.Idx)), hashBigInt)
}

type CreateAccountCircuitInput struct {
	Balance      string   `json:"balance"`
	Nonce        string   `json:"nonce"`
	EthAddr      string   `json:"ethAddr"`
	Ax           string   `json:"ax"`
	Ay           string   `json:"ay"`
	OldStateRoot string   `json:"oldStateRoot"`
	Siblings     []string `json:"siblings"`
	IsOld0       string   `json:"isOld0"`
	OldKey       string   `json:"oldKey"`
	OldValue     string   `json:"oldValue"`
	NewKey       string   `json:"newKey"`
}

func CreateAccountCircuitInputFromProof(acc *Account, proof *merkletree.CircomProcessorProof) *CreateAccountCircuitInput {
	var siblings []string
	for _, s := range proof.Siblings {
		siblings = append(siblings, s.BigInt().String())
	}
	if len(proof.Siblings) != nLevels+1 {
		panic("invalid siblings length")
	}
	input := CreateAccountCircuitInput{
		Balance:      acc.Balance.String(),
		Nonce:        big.NewInt(int64(acc.Nonce)).String(),
		EthAddr:      new(big.Int).SetBytes(acc.EthAddr.Bytes()).String(),
		OldStateRoot: proof.OldRoot.BigInt().String(),
		Siblings:     siblings,
		OldKey:       proof.OldKey.BigInt().String(),
		OldValue:     proof.OldValue.BigInt().String(),
		NewKey:       proof.NewKey.BigInt().String(),
		Ax:           acc.Ax.String(),
		Ay:           acc.Ay.String(),
	}
	if proof.IsOld0 {
		input.IsOld0 = big.NewInt(1).String()
	} else {
		input.IsOld0 = big.NewInt(0).String()
	}
	return &input
}

type WithdrawCircuitInput struct {
	Idx     string `json:"idx"`
	Balance string `json:"balance"`
	Amount  string `json:"amount"`
	Nonce   string `json:"nonce"`
	EthAddr string `json:"ethAddr"`
	// bjj 公钥
	Ax           string   `json:"ax"`
	Ay           string   `json:"ay"`
	OldStateRoot string   `json:"oldStateRoot"`
	Siblings     []string `json:"siblings"`
	IsOld0       string   `json:"isOld0"`

	// 签名
	S   string `json:"s"`
	R8x string `json:"r8x"`
	R8y string `json:"r8y"`
}

func Withdraw(state *StateDB, idx Idx, amount *big.Int, pk babyjub.PrivateKey) (*WithdrawCircuitInput, error) {
	acc, err := state.GetAccount(idx)
	if err != nil {
		return nil, err
	}
	if amount.Cmp(acc.Balance) == 1 {
		return nil, errors.New("insufficient balance")
	}
	input := WithdrawCircuitInput{
		// 记录余额更新前的账号信息
		Balance: acc.Balance.String(),
		Nonce:   big.NewInt(int64(acc.Nonce)).String(),
		EthAddr: new(big.Int).SetBytes(acc.EthAddr.Bytes()).String(),
		Ax:      acc.Ax.String(),
		Ay:      acc.Ay.String(),
	}
	// 更新余额度
	acc.Balance = acc.Balance.Sub(acc.Balance, amount)
	// 更新nonce
	acc.Nonce = acc.Nonce + 1
	// 更新账户状态，并且拿到proof
	proof, err := state.UpdateAccount(acc)
	if err != nil {
		return nil, err
	}
	input.OldStateRoot = proof.OldRoot.BigInt().String()
	var siblings []string
	for _, s := range proof.Siblings {
		siblings = append(siblings, s.BigInt().String())
	}
	if len(proof.Siblings) != nLevels+1 {
		panic("invalid siblings length")
	}
	input.Siblings = siblings
	if proof.IsOld0 {
		input.IsOld0 = big.NewInt(1).String()
	} else {
		input.IsOld0 = big.NewInt(0).String()
	}
	input.Idx = big.NewInt(int64(idx)).String()
	input.Amount = amount.String()

	// Hash交易参数,然后签名交易
	txBigInts := append([]*big.Int{proof.OldValue.BigInt()}, amount)
	hash, err := poseidon.Hash(txBigInts)
	if err != nil {
		return nil, err
	}
	sign := pk.SignPoseidon(hash)
	input.S = sign.S.String()
	input.R8x = sign.R8.X.String()
	input.R8y = sign.R8.Y.String()

	return &input, nil
}

type DepositCircuitInput WithdrawCircuitInput

func Deposit(state *StateDB, idx Idx, amount *big.Int, pk babyjub.PrivateKey) (*DepositCircuitInput, error) {
	acc, err := state.GetAccount(idx)
	if err != nil {
		return nil, err
	}
	if amount.Cmp(big.NewInt(0)) != 1 {
		return nil, errors.New("invalid amount")
	}
	input := DepositCircuitInput{
		// 记录余额更新前的账号信息
		Balance: acc.Balance.String(),
		Nonce:   big.NewInt(int64(acc.Nonce)).String(),
		EthAddr: new(big.Int).SetBytes(acc.EthAddr.Bytes()).String(),
		Ax:      acc.Ax.String(),
		Ay:      acc.Ay.String(),
	}
	// 更新余额度
	acc.Balance = acc.Balance.Add(acc.Balance, amount)
	// 更新nonce
	acc.Nonce = acc.Nonce + 1
	// 更新账户状态，并且拿到proof
	proof, err := state.UpdateAccount(acc)
	if err != nil {
		return nil, err
	}
	input.OldStateRoot = proof.OldRoot.BigInt().String()
	var siblings []string
	for _, s := range proof.Siblings {
		siblings = append(siblings, s.BigInt().String())
	}
	if len(proof.Siblings) != nLevels+1 {
		panic("invalid siblings length")
	}
	input.Siblings = siblings
	if proof.IsOld0 {
		input.IsOld0 = big.NewInt(1).String()
	} else {
		input.IsOld0 = big.NewInt(0).String()
	}
	input.Idx = big.NewInt(int64(idx)).String()
	input.Amount = amount.String()

	// Hash交易参数,然后签名交易
	txBigInts := append([]*big.Int{proof.OldValue.BigInt()}, amount)
	hash, err := poseidon.Hash(txBigInts)
	if err != nil {
		return nil, err
	}
	sign := pk.SignPoseidon(hash)
	input.S = sign.S.String()
	input.R8x = sign.R8.X.String()
	input.R8y = sign.R8.Y.String()

	return &input, nil
}

type TransferCircuitInput struct {
	SenderIdx     string `json:"senderIdx"`
	SenderBalance string `json:"senderBalance"`
	SenderNonce   string `json:"senderNonce"`
	SenderEthAddr string `json:"senderEthAddr"`
	// bjj 公钥
	SenderAx           string   `json:"senderAx"`
	SenderAy           string   `json:"senderAy"`
	SenderOldStateRoot string   `json:"senderOldStateRoot"`
	SenderSiblings     []string `json:"senderSiblings"`
	SenderIsOld0       string   `json:"senderIsOld0"`

	TransferAmount string `json:"transferAmount"`

	// 签名
	SenderS   string `json:"senderS"`
	SenderR8x string `json:"senderR8x"`
	SenderR8y string `json:"senderR8y"`

	ReceiverIdx     string `json:"receiverIdx"`
	ReceiverBalance string `json:"receiverBalance"`
	ReceiverNonce   string `json:"receiverNonce"`
	ReceiverEthAddr string `json:"receiverEthAddr"`
	// bjj 公钥
	ReceiverAx       string   `json:"receiverAx"`
	ReceiverAy       string   `json:"receiverAy"`
	ReceiverSiblings []string `json:"receiverSiblings"`
	ReceiverIsOld0   string   `json:"receiverIsOld0"`
}

func Transfer(state *StateDB, senderIdx Idx, amount *big.Int, pk babyjub.PrivateKey, receiverIdx Idx) (*TransferCircuitInput, error) {
	sender, err := state.GetAccount(senderIdx)
	if err != nil {
		return nil, err
	}
	receiver, err := state.GetAccount(receiverIdx)
	if err != nil {
		return nil, err
	}
	if amount.Cmp(sender.Balance) == 1 {
		return nil, errors.New("insufficient balance")
	}

	input := TransferCircuitInput{
		// 记录余额更新前的账号信息
		SenderIdx:     big.NewInt(int64(sender.Idx)).String(),
		SenderBalance: sender.Balance.String(),
		SenderNonce:   big.NewInt(int64(sender.Nonce)).String(),
		SenderEthAddr: new(big.Int).SetBytes(sender.EthAddr.Bytes()).String(),
		SenderAx:      sender.Ax.String(),
		SenderAy:      sender.Ay.String(),

		ReceiverIdx:     big.NewInt(int64(receiver.Idx)).String(),
		ReceiverBalance: receiver.Balance.String(),
		ReceiverNonce:   big.NewInt(int64(receiver.Nonce)).String(),
		ReceiverEthAddr: new(big.Int).SetBytes(receiver.EthAddr.Bytes()).String(),
		ReceiverAx:      receiver.Ax.String(),
		ReceiverAy:      receiver.Ay.String(),
		TransferAmount:  amount.String(),
	}

	sender.Balance = sender.Balance.Sub(sender.Balance, amount)
	sender.Nonce = sender.Nonce + 1
	receiver.Balance = receiver.Balance.Add(receiver.Balance, amount)

	senderProof, err := state.UpdateAccount(sender)
	if err != nil {
		return nil, err
	}

	var senderSiblings []string
	for _, s := range senderProof.Siblings {
		senderSiblings = append(senderSiblings, s.BigInt().String())
	}

	input.SenderSiblings = senderSiblings
	input.SenderOldStateRoot = senderProof.OldRoot.BigInt().String()

	if senderProof.IsOld0 {
		input.SenderIsOld0 = big.NewInt(1).String()
	} else {
		input.SenderIsOld0 = big.NewInt(0).String()
	}

	receiverProof, err := state.UpdateAccount(receiver)
	if err != nil {
		return nil, err
	}

	var receiverSiblings []string
	for _, s := range receiverProof.Siblings {
		receiverSiblings = append(receiverSiblings, s.BigInt().String())
	}
	input.ReceiverSiblings = receiverSiblings
	if receiverProof.IsOld0 {
		input.ReceiverIsOld0 = big.NewInt(1).String()
	} else {
		input.ReceiverIsOld0 = big.NewInt(0).String()
	}

	// Hash交易参数,然后签名交易
	txBigInts := append([]*big.Int{senderProof.OldValue.BigInt()}, amount)
	hash, err := poseidon.Hash(txBigInts)
	if err != nil {
		return nil, err
	}
	sign := pk.SignPoseidon(hash)
	input.SenderS = sign.S.String()
	input.SenderR8x = sign.R8.X.String()
	input.SenderR8y = sign.R8.Y.String()
	return &input, nil
}

func main() {
	state, err := LoadState()
	if err != nil {
		panic(err)
	}
	prikey := babyjub.NewRandPrivKey()
	pubkey := prikey.Public()
	acc := Account{
		Idx:     0,
		EthAddr: ethCommon.Address{},
		Nonce:   0,
		Balance: big.NewInt(0),
		Ax:      pubkey.X,
		Ay:      pubkey.Y,
	}
	_, proof, err := state.CreateAccount(&acc)
	if err != nil {
		panic(err)
	}
	input := CreateAccountCircuitInputFromProof(&acc, proof)
	inputBytes, err := json.Marshal(input)
	if err != nil {
		panic(err)
	}
	pwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	err = ioutil.WriteFile(pwd+"/create-account-test/input.json", inputBytes, 0777)
	if err != nil {
		panic(err)
	}

	depositInput, err := Deposit(state, acc.Idx, big.NewInt(100), prikey)
	if err != nil {
		panic(err)
	}
	depositInputBytes, err := json.Marshal(depositInput)
	if err != nil {
		panic(err)
	}
	err = ioutil.WriteFile(pwd+"/deposit-test/input.json", depositInputBytes, 0777)
	if err != nil {
		panic(err)
	}

	withdrawInput, err := Withdraw(state, acc.Idx, big.NewInt(1), prikey)
	if err != nil {
		panic(err)
	}
	withdrawInputBytes, err := json.Marshal(withdrawInput)
	if err != nil {
		panic(err)
	}
	err = ioutil.WriteFile(pwd+"/withdraw-test/input.json", withdrawInputBytes, 0777)
	if err != nil {
		panic(err)
	}

	transferInput, err := Transfer(state, acc.Idx, big.NewInt(1), prikey, acc.Idx-1)
	if err != nil {
		panic(err)
	}

	transferInputBytes, err := json.Marshal(transferInput)
	if err != nil {
		panic(err)
	}

	err = ioutil.WriteFile(pwd+"/transfer-test/input.json", transferInputBytes, 0777)
	if err != nil {
		panic(err)
	}
}
