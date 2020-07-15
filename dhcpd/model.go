package dhcpd

import "github.com/whywaita/ursa/types"

// Subnet is subnet configuration.
type Subnet struct {
	ID        int         `yaml:"id" db:"id"`
	Network   types.IPNet `yaml:"network" db:"network"`
	Start     types.IP    `yaml:"start" db:"start"`
	End       types.IP    `yaml:"end" db:"end"`
	Gateway   *types.IP   `yaml:"gateway" db:"gateway"`
	DNSServer *types.IP   `yaml:"dns_server" db:"dns_server"`
}

// Lease is
type Lease struct {
	ID         int                `db:"id"`
	MACAddress types.HardwareAddr `db:"mac_address"`
	IPAddress  types.IP           `db:"ip_address"`
	SubnetID   int                `db:"subnet_id"`
}
