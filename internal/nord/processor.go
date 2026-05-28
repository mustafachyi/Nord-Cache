package nord

import (
	"encoding/json"
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

		groupMask := 0
		for _, g := range server.Groups {
			switch g.Identifier {
			case "legacy_standard":
				groupMask |= 1
			case "legacy_p2p":
				groupMask |= 2
			case "legacy_dedicated_ip":
				groupMask |= 4
			case "legacy_onion_over_vpn":
				groupMask |= 8
			case "legacy_double_vpn":
				groupMask |= 16
			}
		}

		lowCountryCode := getLowCode(loc.Country.Code)
		prefix := lowCountryCode
		if prefix == "gb" {
			prefix = "uk"
		}

		extractedNumStr := extractNumber(server.Hostname)
		numVal := 0
		if extractedNumStr != "" {
			numVal, _ = strconv.Atoi(extractedNumStr)
		}

		serverNumber := ""
		hName := ""

		if strings.HasSuffix(server.Hostname, ".nordvpn.com") {
			base := strings.TrimSuffix(server.Hostname, ".nordvpn.com")
			if extractedNumStr != "" && prefix+extractedNumStr == base {
				serverNumber = extractedNumStr
			} else {
				serverNumber = base
			}
		} else {
			if extractedNumStr != "" {
				serverNumber = extractedNumStr
			} else {
				serverNumber = "wg"
			}
			hName = server.Hostname
		}

		ipNum := ipToNumeric(server.Station)

		processed = append(processed, ProcessedServer{
			Country:        getNormalized(loc.Country.Name),
			City:           getNormalized(loc.Country.City.Name),
			LowCode:        lowCountryCode,
			Number:         serverNumber,
			NumVal:         numVal,
			KeyIndex:       keyIdx,
			Load:           server.Load,
			GroupMask:      groupMask,
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
		if processed[i].NumVal != processed[j].NumVal {
			return processed[i].NumVal < processed[j].NumVal
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
			GroupMask:   srv.GroupMask,
			HName:       srv.HName,
			DedupSuffix: srv.DedupSuffix,
		}

		currentCity.Servers = append(currentCity.Servers, node)
	}

	return buildJSON(uniqueKeys, countries), nil
}

func buildJSON(keys []string, countries []CountryNode) []byte {
	buf := make([]byte, 0, 2*1024*1024)
	buf = append(buf, '[')

	var kb strings.Builder
	for _, k := range keys {
		kb.WriteString(strings.TrimRight(k, "="))
	}
	buf = strconv.AppendQuote(buf, kb.String())
	buf = append(buf, ',', '[')

	for i, c := range countries {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, '[')
		buf = strconv.AppendQuote(buf, c.Name)
		buf = append(buf, ',')
		buf = strconv.AppendQuote(buf, c.LowCode)
		buf = append(buf, ',', '[')

		for j, city := range c.Cities {
			if j > 0 {
				buf = append(buf, ',')
			}

			keyFreq := make(map[int]int)
			grpFreq := make(map[int]int)
			for _, srv := range city.Servers {
				keyFreq[srv.KeyIdx]++
				grpFreq[srv.GroupMask]++
			}

			defKey, defGrp := -1, -1
			maxK, maxG := -1, -1
			for k, v := range keyFreq {
				if v > maxK {
					maxK = v
					defKey = k
				}
			}
			for g, v := range grpFreq {
				if v > maxG {
					maxG = v
					defGrp = g
				}
			}

			buf = append(buf, '[')
			buf = strconv.AppendQuote(buf, city.Name)
			buf = append(buf, ',')
			buf = strconv.AppendInt(buf, int64(defKey), 10)
			buf = append(buf, ',')
			buf = strconv.AppendInt(buf, int64(defGrp), 10)

			var exceptions [][]any
			var lastNum int
			var lastIp uint32

			for _, srv := range city.Servers {
				isExc := !srv.NumIsInt || srv.KeyIdx != defKey || srv.GroupMask != defGrp || srv.DedupSuffix != "" || srv.HName != ""
				dIp := int64(srv.IpNum) - int64(lastIp)

				if isExc {
					packed := (-1 << 7) | (srv.Load & 0x7F)
					buf = append(buf, ',')
					buf = strconv.AppendInt(buf, int64(packed), 10)
					buf = append(buf, ',')
					buf = strconv.AppendInt(buf, dIp, 10)
					lastIp = srv.IpNum

					var idVal any
					if srv.NumIsInt {
						idVal = srv.NumInt
						lastNum = srv.NumInt
					} else {
						idVal = srv.Number
					}

					kOvr := -1
					if srv.KeyIdx != defKey {
						kOvr = srv.KeyIdx
					}
					gOvr := -1
					if srv.GroupMask != defGrp {
						gOvr = srv.GroupMask
					}

					t := []any{idVal}
					if srv.DedupSuffix != "" {
						t = append(t, kOvr, gOvr, srv.HName, srv.DedupSuffix)
					} else if srv.HName != "" {
						t = append(t, kOvr, gOvr, srv.HName)
					} else if gOvr != -1 {
						t = append(t, kOvr, gOvr)
					} else if kOvr != -1 {
						t = append(t, kOvr)
					}

					exceptions = append(exceptions, t)
				} else {
					dNum := srv.NumInt - lastNum
					packed := (dNum << 7) | (srv.Load & 0x7F)
					buf = append(buf, ',')
					buf = strconv.AppendInt(buf, int64(packed), 10)
					buf = append(buf, ',')
					buf = strconv.AppendInt(buf, dIp, 10)
					lastNum = srv.NumInt
					lastIp = srv.IpNum
				}
			}

			if len(exceptions) > 0 {
				buf = append(buf, ',')
				excBytes, _ := json.Marshal(exceptions)
				buf = append(buf, excBytes...)
			}

			buf = append(buf, ']')
		}

		buf = append(buf, ']')
		buf = append(buf, ']')
	}

	buf = append(buf, ']', ']')

	return buf
}
