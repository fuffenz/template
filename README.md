# Netnod DNS for `libdns`

[![godoc reference](https://img.shields.io/badge/godoc-reference-blue.svg)](https://pkg.go.dev/github.com/libdns/netnod)

This package implements the [libdns](https://github.com/libdns/libdns) interfaces for the [Netnod Primary DNS API](https://primarydnsapi.netnod.se).

[Netnod DNS services](https://netnod.se/dns/)

## Authenticating

To authenticate, you need a Netnod API token. Generate one through the Netnod customer portal and ensure your client IP is within the configured prefix range.

## Example

```go
provider := &netnod.Provider{
    APIToken: "your-api-token",
}

records, err := provider.GetRecords(context.Background(), "example.com.")
```

See [provider_test.go](provider_test.go) for more usage examples.

## Testing

Run integration tests against a live Netnod account:

```bash
NETNOD_API_TOKEN=your-token NETNOD_TEST_ZONE=example.com. go test -race -v ./...
```

## License

BSD 3-Clause
