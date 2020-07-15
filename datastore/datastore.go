package datastore

import (
	"context"

	"github.com/whywaita/ursa/dhcpd"
	"github.com/whywaita/ursa/types"
)

// Datastore is an interface for usra to perform CRUD operations.
type Datastore interface {
	GetManagementSubnet(ctx context.Context) (*dhcpd.Subnet, error)
	GetServiceSubnet(ctx context.Context) (*dhcpd.Subnet, error)
	CreateManagementSubnet(ctx context.Context, network types.IPNet, start, end types.IP) (*dhcpd.Subnet, error)
	CreateServiceSubnet(ctx context.Context, network types.IPNet, start, end, gateway, dnsServer types.IP) (*dhcpd.Subnet, error)

	GetLease(ctx context.Context, mac types.HardwareAddr) (*dhcpd.Lease, error)
	CreateLeaseFromManagementSubnet(ctx context.Context, mac types.HardwareAddr) (*dhcpd.Lease, error)
	CreateLeaseFromServiceSubnet(ctx context.Context, mac types.HardwareAddr) (*dhcpd.Lease, error)

	Close() error
}
