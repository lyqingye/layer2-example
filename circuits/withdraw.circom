pragma circom 2.0.0;
include "circomlib/circuits/smt/smtprocessor.circom";
include "hash-state.circom";

template Withdraw(nLevels) {
    signal input idx;
    signal input balance;
    signal input amount;
    signal input nonce;
    signal input ethAddr;
    signal input ax;
    signal input ay;
    signal input oldStateRoot;
    signal input siblings[nLevels+1];
    signal input isOld0;

    signal output newRoot;
    signal output oldRoot;

    var i;
    signal remainder;

    component is_zero = IsZero();
    is_zero.in <== balance;
    is_zero.out === 0;

    // TODO 验证签名

    // 检查余额是否为 0
    component check_balance = IsZero();
    check_balance.in <== balance;
    check_balance.out === 0;

    // 检查要转账的金额是否为 0
    component check_amount = IsZero();
    check_amount.in <== amount;
    check_amount.out === 0;

    // 计算余额，并且进行溢出检查
    remainder <== balance - amount;
    component remainderBits = Num2Bits(256);
    remainderBits.in <== remainder;
    remainderBits.out[255] === 0;

    // 计算旧状态
    component oldState = HashState();

    oldState.idx <== idx;
    oldState.balance <== balance;
    oldState.nonce <== nonce;
    oldState.ethAddr <== ethAddr;
    oldState.ax <== ax;
    oldState.ay <== ay;

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

component main = Withdraw(10);