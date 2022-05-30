pragma circom 2.0.0;
include "circomlib/circuits/smt/smtprocessor.circom";
include "hash-state.circom";
include "circomlib/circuits/eddsaposeidon.circom";

template Transfer(nLevels) {
    // 发送方账号基本信息
    signal input senderIdx;
    signal input senderBalance;
    signal input senderNonce;
    signal input senderEthAddr;

    // 发送方bjj 公钥
    signal input senderAx;
    signal input senderAy;

    // 转账金额
    signal input transferAmount;

    // 发送方默克尔证明
    signal input senderOldStateRoot;
    signal input senderSiblings[nLevels+1];
    signal input senderIsOld0;

    // 签名
    signal input senderS;
    signal input senderR8x;
    signal input senderR8y;

    // 接收方账号基本信息
    signal input receiverIdx;
    signal input receiverBalance;
    signal input receiverNonce;
    signal input receiverEthAddr;

    // 接收方bjj 公钥
    signal input receiverAx;
    signal input receiverAy;

    // 接收方默克尔证明
    signal input receiverSiblings[nLevels+1];
    signal input receiverIsOld0;

    // 电路的Output
    signal output newRoot;
    signal output oldRoot;
    signal output outTxHash;

    var i;
    signal remainder;

    // 计算旧状态
    component senderOldState = HashState();

    senderOldState.idx <== senderIdx;
    senderOldState.balance <== senderBalance;
    senderOldState.nonce <== senderNonce;
    senderOldState.ethAddr <== senderEthAddr;
    senderOldState.ax <== senderAx;
    senderOldState.ay <== senderAy;

    // 计算交易Hash
    component txHash = Poseidon(2);
    txHash.inputs[0] <== senderOldState.out;
    txHash.inputs[1] <== transferAmount;

    txHash.out ==> outTxHash;

    // 验签
    component signVerify = EdDSAPoseidonVerifier();
    signVerify.enabled <== 1;
    signVerify.Ax <== senderAx;
    signVerify.Ay <== senderAy;
    signVerify.S <== senderS;
    signVerify.R8x <== senderR8x;
    signVerify.R8y <== senderR8y;
    signVerify.M <== txHash.out;

    // 检查要转账的金额是否为 0
    component check_amount = IsZero();
    check_amount.in <== transferAmount;
    check_amount.out === 0;

    // 检查转账金额是否为正数
    component checkAmountBits = Num2Bits(256);
    checkAmountBits.in <== transferAmount;
    checkAmountBits.out[255] === 0;

    // 计算余额，并且进行溢出检查
    remainder <== senderBalance - transferAmount;
    component remainderBits = Num2Bits(256);
    remainderBits.in <== remainder;
    remainderBits.out[255] === 0;

    // 计算发送方新状态
    component senderNewState = HashState();

    senderNewState.idx <== senderIdx;
    senderNewState.balance <== remainder;
    senderNewState.nonce <== senderNonce + 1;
    senderNewState.ethAddr <== senderEthAddr;
    senderNewState.ax <== senderAx;
    senderNewState.ay <== senderAy;

    component senderProcessor = SMTProcessor(nLevels+1);
    for (i = 0; i< nLevels + 1; i++) {
        senderProcessor.siblings[i] <== senderSiblings[i];
    }

    senderProcessor.oldKey <== senderIdx;
    senderProcessor.oldValue <== senderOldState.out;
    senderProcessor.isOld0 <== senderIsOld0;
    senderProcessor.newKey <== senderIdx;
    senderProcessor.newValue <== senderNewState.out;
    senderProcessor.oldRoot <== senderOldStateRoot;
    senderProcessor.fnc[0] <== 0;
    senderProcessor.fnc[1] <== 1;

    // 计算接收方旧状态
    component receiverOldState = HashState();

    receiverOldState.idx <== receiverIdx;
    receiverOldState.balance <== receiverBalance;
    receiverOldState.nonce <== receiverNonce;
    receiverOldState.ethAddr <== receiverEthAddr;
    receiverOldState.ax <== receiverAx;
    receiverOldState.ay <== receiverAy;

    // 计算接收方新状态
    component receiverNewState = HashState();

    receiverNewState.idx <== receiverIdx;
    receiverNewState.balance <== receiverBalance + transferAmount;
    receiverNewState.nonce <== receiverNonce;
    receiverNewState.ethAddr <== receiverEthAddr;
    receiverNewState.ax <== receiverAx;
    receiverNewState.ay <== receiverAy;

    component receiverProcessor = SMTProcessor(nLevels+1);
    for (i = 0; i< nLevels + 1; i++) {
        receiverProcessor.siblings[i] <== receiverSiblings[i];
    }

    receiverProcessor.oldKey <== receiverIdx;
    receiverProcessor.oldValue <== receiverOldState.out;
    receiverProcessor.isOld0 <== receiverIsOld0;
    receiverProcessor.newKey <== receiverIdx;
    receiverProcessor.newValue <== receiverNewState.out;
    receiverProcessor.oldRoot <== senderProcessor.newRoot;
    receiverProcessor.fnc[0] <== 0;
    receiverProcessor.fnc[1] <== 1;

    receiverProcessor.newRoot ==> newRoot;
    senderOldStateRoot ==> oldRoot;
}

component main = Transfer(10);