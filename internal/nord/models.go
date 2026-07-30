package nord

type RawServer struct {
	Station      string       `json:"station"`
	Hostname     string       `json:"hostname"`
	Load         int          `json:"load"`
	Locations    []Location   `json:"locations"`
	Groups       []Group      `json:"groups"`
	Technologies []Technology `json:"technologies"`
}

type Location struct {
	Country Country `json:"country"`
}

type Country struct {
	Name string `json:"name"`
	Code string `json:"code"`
	City City   `json:"city"`
}

type City struct {
	Name string `json:"name"`
}

type Group struct {
	Identifier string `json:"identifier"`
}

type Technology struct {
	Metadata []Metadata `json:"metadata"`
}

type Metadata struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type processedServer struct {
	country             string
	city                string
	countrySort         string
	citySort            string
	countryCode         string
	number              string
	numberValue         int
	numberIsInteger     bool
	publicKey           string
	load                int
	groupMask           int
	ipNumber            uint32
	hostnameOverride    string
	deduplicationSuffix string
}

type serverNode struct {
	number              string
	numberValue         int
	numberIsInteger     bool
	load                int
	ipNumber            uint32
	keyIndex            int
	groupMask           int
	hostnameOverride    string
	deduplicationSuffix string
}

type cityNode struct {
	name    string
	servers []serverNode
}

type countryNode struct {
	name        string
	countryCode string
	cities      []cityNode
}
