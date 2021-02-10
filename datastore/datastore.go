package datastore

import (
	"context"

	uuid "github.com/satori/go.uuid"

	"github.com/lovi-cloud/ursa/dhcpd"
	"github.com/lovi-cloud/ursa/httpd"
	"github.com/lovi-cloud/ursa/types"
)

// Datastore is an interface for usra to perform CRUD operations.
type Datastore interface {
	GetManagementSubnet(ctx context.Context) (*dhcpd.Subnet, error)
	GetServiceSubnet(ctx context.Context) (*dhcpd.Subnet, error)
	CreateManagementSubnet(ctx context.Context, network types.IPNet, start, end types.IP) (*dhcpd.Subnet, error)
	CreateServiceSubnet(ctx context.Context, network types.IPNet, start, end, gateway, dnsServer types.IP) (*dhcpd.Subnet, error)

	GetLeaseByID(ctx context.Context, id int) (*httpd.Lease, error)
	GetLeaseFromManagementSubnet(ctx context.Context, mac types.HardwareAddr) (*dhcpd.Lease, error)
	GetLeaseFromServiceSubnet(ctx context.Context, mac types.HardwareAddr) (*dhcpd.Lease, error)
	CreateLeaseFromManagementSubnet(ctx context.Context, mac types.HardwareAddr) (*dhcpd.Lease, error)
	CreateLeaseFromServiceSubnet(ctx context.Context, mac types.HardwareAddr) (*dhcpd.Lease, error)

	RegisterHost(ctx context.Context, serverID uuid.UUID, serial, product, manufacturer string, serviceLeaseID, managementLeaseID int) (*httpd.Host, error)
	GetHostByAddress(ctx context.Context, address types.IP) (*httpd.Host, error)

	ListUser(ctx context.Context) ([]httpd.User, error)
	ListKeyByUserID(ctx context.Context, userID int) ([]httpd.Key, error)

	Close() error
}
