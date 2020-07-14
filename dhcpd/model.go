package dhcpd

import "github.com/whywaita/ursa/types"

// Subnet is subnet configuration.
type Subnet struct {
	ID        int          `yaml:"id" db:"id"`
	Gateway   types.IP     `yaml:"gateway" db:"gateway"`
	Netmask   types.IPMask `yaml:"netmask" db:"netmask"`
	MyAddress types.IP     `yaml:"my_address" db:"my_address"`
	Start     types.IP     `yaml:"start" db:"start"`
	End       types.IP     `yaml:"end" db:"end"`
	DNSServer types.IP     `yaml:"dns_server" db:"dns_server"`
}

// Lease is
type Lease struct {
	ID         int                `db:"id"`
	MACAddress types.HardwareAddr `db:"mac_address"`
	IPAddress  types.IP           `db:"ip_address"`
	SubnetID   int                `db:"subnet_id"`
}
