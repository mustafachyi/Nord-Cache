# Nord Cache

A caching proxy for the NordVPN WireGuard server list. Fetches, filters, and serves an aggressively compacted, bitwise-packed server index refreshed every 5 minutes.

## Background

Built to support [NordVPN WireGuard Config Generator](https://github.com/mustafachyi/NordVPN-WireGuard-Config-Generator). Processing the raw NordVPN API response directly inside the Cloudflare Workers free tier was exceeding the 10ms CPU time limit. Nord Cache offloads that work to a dedicated service and exposes a pre-processed, type-bound, delta-encoded payload the worker can consume within budget.

## Operation

- Polls the NordVPN API for all `wireguard_udp` servers.
- Filters to servers with a valid public key and country code.
- Extracts server group designations via a bitmask representation.
- Compresses the dataset using delta encoding, default hoisting, bitwise packing, and V8-optimized flat arrays.
- Serves the result with Brotli encoding, ETag validation, and CORS headers.

## Output Format

The payload utilizes a highly normalized nested array structure to eliminate JSON structural overhead and force the V8 JavaScript engine to utilize `PACKED_SMI_ELEMENTS` (contiguous small integers) for maximum parsing iteration speed.

```json
[
  "kjAOz...Wed9wMVSa6...ExUs...",
  [
    [
      "CountryName", "cc",
      [
        [
          "CityName", <default_key_idx>, <default_group_mask>,
          <packed_0>, <dip_0>,
          <packed_1>, <dip_1>,
          [
            [<exception_id>, <key_ovr>, <grp_ovr>, "hname", "dedup"]
          ]
        ]
      ]
    ]
  ]
]
```

### Decoding Mechanics

#### 1. Monolithic Key String
The first element of the root array is a single continuous string containing all unique WireGuard public keys. The `=` base64 padding is stripped. To retrieve a key by its index, extract a 43-character substring and append `=`.

#### 2. City Node
Each city array starts with the city name, the default `key_idx`, and the default `group_mask` for servers in that city. Following these three elements is a flat sequence of integers. 

#### 3. Flat Server Integers (`packed` and `dip`)
Every server is represented by two sequential integers in the array:
- `dip`: The numeric delta from the preceding server's numeric IP.
- `packed`: A bitwise combination of the numeric hostname delta (`dNum`) and the server load (`Load`).

`Packed = (dNum << 7) | Load`.
- `Load = Packed & 0x7F`
- `dNum = Packed >> 7`

#### 4. Exceptions Array
If `Packed < 0`, it acts as a negative-space exception signal.
- The `Load` is still derived via `Packed & 0x7F`.
- The server's identifier and metadata overrides are located in the trailing exceptions array appended to the end of the city block. The V8 parser can safely break the integer loop using a `typeof === 'number'` boundary check. Exception parameters omit missing values based on array length.

### Group Mask
The `group_mask` maps server categories via bitwise addition:
- `1`: `legacy_standard`
- `2`: `legacy_p2p`
- `4`: `legacy_dedicated_ip`
- `8`: `legacy_onion_over_vpn`
- `16`: `legacy_double_vpn`

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Returns `200 OK` |
| `GET` | `/api/servers` | Returns the processed server list |

`/api/servers` supports `If-None-Match` and `Accept-Encoding: br`. Returns `503` until the first fetch completes.

## Running

**Docker**

```sh
docker build -t nord-cache .
docker run -p 8080:8080 nord-cache
```

**Local**

```sh
go run ./cmd/api
```

Requires Go 1.26.3.

## Configuration

All parameters are hardcoded. Edit the relevant file to change them.

| Parameter | Value | File |
|-----------|-------|------|
| Listen port | `:8080` | `cmd/api/main.go` |
| Refresh interval | `5m` | `cmd/api/main.go` |
| Upstream request timeout | `20s` | `internal/nord/client.go` |