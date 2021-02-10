package dhcpd

import "github.com/lovi-cloud/ursa/types"

// Subnet is subnet configuration.
type Subnet struct {
	ID        int         `db:"id"`
	Network   types.IPNet `db:"network"`
	Start     types.IP    `db:"start"`
	End       types.IP    `db:"end"`
	Gateway   *types.IP   `db:"gateway"`
	DNSServer *types.IP   `db:"dns_server"`
}

// Lease is
type Lease struct {
	ID         int                `db:"id"`
	MACAddress types.HardwareAddr `db:"mac_address"`
	IPAddress  types.IP           `db:"ip_address"`
	SubnetID   int                `db:"subnet_id"`
}
