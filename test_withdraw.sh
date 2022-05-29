#!/usr/bin/env bash

cd circuits && \
rm -rf withdraw_js && \
rm -rf withdraw-test && \
rm -rf withdraw.r1cs && \
circom withdraw.circom --r1cs --wasm && \
mkdir withdraw-test && \
pwd && \
mv withdraw_js/withdraw.wasm withdraw-test/ && \
mv withdraw_js/generate_witness.js withdraw-test/ && \
mv withdraw_js/witness_calculator.js withdraw-test/ && \
go run ../rollup.go && \
cd withdraw-test && node generate_witness.js withdraw.wasm input.json witness.wtns