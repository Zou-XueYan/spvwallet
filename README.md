# Lightweight Spv Client for Bitcoin

The original code comes from [OpenBazaar/spvwallet](https://github.com/OpenBazaar/spvwallet), thanks a lot.

This is a light bitcoin client which only synchronize and store block headers. You can get headers in bytes from this client and decode by btcd.

## Usage

Build the client and use <u>./run help</u> to check the configuration.

```go
go build ./cmd/lightcli/run.go
```

