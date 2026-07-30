package nord

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/netip"
	"sort"
	"strconv"
	"strings"
)

const maxPackedNumberDelta = math.MaxInt32 >> 7

func extractNumber(value string) string {
	start := -1
	for index := 0; index < len(value); index++ {
		character := value[index]
		if character >= '0' && character <= '9' {
			if start == -1 {
				start = index
			}
			continue
		}

		if start != -1 {
			return value[start:index]
		}
	}

	if start == -1 {
		return ""
	}

	return value[start:]
}

func normalizeName(value string) string {
	buffer := make([]byte, 0, len(value))
	separatorPending := false

	for index := 0; index < len(value); index++ {
		character := value[index]
		if character >= 'A' && character <= 'Z' {
			character += 'a' - 'A'
		}

		if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') {
			if separatorPending && len(buffer) > 0 {
				buffer = append(buffer, '_')
			}
			buffer = append(buffer, character)
			separatorPending = false
			continue
		}

		separatorPending = len(buffer) > 0
	}

	return string(buffer)
}

func sanitizeIdentifier(value string) string {
	buffer := make([]byte, 0, len(value))
	separatorPending := false

	for index := 0; index < len(value); index++ {
		character := value[index]
		isSeparator := character == ' ' || character == '\t' || character == '\n' || character == '\r' || character == '#'
		if isSeparator {
			if !separatorPending {
				buffer = append(buffer, '_')
				separatorPending = true
			}
			continue
		}

		buffer = append(buffer, character)
		separatorPending = false
	}

	return string(buffer)
}

func Process(rawServers []RawServer) ([]byte, error) {
	processedServers := make([]processedServer, 0, len(rawServers))
	normalizedNameCache := make(map[string]string)
	sanitizedIdentifierCache := make(map[string]string)

	getNormalizedName := func(value string) string {
		if normalized, exists := normalizedNameCache[value]; exists {
			return normalized
		}

		normalized := normalizeName(value)
		normalizedNameCache[value] = normalized
		return normalized
	}

	getSanitizedIdentifier := func(value string) string {
		if sanitized, exists := sanitizedIdentifierCache[value]; exists {
			return sanitized
		}

		sanitized := sanitizeIdentifier(value)
		sanitizedIdentifierCache[value] = sanitized
		return sanitized
	}

	for index, server := range rawServers {
		if len(server.Locations) == 0 {
			continue
		}

		publicKey := findPublicKey(server.Technologies)
		if publicKey == "" {
			continue
		}
		if !isWireGuardKey(publicKey) {
			return nil, fmt.Errorf("server %d contains an invalid public key", index)
		}
		if server.Load < 0 || server.Load > 100 {
			return nil, fmt.Errorf("server %d contains an invalid load", index)
		}
		if !isHostname(server.Hostname) {
			return nil, fmt.Errorf("server %d contains an invalid hostname", index)
		}

		location := server.Locations[0]
		countryCode := strings.ToLower(location.Country.Code)
		if !isCountryCode(countryCode) {
			return nil, fmt.Errorf("server %d contains an invalid country code", index)
		}

		countrySort := getNormalizedName(location.Country.Name)
		citySort := getNormalizedName(location.Country.City.Name)
		if countrySort == "" || citySort == "" {
			return nil, fmt.Errorf("server %d contains an invalid location", index)
		}
		country := getSanitizedIdentifier(location.Country.Name)
		city := getSanitizedIdentifier(location.Country.City.Name)

		address, err := netip.ParseAddr(server.Station)
		if err != nil || !address.Is4() {
			return nil, fmt.Errorf("server %d contains an invalid IPv4 address", index)
		}
		addressBytes := address.As4()

		number, numberValue, numberIsInteger, hostnameOverride, err := parseServerIdentity(
			server.Hostname,
			countryCode,
		)
		if err != nil {
			return nil, fmt.Errorf("server %d: %w", index, err)
		}

		processedServers = append(processedServers, processedServer{
			country:          country,
			city:             city,
			countrySort:      countrySort,
			citySort:         citySort,
			countryCode:      countryCode,
			number:           number,
			numberValue:      numberValue,
			numberIsInteger:  numberIsInteger,
			publicKey:        publicKey,
			load:             server.Load,
			groupMask:        buildGroupMask(server.Groups),
			ipNumber:         binary.BigEndian.Uint32(addressBytes[:]),
			hostnameOverride: hostnameOverride,
		})
	}

	if len(processedServers) == 0 {
		return nil, errors.New("upstream catalog contains no usable servers")
	}

	sortProcessedServers(processedServers)
	assignDeduplicationSuffixes(processedServers)

	publicKeys, keyIndexes := buildPublicKeyIndex(processedServers)
	countries := buildCountryNodes(processedServers, keyIndexes)
	return buildJSON(publicKeys, countries)
}

func findPublicKey(technologies []Technology) string {
	for _, technology := range technologies {
		for _, metadata := range technology.Metadata {
			if metadata.Name == "public_key" {
				return metadata.Value
			}
		}
	}

	return ""
}

func isWireGuardKey(value string) bool {
	if len(value) != 44 || value[43] != '=' {
		return false
	}

	decoded, err := base64.StdEncoding.DecodeString(value)
	return err == nil && len(decoded) == 32
}

func isCountryCode(value string) bool {
	if len(value) != 2 {
		return false
	}

	for index := 0; index < len(value); index++ {
		if value[index] < 'a' || value[index] > 'z' {
			return false
		}
	}

	return true
}

func isHostname(value string) bool {
	if len(value) == 0 || len(value) > 253 || strings.HasSuffix(value, ".") {
		return false
	}

	for _, label := range strings.Split(value, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}

		for index := 0; index < len(label); index++ {
			character := label[index]
			if (character >= 'a' && character <= 'z') ||
				(character >= 'A' && character <= 'Z') ||
				(character >= '0' && character <= '9') ||
				character == '-' {
				continue
			}
			return false
		}
	}

	return true
}

func isSafeIdentifier(value string) bool {
	if value == "" {
		return false
	}

	for index := 0; index < len(value); index++ {
		character := value[index]
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			character == '.' || character == '_' || character == '-' {
			continue
		}
		return false
	}

	return true
}

func parseServerIdentity(hostname string, countryCode string) (string, int, bool, string, error) {
	hostnamePrefix := countryCode
	if hostnamePrefix == "gb" {
		hostnamePrefix = "uk"
	}

	extractedNumber := extractNumber(hostname)
	numberValue := 0
	if extractedNumber != "" {
		parsedNumber, err := strconv.Atoi(extractedNumber)
		if err != nil {
			return "", 0, false, "", errors.New("hostname contains an invalid server number")
		}
		numberValue = parsedNumber
	}

	if strings.HasSuffix(hostname, ".nordvpn.com") {
		baseHostname := strings.TrimSuffix(hostname, ".nordvpn.com")
		if extractedNumber != "" && hostnamePrefix+extractedNumber == baseHostname {
			return extractedNumber, numberValue, true, "", nil
		}
		if !isSafeIdentifier(baseHostname) {
			return "", 0, false, "", errors.New("hostname contains an invalid server identifier")
		}
		return baseHostname, numberValue, false, "", nil
	}

	if extractedNumber != "" {
		return extractedNumber, numberValue, true, hostname, nil
	}

	return "wg", 0, false, hostname, nil
}

func buildGroupMask(groups []Group) int {
	mask := 0
	for _, group := range groups {
		switch group.Identifier {
		case "legacy_standard":
			mask |= 1
		case "legacy_p2p":
			mask |= 2
		case "legacy_dedicated_ip":
			mask |= 4
		case "legacy_onion_over_vpn":
			mask |= 8
		case "legacy_double_vpn":
			mask |= 16
		}
	}
	return mask
}

func sortProcessedServers(servers []processedServer) {
	sort.Slice(servers, func(firstIndex int, secondIndex int) bool {
		first := servers[firstIndex]
		second := servers[secondIndex]

		if first.countrySort != second.countrySort {
			return first.countrySort < second.countrySort
		}
		if first.country != second.country {
			return first.country < second.country
		}
		if first.countryCode != second.countryCode {
			return first.countryCode < second.countryCode
		}
		if first.citySort != second.citySort {
			return first.citySort < second.citySort
		}
		if first.city != second.city {
			return first.city < second.city
		}
		if first.numberValue != second.numberValue {
			return first.numberValue < second.numberValue
		}
		if first.number != second.number {
			return first.number < second.number
		}
		if first.hostnameOverride != second.hostnameOverride {
			return first.hostnameOverride < second.hostnameOverride
		}
		if first.ipNumber != second.ipNumber {
			return first.ipNumber < second.ipNumber
		}
		if first.publicKey != second.publicKey {
			return first.publicKey < second.publicKey
		}
		if first.groupMask != second.groupMask {
			return first.groupMask < second.groupMask
		}
		return first.load < second.load
	})
}

func assignDeduplicationSuffixes(servers []processedServer) {
	for cityStart := 0; cityStart < len(servers); {
		cityEnd := cityStart + 1
		for cityEnd < len(servers) &&
			servers[cityEnd].country == servers[cityStart].country &&
			servers[cityEnd].countryCode == servers[cityStart].countryCode &&
			servers[cityEnd].city == servers[cityStart].city {
			cityEnd++
		}

		nameCounts := make(map[string]int)
		for index := cityStart; index < cityEnd; index++ {
			baseName := servers[index].countryCode + servers[index].number
			count := nameCounts[baseName]
			nameCounts[baseName] = count + 1
			if count > 0 {
				servers[index].deduplicationSuffix = "_" + strconv.Itoa(count)
			}
		}

		cityStart = cityEnd
	}
}

func buildPublicKeyIndex(servers []processedServer) ([]string, map[string]int) {
	keyFrequencies := make(map[string]int)
	for _, server := range servers {
		keyFrequencies[server.publicKey]++
	}

	keys := make([]string, 0, len(keyFrequencies))
	for key := range keyFrequencies {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(firstIndex int, secondIndex int) bool {
		first := keys[firstIndex]
		second := keys[secondIndex]
		if keyFrequencies[first] != keyFrequencies[second] {
			return keyFrequencies[first] > keyFrequencies[second]
		}
		return first < second
	})

	indexes := make(map[string]int, len(keys))
	for index, key := range keys {
		indexes[key] = index
	}

	return keys, indexes
}

func buildCountryNodes(servers []processedServer, keyIndexes map[string]int) []countryNode {
	countries := make([]countryNode, 0)

	for _, server := range servers {
		if len(countries) == 0 ||
			countries[len(countries)-1].name != server.country ||
			countries[len(countries)-1].countryCode != server.countryCode {
			countries = append(countries, countryNode{
				name:        server.country,
				countryCode: server.countryCode,
			})
		}

		country := &countries[len(countries)-1]
		if len(country.cities) == 0 || country.cities[len(country.cities)-1].name != server.city {
			country.cities = append(country.cities, cityNode{name: server.city})
		}

		city := &country.cities[len(country.cities)-1]
		city.servers = append(city.servers, serverNode{
			number:              server.number,
			numberValue:         server.numberValue,
			numberIsInteger:     server.numberIsInteger,
			load:                server.load,
			ipNumber:            server.ipNumber,
			keyIndex:            keyIndexes[server.publicKey],
			groupMask:           server.groupMask,
			hostnameOverride:    server.hostnameOverride,
			deduplicationSuffix: server.deduplicationSuffix,
		})
	}

	return countries
}

func buildJSON(keys []string, countries []countryNode) ([]byte, error) {
	buffer := make([]byte, 0, 2*1024*1024)
	buffer = append(buffer, '[')

	var keyCollection strings.Builder
	keyCollection.Grow(len(keys) * 43)
	for _, key := range keys {
		keyCollection.WriteString(strings.TrimSuffix(key, "="))
	}
	buffer = strconv.AppendQuote(buffer, keyCollection.String())
	buffer = append(buffer, ',', '[')

	for countryIndex, country := range countries {
		if countryIndex > 0 {
			buffer = append(buffer, ',')
		}

		buffer = append(buffer, '[')
		buffer = strconv.AppendQuote(buffer, country.name)
		buffer = append(buffer, ',')
		buffer = strconv.AppendQuote(buffer, country.countryCode)
		buffer = append(buffer, ',', '[')

		for cityIndex, city := range country.cities {
			if cityIndex > 0 {
				buffer = append(buffer, ',')
			}

			defaultKeyIndex := selectDefault(city.servers, func(server serverNode) int {
				return server.keyIndex
			})
			defaultGroupMask := selectDefault(city.servers, func(server serverNode) int {
				return server.groupMask
			})

			buffer = append(buffer, '[')
			buffer = strconv.AppendQuote(buffer, city.name)
			buffer = append(buffer, ',')
			buffer = strconv.AppendInt(buffer, int64(defaultKeyIndex), 10)
			buffer = append(buffer, ',')
			buffer = strconv.AppendInt(buffer, int64(defaultGroupMask), 10)

			exceptions := make([][]any, 0)
			lastNumber := 0
			var lastIP uint32

			for _, server := range city.servers {
				isException := !server.numberIsInteger ||
					server.keyIndex != defaultKeyIndex ||
					server.groupMask != defaultGroupMask ||
					server.deduplicationSuffix != "" ||
					server.hostnameOverride != ""
				ipDelta := int64(server.ipNumber) - int64(lastIP)

				if isException {
					packedValue := (-1 << 7) | (server.load & 0x7f)
					buffer = appendPackedServer(buffer, packedValue, ipDelta)
					lastIP = server.ipNumber

					var identifier any
					if server.numberIsInteger {
						identifier = server.numberValue
						lastNumber = server.numberValue
					} else {
						identifier = server.number
					}

					keyOverride := -1
					if server.keyIndex != defaultKeyIndex {
						keyOverride = server.keyIndex
					}
					groupOverride := -1
					if server.groupMask != defaultGroupMask {
						groupOverride = server.groupMask
					}

					exception := []any{identifier}
					switch {
					case server.deduplicationSuffix != "":
						exception = append(
							exception,
							keyOverride,
							groupOverride,
							server.hostnameOverride,
							server.deduplicationSuffix,
						)
					case server.hostnameOverride != "":
						exception = append(exception, keyOverride, groupOverride, server.hostnameOverride)
					case groupOverride != -1:
						exception = append(exception, keyOverride, groupOverride)
					case keyOverride != -1:
						exception = append(exception, keyOverride)
					}
					exceptions = append(exceptions, exception)
					continue
				}

				numberDelta := server.numberValue - lastNumber
				if numberDelta < 0 || numberDelta > maxPackedNumberDelta {
					return nil, errors.New("server number delta exceeds the packed format")
				}

				packedValue := (numberDelta << 7) | (server.load & 0x7f)
				buffer = appendPackedServer(buffer, packedValue, ipDelta)
				lastNumber = server.numberValue
				lastIP = server.ipNumber
			}

			if len(exceptions) > 0 {
				exceptionJSON, err := json.Marshal(exceptions)
				if err != nil {
					return nil, fmt.Errorf("encode server exceptions: %w", err)
				}
				buffer = append(buffer, ',')
				buffer = append(buffer, exceptionJSON...)
			}

			buffer = append(buffer, ']')
		}

		buffer = append(buffer, ']', ']')
	}

	buffer = append(buffer, ']', ']')
	return buffer, nil
}

func selectDefault(servers []serverNode, valueOf func(serverNode) int) int {
	counts := make(map[int]int)
	selectedValue := 0
	selectedCount := -1

	for _, server := range servers {
		value := valueOf(server)
		counts[value]++
		count := counts[value]
		if count > selectedCount || (count == selectedCount && value < selectedValue) {
			selectedValue = value
			selectedCount = count
		}
	}

	return selectedValue
}

func appendPackedServer(buffer []byte, packedValue int, ipDelta int64) []byte {
	buffer = append(buffer, ',')
	buffer = strconv.AppendInt(buffer, int64(packedValue), 10)
	buffer = append(buffer, ',')
	return strconv.AppendInt(buffer, ipDelta, 10)
}
