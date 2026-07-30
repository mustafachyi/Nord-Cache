package nord

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	upstreamURL              = "https://api.nordvpn.com/v1/servers?limit=16384&filters[servers_technologies][identifier]=wireguard_udp&fields[station]=1&fields[hostname]=1&fields[load]=1&fields[technologies.metadata]=1&fields[locations.country.name]=1&fields[locations.country.code]=1&fields[locations.country.city.name]=1&fields[groups.identifier]=1"
	upstreamResponseMaxBytes = 64 * 1024 * 1024
)

var httpClient = &http.Client{
	Timeout: 20 * time.Second,
}

func FetchAndProcess(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstreamURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create upstream request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "NordCache/1.0")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request upstream catalog: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream returned HTTP %d", resp.StatusCode)
	}

	if resp.ContentLength > upstreamResponseMaxBytes {
		return nil, errors.New("upstream response exceeds the configured limit")
	}

	reader := &io.LimitedReader{
		R: resp.Body,
		N: upstreamResponseMaxBytes + 1,
	}
	decoder := json.NewDecoder(reader)

	var rawServers []RawServer
	if err := decoder.Decode(&rawServers); err != nil {
		return nil, fmt.Errorf("decode upstream response: %w", err)
	}

	if reader.N <= 0 {
		return nil, errors.New("upstream response exceeds the configured limit")
	}

	var trailingValue any
	if err := decoder.Decode(&trailingValue); err != io.EOF {
		if err == nil {
			return nil, errors.New("upstream response contains trailing JSON")
		}
		return nil, fmt.Errorf("decode upstream response: %w", err)
	}

	return Process(rawServers)
}
