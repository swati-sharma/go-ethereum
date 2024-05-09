#!/bin/bash

# Download .so files
wget https://github.com/scroll-tech/da-codec/releases/download/v0.0.0-rc0-ubuntu20.04/libzktrie.so
wget https://github.com/scroll-tech/da-codec/releases/download/v0.0.0-rc0-ubuntu20.04/libscroll_zstd.so

# Set the environment variable
export LD_LIBRARY_PATH=$LD_LIBRARY_PATH:$(pwd)
export CGO_LDFLAGS="-L$(pwd) -lscroll_zstd -lzktrie"

# Download and install the project dependencies
go run build/ci.go install
go get ./...

# Save the root directory of the project
ROOT_DIR=$(pwd)

# Run genesis test
cd $ROOT_DIR/cmd/geth
go test -test.run TestCustomGenesis

# Run module tests
cd $ROOT_DIR
go run build/ci.go test ./consensus ./core ./eth ./miner ./node ./trie ./rollup/rollup_sync_service
