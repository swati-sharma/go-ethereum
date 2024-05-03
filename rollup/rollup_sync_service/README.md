# libzstd

## Building `libscroll_zstd.so` File.

Follow these steps to build the `.so` file, in ./rollup/rollup_sync_service:

1. Build and enter the container:
    ```
    docker build -t my-dev-container --platform linux/amd64 .
    docker run -it --rm -v "$(PWD):/workspace" -w /workspace my-dev-container
    ```

2. Change directory:
    ```
    cd libzstd
    ```

3. Build libzstd:
    ```
    export CARGO_NET_GIT_FETCH_WITH_CLI=true
    make libzstd
    ```

## Running unit tests

Follow these steps to run unit tests, in the repo's root dir:

1. Build and enter the container:
    ```
    docker run -it --rm -v "$(PWD):/workspace" -w /workspace my-dev-container
    ```

2. Set the directory for shared libraries:
    ```
    export LD_LIBRARY_PATH=${PWD}/rollup/rollup_sync_service/libzstd:$LD_LIBRARY_PATH
    ```

3. Execute the unit tests:
    ```
    cd rollup/rollup_sync_service
    go test -v -race ./...
    ```
