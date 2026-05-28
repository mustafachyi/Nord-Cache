package nord

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const upstreamURL = "https://api.nordvpn.com/v1/servers?limit=16384&filters[servers_technologies][identifier]=wireguard_udp&fields[station]=1&fields[hostname]=1&fields[load]=1&fields[technologies.metadata]=1&fields[locations.country.name]=1&fields[locations.country.code]=1&fields[locations.country.city.name]=1&fields[groups.identifier]=1"

var httpClient = &http.Client{
	Timeout: 20 * time.Second,
}

func FetchAndProcess() ([]byte, error) {
	req, err := http.NewRequest("GET", upstreamURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream returned status: %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var rawServers []RawServer
	if err := json.Unmarshal(bodyBytes, &rawServers); err != nil {
		return nil, err
	}

	return Process(rawServers)
}
