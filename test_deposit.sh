#!/usr/bin/env bash

cd circuits && \
rm -rf deposit_js && \
rm -rf deposit-test && \
rm -rf deposit.r1cs && \
circom deposit.circom --r1cs --wasm && \
mkdir deposit-test && \
pwd && \
mv deposit_js/deposit.wasm deposit-test/ && \
mv deposit_js/generate_witness.js deposit-test/ && \
mv deposit_js/witness_calculator.js deposit-test/ && \
go run ../rollup.go && \
cd deposit-test && node generate_witness.js deposit.wasm input.json witness.wtns