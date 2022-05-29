pragma circom 2.0.0;
include "circomlib/circuits/bitify.circom";
template BalanceUpdater() {
    signal input oldBalanceSender;
    signal input oldBalanceReceiver;
    signal input amount;

    signal output newBalanceSender;
    signal output newBalanceReceiver;

    newBalanceSender <== oldBalanceSender - amount;
    newBalanceReceiver <== oldBalanceReceiver + amount;
}

component main = BalanceUpdater();
