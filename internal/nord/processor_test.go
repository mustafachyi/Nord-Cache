package nord

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

func TestProcessMatchesCatalogContract(t *testing.T) {
	firstKey := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))
	secondKey := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{2}, 32))
	servers := []RawServer{
		newRawServer("1.1.1.1", "us100.nordvpn.com", 10, firstKey, "legacy_standard"),
		newRawServer("1.1.1.2", "us101.nordvpn.com", 20, secondKey, "legacy_p2p"),
		newRawServer("1.1.1.3", "special.nordvpn.com", 30, firstKey, "legacy_standard"),
	}

	result, err := Process(servers)
	if err != nil {
		t.Fatalf("Process() returned an error: %v", err)
	}

	expected := `["` + strings.TrimSuffix(firstKey, "=") + strings.TrimSuffix(secondKey, "=") + `",[["United_States","us",[["New_York",0,1,-98,16843011,12810,-2,-108,1,[["special"],[101,1,2]]]]]]]`
	if string(result) != expected {
		t.Fatalf("Process() output = %s, want %s", result, expected)
	}
}

func TestProcessIsDeterministic(t *testing.T) {
	firstKey := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))
	secondKey := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{2}, 32))
	servers := []RawServer{
		newRawServer("1.1.1.1", "us100.nordvpn.com", 10, firstKey, "legacy_standard"),
		newRawServer("1.1.1.2", "us101.nordvpn.com", 20, secondKey, "legacy_p2p"),
		newRawServer("1.1.1.3", "special.nordvpn.com", 30, firstKey, "legacy_standard"),
	}

	forward, err := Process(servers)
	if err != nil {
		t.Fatalf("Process() returned an error: %v", err)
	}

	reversed, err := Process([]RawServer{servers[2], servers[1], servers[0]})
	if err != nil {
		t.Fatalf("Process() returned an error: %v", err)
	}

	if !bytes.Equal(forward, reversed) {
		t.Fatalf("Process() output depends on input order\nforward: %s\nreverse: %s", forward, reversed)
	}
}

func TestProcessRejectsInvalidIPv4Address(t *testing.T) {
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))
	server := newRawServer("999.1.1.1", "us100.nordvpn.com", 10, key, "legacy_standard")

	if _, err := Process([]RawServer{server}); err == nil {
		t.Fatal("Process() accepted an invalid IPv4 address")
	}
}

func TestProcessRejectsEmptyCatalog(t *testing.T) {
	if _, err := Process(nil); err == nil {
		t.Fatal("Process() accepted an empty catalog")
	}
}

func newRawServer(station string, hostname string, load int, key string, group string) RawServer {
	return RawServer{
		Station:  station,
		Hostname: hostname,
		Load:     load,
		Locations: []Location{{
			Country: Country{
				Name: "United States",
				Code: "US",
				City: City{Name: "New York"},
			},
		}},
		Groups: []Group{{Identifier: group}},
		Technologies: []Technology{{
			Metadata: []Metadata{{Name: "public_key", Value: key}},
		}},
	}
}
