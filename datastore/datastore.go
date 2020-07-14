package datastore

import (
	"context"

	"github.com/whywaita/ursa/dhcpd"
	"github.com/whywaita/ursa/types"
)

// Datastore is an interface for usra to perform CRUD operations.
type Datastore interface {
	GetSubnetByID(ctx context.Context, subnetID int) (*dhcpd.Subnet, error)
	GetSubnetByMyAddress(ctx context.Context, myAddr types.IP) (*dhcpd.Subnet, error)
	CreateSubnet(ctx context.Context, subnet dhcpd.Subnet) (*dhcpd.Subnet, error)

	GetLease(ctx context.Context, mac types.HardwareAddr) (*dhcpd.Lease, error)
	CreateLease(ctx context.Context, subnetID int, mac types.HardwareAddr) (*dhcpd.Lease, error)

	Close() error
}
