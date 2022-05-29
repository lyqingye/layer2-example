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
	Balance string `json:"balance"`
	Nonce   string `json:"nonce"`
	EthAddr string `json:"ethAddr"`
	// bjj 公钥
	Ax           string   `json:"ax"`
	Ay           string   `json:"ay"`
	OldStateRoot string   `json:"oldStateRoot"`
	Siblings     []string `json:"siblings"`
	IsOld0       string   `json:"isOld0"`
	OldKey       string   `json:"oldKey"`
	OldValue     string   `json:"oldValue"`
	NewKey       string   `json:"newKey"`

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
	accBigInts, err := acc.BigInts()
	if err != nil {
		return nil, err
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

	input.OldStateRoot = proof.OldRoot.String()
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
	input.OldKey = proof.OldKey.String()
	input.OldValue = proof.OldValue.String()
	input.NewKey = proof.NewKey.String()

	// Hash交易参数,然后签名交易
	txBigInts := append(accBigInts[:], amount)
	hash, err := poseidon.Hash(txBigInts)
	sign := pk.SignPoseidon(hash)
	input.S = sign.S.String()
	input.R8x = sign.R8.X.String()
	input.R8y = sign.R8.Y.String()

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
	err = ioutil.WriteFile("/home/lyqingye/GolandProjects/circom-example/circuits/create-account-test/input.json", inputBytes, 0777)
	if err != nil {
		panic(err)
	}
}
