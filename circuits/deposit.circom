pragma circom 2.0.0;
include "circomlib/circuits/smt/smtprocessor.circom";
include "hash-state.circom";
include "circomlib/circuits/eddsaposeidon.circom";

template Deposit(nLevels) {
    // 账号基本信息
    signal input idx;
    signal input balance;
    signal input amount;
    signal input nonce;
    signal input ethAddr;

    // bjj 公钥
    signal input ax;
    signal input ay;

    // 默克尔证明
    signal input oldStateRoot;
    signal input siblings[nLevels+1];
    signal input isOld0;

    // 签名
    signal input s;
    signal input r8x;
    signal input r8y;

    // 电路的Output
    signal output newRoot;
    signal output oldRoot;
    signal output outTxHash;

    var i;
    signal remainder;

    // 计算旧状态
    component oldState = HashState();

    oldState.idx <== idx;
    oldState.balance <== balance;
    oldState.nonce <== nonce;
    oldState.ethAddr <== ethAddr;
    oldState.ax <== ax;
    oldState.ay <== ay;

    // 计算交易Hash
    component txHash = Poseidon(2);
    txHash.inputs[0] <== oldState.out;
    txHash.inputs[1] <== amount;

    txHash.out ==> outTxHash;

    // 验签
    component signVerify = EdDSAPoseidonVerifier();
    signVerify.enabled <== 1;
    signVerify.Ax <== ax;
    signVerify.Ay <== ay;
    signVerify.S <== s;
    signVerify.R8x <== r8x;
    signVerify.R8y <== r8y;
    signVerify.M <== txHash.out;

    // 检查要充值的金额是否为 0
    component check_amount = IsZero();
    check_amount.in <== amount;
    check_amount.out === 0;

    // 检查转账金额是否为正数
    component checkAmountBits = Num2Bits(256);
    checkAmountBits.in <== amount;
    checkAmountBits.out[255] === 0;

    // 计算余额，并且进行溢出检查
    remainder <== balance + amount;
    component remainderBits = Num2Bits(256);
    remainderBits.in <== remainder;
    remainderBits.out[255] === 0;


    // 计算新状态
    component newState = HashState();

    newState.idx <== idx;
    newState.balance <== remainder;
    newState.nonce <== nonce + 1;
    newState.ethAddr <== ethAddr;
    newState.ax <== ax;
    newState.ay <== ay;

    component processor = SMTProcessor(nLevels+1);
    for (i = 0; i< nLevels + 1; i++) {
        processor.siblings[i] <== siblings[i];
    }

    processor.oldKey <== idx;
    processor.oldValue <== oldState.out;
    processor.isOld0 <== isOld0;
    processor.newKey <== idx;
    processor.newValue <== newState.out;
    processor.oldRoot <== oldStateRoot;
    processor.fnc[0] <== 0;
    processor.fnc[1] <== 1;

    processor.newRoot ==> newRoot;
    oldStateRoot ==> oldRoot;
}

component main = Deposit(10);