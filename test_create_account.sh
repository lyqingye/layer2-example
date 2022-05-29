#!/usr/bin/env bash

cd circuits && \
rm -rf create-account_js && \
rm -rf create-account-test && \
rm -rf create-account.r1cs && \
circom create-account.circom --r1cs --wasm && \
mkdir create-account-test && \
pwd && \
mv create-account_js/create-account.wasm create-account-test/ && \
mv create-account_js/generate_witness.js create-account-test/ && \
mv create-account_js/witness_calculator.js create-account-test/ && \
go run ../rollup.go && \
cd create-account-test && node generate_witness.js create-account.wasm input.json witness.wtns