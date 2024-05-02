#!/bin/bash

# Save the root directory of the project
ROOT_DIR=$(pwd)

# Set the environment variable
export LD_LIBRARY_PATH=$ROOT_DIR/rollup/rollup_sync_service/libzstd:$LD_LIBRARY_PATH

# Compile libzstd
cd $ROOT_DIR/rollup/rollup_sync_service/libzstd
make libzstd

# Run genesis test
cd $ROOT_DIR/cmd/geth
go test -test.run TestCustomGenesis

# Run module tests
cd $ROOT_DIR
env GO111MODULE=on go run build/ci.go test ./consensus ./core ./eth ./miner ./node ./trie ./rollup/rollup_sync_service
