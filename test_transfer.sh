#!/usr/bin/env bash

cd circuits && \
rm -rf transfer_js && \
rm -rf transfer-test && \
rm -rf transfer.r1cs && \
circom transfer.circom --r1cs --wasm && \
mkdir transfer-test && \
pwd && \
mv transfer_js/transfer.wasm transfer-test/ && \
mv transfer_js/generate_witness.js transfer-test/ && \
mv transfer_js/witness_calculator.js transfer-test/ && \
go run ../rollup.go && \
cd transfer-test && node generate_witness.js transfer.wasm input.json witness.wtns