# Nord Cache

Nord Cache fetches NordVPN WireGuard server metadata, validates and compacts it, and serves the resulting catalog to NordGen.

## Architecture

- Polls the NordVPN server API every five minutes.
- Retains only servers with a location and valid WireGuard public key.
- Validates hostnames, IPv4 addresses, loads, country codes, and public keys before publishing a new catalog.
- Produces deterministic output independent of upstream server ordering.
- Stores raw and Brotli-encoded representations in memory behind an atomic pointer.
- Uses a weak SHA-256 content ETag shared by the equivalent raw and Brotli representations.
- Skips Brotli recompression and store replacement when the processed catalog is unchanged.
- Keeps the previous valid catalog available when a refresh fails.

The service has no persistent state. After a restart, `/api/servers` returns `503 Service Unavailable` until the first successful refresh completes.

## Output format

The response body is a compact JSON tuple:

```json
[
  "concatenated-public-keys",
  [
    [
      "Country_Name",
      "cc",
      [
        [
          "City_Name",
          0,
          1,
          12810,
          16843009,
          [
            ["identifier", -1, -1, "hostname", "_1"]
          ]
        ]
      ]
    ]
  ]
]
```

### Public key collection

The first tuple element concatenates all unique WireGuard public keys after removing each key's trailing `=` padding. Every key occupies 43 characters. Consumers recover a key by slicing the string at `keyIndex * 43` and appending `=`.

### Country and city nodes

Each country is encoded as:

```text
[countryName, countryCode, cities]
```

Each city starts with:

```text
[cityName, defaultKeyIndex, defaultGroupMask, packedServerData...]
```

The default key and group values are selected deterministically by frequency and then by the lowest numeric value when frequencies tie.

### Packed server data

Each normal server uses two integers:

```text
[packedNumberAndLoad, ipDelta]
```

The packed value is:

```text
(numberDelta << 7) | load
```

The low seven bits contain the load. The remaining bits contain the numeric server-number delta.

A negative packed value marks an exception. Its corresponding entry in the city's trailing exception array supplies the server identifier and any key, group, hostname, or deduplication overrides.

### Group mask

- `1`: `legacy_standard`
- `2`: `legacy_p2p`
- `4`: `legacy_dedicated_ip`
- `8`: `legacy_onion_over_vpn`
- `16`: `legacy_double_vpn`

## Endpoints

| Method | Path | Description |
|---|---|---|
| `GET` | `/health` | Liveness response |
| `HEAD` | `/health` | Liveness headers |
| `GET` | `/api/servers` | Processed catalog |
| `HEAD` | `/api/servers` | Catalog headers without a body |
| `OPTIONS` | `/api/servers` | CORS preflight response |

`/api/servers` supports `If-None-Match` and Brotli content negotiation. It returns `503` until the first successful refresh.

## Requirements

- Go 1.26.5
- Docker, when building the container image

## Run locally

```cmd
go run ./cmd/api
```

## Test

```cmd
go test ./...
```

```cmd
go vet ./...
```

## Build and run with Docker

```cmd
docker build --pull -t nord-cache:1.26.5 .
```

```cmd
docker run --rm -p 8080:8080 nord-cache:1.26.5
```

## Fixed configuration

| Setting | Value | File |
|---|---:|---|
| Listen address | `:8080` | `cmd/api/main.go` |
| Refresh interval | `5m` | `cmd/api/main.go` |
| Upstream request timeout | `20s` | `internal/nord/client.go` |
| Upstream response limit | `64 MiB` | `internal/nord/client.go` |
| Read-header timeout | `5s` | `cmd/api/main.go` |
| Read timeout | `10s` | `cmd/api/main.go` |
| Write timeout | `15s` | `cmd/api/main.go` |