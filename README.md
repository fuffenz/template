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

## Caveats

### Rate Limiting

The Netnod API enforces rate limiting (HTTP 429). This provider automatically retries
rate-limited requests using exponential backoff, respecting the `Retry-After` header when
present. It is important to set a deadline on the context to avoid retrying indefinitely:

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
records, err := provider.GetRecords(ctx, "example.com.")
```

### TTL

The Netnod API operates on RRsets (all records sharing a name and type). A single TTL applies
to the entire RRset. If multiple records with the same name and type specify different TTLs,
only the first value is used.

### Atomicity

Updates are not atomic across processes. Concurrent modifications from multiple processes to
the same RRset may result in inconsistent state. To avoid conflicts, ensure that concurrent
processes operate on different RRsets.

## Testing

Run integration tests against a live Netnod account:

```bash
NETNOD_API_TOKEN=your-token NETNOD_TEST_ZONE=example.com. go test -race -v ./...
```

## License

BSD 3-Clause
