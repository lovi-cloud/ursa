package datastore

import (
	"context"

	uuid "github.com/satori/go.uuid"

	"github.com/whywaita/ursa/dhcpd"
	"github.com/whywaita/ursa/httpd"
	"github.com/whywaita/ursa/types"
)

// Datastore is an interface for usra to perform CRUD operations.
type Datastore interface {
	GetManagementSubnet(ctx context.Context) (*dhcpd.Subnet, error)
	GetServiceSubnet(ctx context.Context) (*dhcpd.Subnet, error)
	CreateManagementSubnet(ctx context.Context, network types.IPNet, start, end types.IP) (*dhcpd.Subnet, error)
	CreateServiceSubnet(ctx context.Context, network types.IPNet, start, end, gateway, dnsServer types.IP) (*dhcpd.Subnet, error)

	GetLeaseFromManagementSubnet(ctx context.Context, mac types.HardwareAddr) (*dhcpd.Lease, error)
	CreateLeaseFromManagementSubnet(ctx context.Context, mac types.HardwareAddr) (*dhcpd.Lease, error)
	CreateLeaseFromServiceSubnet(ctx context.Context, mac types.HardwareAddr) (*dhcpd.Lease, error)

	RegisterHost(ctx context.Context, serverID uuid.UUID, leaseID int) (*httpd.Host, error)

	Close() error
}
