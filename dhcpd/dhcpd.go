package dhcpd

import (
	"context"
	"net"
)

// DHCPd is the interface for usra to provide the DHCP daemon.
type DHCPd interface {
	Serve(ctx context.Context, addr net.IP, iface string) error
}
