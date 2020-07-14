package dhcpd

import "context"

// DHCPd is the interface for usra to provide the DHCP daemon.
type DHCPd interface {
	Serve(ctx context.Context) error
}
