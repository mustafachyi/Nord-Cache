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

type ProcessedServer struct {
	Country        string
	City           string
	LowCode        string
	Number         string
	NumVal         int
	KeyIndex       int
	Load           int
	GroupMask      int
	RawCountryName string
	RawCityName    string
	DedupSuffix    string
	IpNum          uint32
	HName          string
}

type ServerNode struct {
	Number      string
	NumInt      int
	NumIsInt    bool
	Load        int
	IpNum       uint32
	KeyIdx      int
	GroupMask   int
	HName       string
	DedupSuffix string
}

type CityNode struct {
	Name    string
	Servers []ServerNode
}

type CountryNode struct {
	Name    string
	LowCode string
	Cities  []CityNode
}
