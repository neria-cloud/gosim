# Simulated file read with Go 1.25.2

This standalone module creates `/tmp/testfile.txt` in Gosim's simulated
filesystem with the exact contents `Hello World!`, reads it with `os.ReadFile`,
and compares the result. It does not read or write the host's `/tmp/testfile.txt`.

From the repository root:

```sh
make build
GOTOOLCHAIN=go1.25.2 ./.gosim/gosimtool -C examples/filesystem test -features rt,fs -v -run .
```

`GOTOOLCHAIN=go1.25.2` selects exactly that compiler, downloading it if needed.
The test fails if another compiler version is used. The `go` directive in
`go.mod` alone is only a minimum version, not an exact compiler pin.

The launcher can be built with another supported Go version; it selects a
Go 1.25.2 worker for this command. The test is simulation-only, and this nested
module is not included in the root `make test` suite.
