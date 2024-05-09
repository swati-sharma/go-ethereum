# Running unit tests

Follow these steps to run unit tests, in the repo's root dir:

```
docker build -f ./rollup/rollup_sync_service/Dockerfile -t my-dev-container --platform linux/amd64 .
docker run -it --rm -v "$(PWD):/workspace" -w /workspace my-dev-container
go test -v -race ./rollup/rollup_sync_service/...
```
