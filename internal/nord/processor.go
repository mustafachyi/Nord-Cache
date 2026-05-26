package nord

import (
	"sort"
	"strconv"
	"strings"
)

func ipToNumeric(ip string) uint32 {
	var val uint32
	var octet uint32
	var shift uint32 = 24
	for i := 0; i < len(ip); i++ {
		c := ip[i]
		if c == '.' {
			val = val | (octet << shift)
			octet = 0
			shift -= 8
		} else {
			octet = octet*10 + uint32(c-'0')
		}
	}
	return val | (octet << shift)
}

func extractNumber(s string) string {
	start := -1
	end := -1
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			if start == -1 {
				start = i
			}
			end = i + 1
		} else if start != -1 {
			break
		}
	}
	if start == -1 {
		return ""
	}
	return s[start:end]
}

func validateVersion(v string) bool {
	dot1 := -1
	for i := 0; i < len(v); i++ {
		if v[i] == '.' {
			dot1 = i
			break
		}
	}
	if dot1 <= 0 {
		return false
	}
	major, err := strconv.Atoi(v[:dot1])
	if err != nil {
		return false
	}
	if major > 2 {
		return true
	}
	if major < 2 {
		return false
	}

	rest := v[dot1+1:]
	dot2 := -1
	for i := 0; i < len(rest); i++ {
		if rest[i] == '.' {
			dot2 = i
			break
		}
	}
	var minorStr string
	if dot2 == -1 {
		minorStr = rest
	} else {
		minorStr = rest[:dot2]
	}
	minor, err := strconv.Atoi(minorStr)
	if err != nil {
		return false
	}
	return minor >= 1
}

func normalizeName(s string) string {
	var b []byte
	inReplace := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			b = append(b, c)
			inReplace = false
		} else {
			if !inReplace && len(b) > 0 {
				b = append(b, '_')
				inReplace = true
			}
		}
	}
	if len(b) > 0 && b[len(b)-1] == '_' {
		b = b[:len(b)-1]
	}
	return string(b)
}

func sanitizeIdentifier(s string) string {
	var b []byte
	inReplace := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		isMatch := c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '#'
		if isMatch {
			if !inReplace {
				b = append(b, '_')
				inReplace = true
			}
		} else {
			b = append(b, c)
			inReplace = false
		}
	}
	return string(b)
}

func Process(rawServers []RawServer) ([]byte, error) {
	processed := make([]ProcessedServer, 0, len(rawServers))
	var uniqueKeys []string
	keyMap := make(map[string]int)

	normalizeCache := make(map[string]string)
	sanitizeCache := make(map[string]string)
	lowCodeCache := make(map[string]string)

	getNormalized := func(val string) string {
		if res, ok := normalizeCache[val]; ok {
			return res
		}
		res := normalizeName(val)
		normalizeCache[val] = res
		return res
	}

	getSanitized := func(val string) string {
		if res, ok := sanitizeCache[val]; ok {
			return res
		}
		res := sanitizeIdentifier(val)
		sanitizeCache[val] = res
		return res
	}

	getLowCode := func(val string) string {
		if res, ok := lowCodeCache[val]; ok {
			return res
		}
		res := strings.ToLower(val)
		lowCodeCache[val] = res
		return res
	}

	for _, server := range rawServers {
		if len(server.Locations) == 0 {
			continue
		}

		version := "0.0.0"
		for _, spec := range server.Specifications {
			if spec.Identifier == "version" && len(spec.Values) > 0 {
				version = spec.Values[0].Value
				break
			}
		}

		if !validateVersion(version) {
			continue
		}

		publicKey := ""
		for _, tech := range server.Technologies {
			for _, meta := range tech.Metadata {
				if meta.Name == "public_key" {
					publicKey = meta.Value
					break
				}
			}
			if publicKey != "" {
				break
			}
		}

		loc := server.Locations[0]
		if loc.Country.Code == "" || publicKey == "" {
			continue
		}

		keyIdx, exists := keyMap[publicKey]
		if !exists {
			keyIdx = len(uniqueKeys)
			uniqueKeys = append(uniqueKeys, publicKey)
			keyMap[publicKey] = keyIdx
		}

		lowCountryCode := getLowCode(loc.Country.Code)
		serverNumber := extractNumber(server.Hostname)
		if serverNumber == "" {
			serverNumber = "wg"
		}

		ipNum := ipToNumeric(server.Station)
		prefix := lowCountryCode
		if prefix == "gb" {
			prefix = "uk"
		}
		expectedHostname := prefix + serverNumber + ".nordvpn.com"
		hName := ""
		if server.Hostname != expectedHostname {
			hName = server.Hostname
		}

		processed = append(processed, ProcessedServer{
			Country:        getNormalized(loc.Country.Name),
			City:           getNormalized(loc.Country.City.Name),
			LowCode:        lowCountryCode,
			Number:         serverNumber,
			KeyIndex:       keyIdx,
			Load:           server.Load,
			RawCountryName: loc.Country.Name,
			RawCityName:    loc.Country.City.Name,
			IpNum:          ipNum,
			HName:          hName,
		})
	}

	sort.Slice(processed, func(i, j int) bool {
		if processed[i].Country != processed[j].Country {
			return processed[i].Country < processed[j].Country
		}
		if processed[i].City != processed[j].City {
			return processed[i].City < processed[j].City
		}
		numA, errA := strconv.Atoi(processed[i].Number)
		numB, errB := strconv.Atoi(processed[j].Number)
		if errA == nil && errB == nil {
			return numA < numB
		}
		return processed[i].Number < processed[j].Number
	})

	cityStart := 0
	totalServers := len(processed)
	for cityStart < totalServers {
		cityEnd := cityStart + 1
		for cityEnd < totalServers &&
			processed[cityEnd].City == processed[cityStart].City &&
			processed[cityEnd].Country == processed[cityStart].Country {
			cityEnd++
		}

		nameCounts := make(map[string]int)
		for i := cityStart; i < cityEnd; i++ {
			baseName := processed[i].LowCode + processed[i].Number
			count := nameCounts[baseName]
			nameCounts[baseName] = count + 1
			if count > 0 {
				processed[i].DedupSuffix = "_" + strconv.Itoa(count)
			}
		}
		cityStart = cityEnd
	}

	var countries []CountryNode
	var currentCountry *CountryNode
	var currentCity *CityNode

	for _, srv := range processed {
		countryKey := getSanitized(srv.RawCountryName)
		cityKey := getSanitized(srv.RawCityName)

		if currentCountry == nil || currentCountry.Name != countryKey {
			countries = append(countries, CountryNode{
				Name:    countryKey,
				LowCode: srv.LowCode,
				Cities:  []CityNode{},
			})
			currentCountry = &countries[len(countries)-1]
			currentCity = nil
		}

		if currentCity == nil || currentCity.Name != cityKey {
			currentCountry.Cities = append(currentCountry.Cities, CityNode{
				Name:    cityKey,
				Servers: []ServerNode{},
			})
			currentCity = &currentCountry.Cities[len(currentCountry.Cities)-1]
		}

		numInt, err := strconv.Atoi(srv.Number)
		node := ServerNode{
			Number:      srv.Number,
			NumInt:      numInt,
			NumIsInt:    err == nil,
			Load:        srv.Load,
			IpNum:       srv.IpNum,
			KeyIdx:      srv.KeyIndex,
			HName:       srv.HName,
			DedupSuffix: srv.DedupSuffix,
		}

		currentCity.Servers = append(currentCity.Servers, node)
	}

	return buildJSON(uniqueKeys, countries), nil
}

func buildJSON(keys []string, countries []CountryNode) []byte {
	buf := make([]byte, 0, 2*1024*1024)

	buf = append(buf, `{"k":[`...)
	for i, k := range keys {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = strconv.AppendQuote(buf, k)
	}
	buf = append(buf, `],"l":[`...)

	for i, c := range countries {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, '[')
		buf = strconv.AppendQuote(buf, c.Name)
		buf = append(buf, ',')
		buf = strconv.AppendQuote(buf, c.LowCode)
		buf = append(buf, ',')
		buf = append(buf, '[')
		for j, city := range c.Cities {
			if j > 0 {
				buf = append(buf, ',')
			}
			buf = append(buf, '[')
			buf = strconv.AppendQuote(buf, city.Name)
			buf = append(buf, ',')
			buf = append(buf, '[')
			for k, srv := range city.Servers {
				if k > 0 {
					buf = append(buf, ',')
				}
				buf = append(buf, '[')

				if srv.NumIsInt {
					buf = strconv.AppendInt(buf, int64(srv.NumInt), 10)
				} else {
					buf = strconv.AppendQuote(buf, srv.Number)
				}
				buf = append(buf, ',')
				buf = strconv.AppendInt(buf, int64(srv.Load), 10)
				buf = append(buf, ',')
				buf = strconv.AppendUint(buf, uint64(srv.IpNum), 10)
				buf = append(buf, ',')
				buf = strconv.AppendInt(buf, int64(srv.KeyIdx), 10)

				if srv.DedupSuffix != "" {
					buf = append(buf, ',')
					buf = strconv.AppendQuote(buf, srv.HName)
					buf = append(buf, ',')
					buf = strconv.AppendQuote(buf, srv.DedupSuffix)
				} else if srv.HName != "" {
					buf = append(buf, ',')
					buf = strconv.AppendQuote(buf, srv.HName)
				}
				buf = append(buf, ']')
			}
			buf = append(buf, ']')
			buf = append(buf, ']')
		}
		buf = append(buf, ']')
		buf = append(buf, ']')
	}
	buf = append(buf, `]}`...)

	return buf
}
