pragma circom 2.0.0;
include "circomlib/circuits/poseidon.circom";

template HashState() {
		signal input idx;
		signal input nonce;
		signal input balance;
		signal input ethAddr;
		signal input ax;
		signal input ay;

		signal output out;

		component hash = Poseidon(6);
		
		hash.inputs[0] <== idx;
		hash.inputs[1] <== nonce;
		hash.inputs[2] <== balance;
		hash.inputs[3] <== ethAddr;
		hash.inputs[4] <== ax;
		hash.inputs[5] <== ay;

		hash.out ==> out;
}

// component main = HashState();
