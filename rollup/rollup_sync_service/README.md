# Running unit tests

Follow these steps to run unit tests, in the repo's root dir:

```
docker pull scrolltech/go-rust-builder:go-1.21-rust-nightly-2023-12-03 --platform linux/amd64
docker run -it --rm -v "$(PWD):/workspace" -w /workspace scrolltech/go-rust-builder:go-1.21-rust-nightly-2023-12-03
mkdir -p /scroll/lib
wget -O /scroll/lib/libscroll_zstd.so https://github.com/scroll-tech/da-codec/releases/download/v0.0.0-rc0-ubuntu20.04/libscroll_zstd.so
wget -O /scroll/lib/libzktrie.so https://github.com/scroll-tech/da-codec/releases/download/v0.0.0-rc0-ubuntu20.04/libscroll_zstd.so
export LD_LIBRARY_PATH=$LD_LIBRARY_PATH:/scroll/lib/
export CGO_LDFLAGS="-L/scroll/lib/ -Wl,-rpath=/scroll/lib/"
go test -v -race ./rollup/rollup_sync_service/...
```
