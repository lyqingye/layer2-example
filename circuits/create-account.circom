pragma circom 2.0.0;
include "circomlib/circuits/smt/smtprocessor.circom";
include "hash-state.circom";

template CreateAccount(nLevels) {
        signal input balance;
		signal input nonce;
		signal input ethAddr;
		signal input oldStateRoot;
		signal input siblings[nLevels+1];
		signal input isOld0;
		signal input oldKey;
		signal input oldValue;
		signal input newKey;

		signal output newRoot;
		signal output oldRoot;

		var i;

		nonce === 0;
		balance === 0;

		component newLeafHash = HashState();

		newLeafHash.balance <== balance;
		newLeafHash.nonce <== nonce;
		newLeafHash.ethAddr <== ethAddr;
		newLeafHash.idx <== newKey;

		component processor = SMTProcessor(nLevels+1);
		for (i = 0; i< nLevels + 1; i++) {
			processor.siblings[i] <== siblings[i];
		}

		processor.oldKey <== oldKey;
		processor.oldValue <== oldValue;
		processor.isOld0 <== isOld0;
		processor.newKey <== newKey;
		processor.newValue <== newLeafHash.out;
		processor.oldRoot <== oldStateRoot;
		processor.fnc[0] <== 1;
		processor.fnc[1] <== 0;

	  processor.newRoot ==> newRoot;
	  oldStateRoot ==> oldRoot;
}

component main = CreateAccount(10);
