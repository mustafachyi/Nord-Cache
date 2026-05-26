# Nord Cache

A caching proxy for the NordVPN WireGuard server list. Fetches, filters, and serves a compacted server index refreshed every 5 minutes.

## Background

Built to support [NordVPN WireGuard Config Generator](https://github.com/mustafachyi/NordVPN-WireGuard-Config-Generator). Processing the raw NordVPN API response directly inside the Cloudflare Workers free tier was exceeding the 10ms CPU time limit. Nord Cache offloads that work to a dedicated service and exposes a pre-processed, compacted payload the worker can consume within budget.

## What it does

- Polls the NordVPN API for all `wireguard_udp` servers
- Filters to servers running WireGuard protocol version ≥ 2.1 with a valid public key and country code
- Normalizes and deduplicates server data into a compact JSON structure
- Serves the result with gzip encoding, ETag validation, and CORS headers

## Output format

```json
{
  "k": ["<wireguard-public-key>", "..."],
  "l": [
    ["<country>", "<country-code>", [
      ["<city>", [
        [<number>, <load>, <ip-numeric>, <key-index>]
      ]]
    ]]
  ]
}
```

`k` is a deduplicated list of WireGuard public keys. Each server references its key by index. Countries and cities are sorted alphabetically; servers are sorted by number. A server tuple may include an additional hostname field when it does not follow the standard `<code><number>.nordvpn.com` pattern, and a dedup suffix when multiple servers in the same city share the same identifier.

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Returns `200 OK` |
| `GET` | `/api/servers` | Returns the processed server list |

`/api/servers` supports `If-None-Match` and `Accept-Encoding: gzip`. Returns `503` until the first fetch completes.

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

Requires Go 1.26.3. No external dependencies.

## Configuration

All parameters are hardcoded. Edit the relevant file to change them.

| Parameter | Value | File |
|-----------|-------|------|
| Listen port | `:8080` | `cmd/api/main.go` |
| Refresh interval | `5m` | `cmd/api/main.go` |
| Upstream request timeout | `20s` | `internal/nord/client.go` |