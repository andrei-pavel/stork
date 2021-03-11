package keadata

// Represents a DHCPv4 lease fetched from Kea.
type Lease4 struct {
	ClientID      string `mapstructure:"client-id" json:"client-id,omitempty"`
	Cltt          uint64 `mapstructure:"cltt" json:"cltt,omitempty"`
	FqdnFwd       bool   `mapstructure:"fqdn-fwd" json:"fqdn-fwd,omitempty"`
	FqdnRev       bool   `mapstructure:"fqdn-rev" json:"fqdn-rev,omitempty"`
	Hostname      string `mapstructure:"hostname" json:"hostname,omitempty"`
	HWAddress     string `mapstructure:"hw-address" json:"hw-address,omitempty"`
	IPAddress     string `mapstructure:"ip-address" json:"ip-address,omitempty"`
	State         int    `mapstructure:"state" json:"state,omitempty"`
	SubnetID      uint32 `mapstructure:"subnet-id" json:"subnet-id,omitempty"`
	UserContext   string `mapstructure:"user-context" json:"user-context,omitempty"`
	ValidLifetime uint32 `mapstructure:"valid-lft" json:"valid-lft,omitempty"`
}

// Represents a DHCPv6 lease fetched from Kea.
type Lease6 struct {
	Cltt              uint64 `mapstructure:"cltt" json:"cltt,omitempty"`
	DUID              string `mapstructure:"duid" json:"duid,omitempty"`
	FqdnFwd           bool   `mapstructure:"fqdn-fwd" json:"fqdn-fwd,omitempty"`
	FqdnRev           bool   `mapstructure:"fqdn-rev" json:"fqdn-rev,omitempty"`
	Hostname          string `mapstructure:"hostname" json:"hostname,omitempty"`
	HWAddress         string `mapstructure:"hw-address" json:"hw-address,omitempty"`
	IAID              uint32 `mapstructure:"iaid" json:"iaid,omitempty"`
	IPAddress         string `mapstructure:"ip-address" json:"ip-address,omitempty"`
	PreferredLifetime uint32 `mapstructure:"preferred-lft" json:"preferred-lft,omitempty"`
	PrefixLength      uint8  `mapstructure:"prefix-len" json:"prefix-len,omitempty"`
	State             int    `mapstructure:"state" json:"state,omitempty"`
	SubnetID          uint32 `mapstructure:"subnet-id" json:"subnet-id,omitempty"`
	Type              string `mapstructure:"type" json:"type,omitempty"`
	UserContext       string `mapstructure:"user-context" json:"user-context,omitempty"`
	ValidLifetime     uint32 `mapstructure:"valid-lft" json:"valid-lft,omitempty"`
}
