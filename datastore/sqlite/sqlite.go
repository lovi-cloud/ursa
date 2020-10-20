package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"

	uuid "github.com/satori/go.uuid"
	"github.com/whywaita/ursa/httpd"

	// SQLite driver
	_ "github.com/mattn/go-sqlite3"

	"github.com/jmoiron/sqlx"

	"github.com/whywaita/ursa/datastore"
	"github.com/whywaita/ursa/dhcpd"
	"github.com/whywaita/ursa/types"
)

const (
	managementSubnetID = 0
	serviceSubnetID    = 1
)

// SQLite is
type SQLite struct {
	db             *sqlx.DB
	hostnamePrefix string
}

// New is
func New(ctx context.Context, dsn, hostnamePrefix string) (datastore.Datastore, error) {
	db, err := sqlx.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite connection: %w", err)
	}

	err = createTable(db)
	if err != nil {
		return nil, err
	}

	return &SQLite{
		db:             db,
		hostnamePrefix: hostnamePrefix,
	}, nil
}

func (s *SQLite) getSubnetByID(ctx context.Context, subnetID int) (*dhcpd.Subnet, error) {
	query := `SELECT id, network, start, end, gateway, dns_server FROM subnet WHERE id = ?`
	stmt, err := s.db.Preparex(query)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare statement: %w", err)
	}
	var subnet dhcpd.Subnet
	err = stmt.GetContext(ctx, &subnet, subnetID)
	if err != nil {
		return nil, fmt.Errorf("failed to get subnet: %w", err)
	}
	return &subnet, nil
}

// GetManagementSubnet is
func (s *SQLite) GetManagementSubnet(ctx context.Context) (*dhcpd.Subnet, error) {
	return s.getSubnetByID(ctx, managementSubnetID)
}

// GetServiceSubnet is
func (s *SQLite) GetServiceSubnet(ctx context.Context) (*dhcpd.Subnet, error) {
	return s.getSubnetByID(ctx, serviceSubnetID)
}

func (s *SQLite) createSubnet(ctx context.Context, subnet dhcpd.Subnet) (*dhcpd.Subnet, error) {
	query := `INSERT INTO subnet(id, network, start, end, gateway, dns_server) VALUES(?, ?, ?, ?, ?, ?)`
	stmt, err := s.db.Preparex(query)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare stetment: %w", err)
	}
	_, err = stmt.ExecContext(ctx, subnet.ID, subnet.Network, subnet.Start, subnet.End, subnet.Gateway, subnet.DNSServer)
	if err != nil {
		return nil, fmt.Errorf("failed to create new subnet: %w", err)
	}
	return &subnet, nil
}

// CreateManagementSubnet is
func (s *SQLite) CreateManagementSubnet(ctx context.Context, network types.IPNet, start, end types.IP) (*dhcpd.Subnet, error) {
	return s.createSubnet(ctx, dhcpd.Subnet{
		ID:        managementSubnetID,
		Network:   network,
		Start:     start,
		End:       end,
		Gateway:   nil,
		DNSServer: nil,
	})
}

// CreateServiceSubnet is
func (s *SQLite) CreateServiceSubnet(ctx context.Context, network types.IPNet, start, end, gateway, dnsServer types.IP) (*dhcpd.Subnet, error) {
	return s.createSubnet(ctx, dhcpd.Subnet{
		ID:        serviceSubnetID,
		Network:   network,
		Start:     start,
		End:       end,
		Gateway:   &gateway,
		DNSServer: &dnsServer,
	})
}

func (s *SQLite) getLease(ctx context.Context, subnetID int, mac types.HardwareAddr) (*dhcpd.Lease, error) {
	query := `SELECT id, mac_address, ip_address, subnet_id FROM lease WHERE subnet_id = ? AND mac_address = ?`
	stmt, err := s.db.Preparex(query)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare stetment: %w", err)
	}
	var lease dhcpd.Lease
	err = stmt.GetContext(ctx, &lease, subnetID, mac)
	if err != nil {
		return nil, fmt.Errorf("failed to get lease: %w", err)
	}

	return &lease, nil
}

// GetLeaseByID is
func (s *SQLite) GetLeaseByID(ctx context.Context, id int) (*httpd.Lease, error) {
	query := `SELECT lease.id AS id, ip_address, network, gateway, dns_server FROM lease JOIN subnet ON lease.subnet_id = subnet.id WHERE lease.id = ?`
	stmt, err := s.db.Preparex(query)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare stetment: %w", err)
	}
	var lease httpd.Lease
	err = stmt.GetContext(ctx, &lease, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get lease: %w", err)
	}

	return &lease, nil
}

// GetLeaseFromManagementSubnet is
func (s *SQLite) GetLeaseFromManagementSubnet(ctx context.Context, mac types.HardwareAddr) (*dhcpd.Lease, error) {
	return s.getLease(ctx, managementSubnetID, mac)
}

// GetLeaseFromServiceSubnet is
func (s *SQLite) GetLeaseFromServiceSubnet(ctx context.Context, mac types.HardwareAddr) (*dhcpd.Lease, error) {
	return s.getLease(ctx, serviceSubnetID, mac)
}

func (s *SQLite) createLease(ctx context.Context, subnetID int, mac types.HardwareAddr) (*dhcpd.Lease, error) {
	subnet, err := s.getSubnetByID(ctx, subnetID)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.Beginx()
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	query := `SELECT id, mac_address, ip_address, subnet_id FROM lease WHERE subnet_id = ? ORDER BY ip_address DESC LIMIT 1`
	var latest dhcpd.Lease
	var next types.IP
	stmt, err := tx.Preparex(query)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare statement: %w", err)
	}
	err = stmt.GetContext(ctx, &latest, subnetID)
	if errors.Is(err, sql.ErrNoRows) {
		next = subnet.Start
	} else if err == nil {
		next = getNextAddress(latest.IPAddress)
	} else {
		return nil, fmt.Errorf("failed to get latest lease: %w", err)
	}

	query = `INSERT INTO lease(mac_address, ip_address, subnet_id) VALUES(?, ?, ?)`
	stmt, err = tx.Preparex(query)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare stetment: %w", err)
	}
	ret, err := stmt.ExecContext(ctx, mac, next, subnetID)
	if err != nil {
		return nil, fmt.Errorf("failed to create new lease: %w", err)
	}
	id, err := ret.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get inserted id: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &dhcpd.Lease{
		ID:         int(id),
		MACAddress: mac,
		IPAddress:  next,
		SubnetID:   subnetID,
	}, nil
}

// CreateLeaseFromManagementSubnet is
func (s *SQLite) CreateLeaseFromManagementSubnet(ctx context.Context, mac types.HardwareAddr) (*dhcpd.Lease, error) {
	return s.createLease(ctx, managementSubnetID, mac)
}

// CreateLeaseFromServiceSubnet is
func (s *SQLite) CreateLeaseFromServiceSubnet(ctx context.Context, mac types.HardwareAddr) (*dhcpd.Lease, error) {
	return s.createLease(ctx, serviceSubnetID, mac)
}

func (s *SQLite) generateHostname(ctx context.Context) (string, error) {
	query := `SELECT COALESCE(MAX(id), 0) AS id FROM host`
	var id int
	stmt, err := s.db.Preparex(query)
	if err != nil {
		return "", fmt.Errorf("failed to prepare statement: %w", err)
	}
	err = stmt.GetContext(ctx, &id)
	if err != nil {
		return "", fmt.Errorf("failed to get maximum id: %w", err)
	}
	return fmt.Sprintf("%s%04d", s.hostnamePrefix, id+1), nil
}

// RegisterHost is
func (s *SQLite) RegisterHost(ctx context.Context, serverID uuid.UUID, serial, product, manufacturer string, serviceLeaseID, managementLeaseID int) (*httpd.Host, error) {
	name, err := s.generateHostname(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to generate hostname: %w", err)
	}
	host := httpd.Host{
		UUID:              serverID,
		Name:              name,
		ServiceLeaseID:    serviceLeaseID,
		ManagementLeaseID: managementLeaseID,
	}

	query := `INSERT INTO host(uuid, name, serial, product, manufacturer, service_lease_id, management_lease_id) VALUES(?, ?, ?, ?, ?, ?, ?)`
	stmt, err := s.db.Preparex(query)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare statement: %w", err)
	}
	ret, err := stmt.ExecContext(ctx, host.UUID, host.Name, serial, product, manufacturer, host.ServiceLeaseID, host.ManagementLeaseID)
	if err != nil {
		return nil, fmt.Errorf("failed to create new host: %w", err)
	}
	id, err := ret.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get inserted id: %w", err)
	}
	host.ID = int(id)

	return &host, nil
}

// GetHostByAddress is
func (s *SQLite) GetHostByAddress(ctx context.Context, address types.IP) (*httpd.Host, error) {
	query := `SELECT host.id AS id, uuid, name, service_lease_id, management_lease_id FROM lease JOIN host ON lease.id = host.management_lease_id WHERE ip_address = ?`
	stmt, err := s.db.Preparex(query)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare statement: %w", err)
	}
	var host httpd.Host
	err = stmt.GetContext(ctx, &host, address)
	if err != nil {
		return nil, fmt.Errorf("failed to get host: %w", err)
	}
	return &host, nil
}

// ListUser is
func (s *SQLite) ListUser(ctx context.Context) ([]httpd.User, error) {
	query := `SELECT id, name FROM user`
	stmt, err := s.db.Preparex(query)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare statement: %w", err)
	}
	var users []httpd.User
	err = stmt.SelectContext(ctx, &users)
	if err != nil {
		return nil, fmt.Errorf("failed to get user list: %w", err)
	}
	return users, nil
}

// ListKeyByUserID is
func (s *SQLite) ListKeyByUserID(ctx context.Context, userID int) ([]httpd.Key, error) {
	query := `SELECT id, key, user_id FROM key WHERE user_id = ?`
	stmt, err := s.db.Preparex(query)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare statement: %w", err)
	}
	var keys []httpd.Key
	err = stmt.SelectContext(ctx, &keys, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get key list: %w", err)
	}
	return keys, nil
}

// Close closes the database connections.
func (s *SQLite) Close() error {
	return s.db.Close()
}

func createTable(db *sqlx.DB) error {
	for _, table := range tables {
		_, err := db.Exec(table)
		if err != nil {
			return fmt.Errorf("failed to create lease tables: %w", err)
		}
	}
	return nil
}

func getNextAddress(ip types.IP) types.IP {
	a := net.ParseIP(ip.String())
	for i := len(a) - 1; i >= 0; i-- {
		a[i]++
		if a[i] > 0 {
			break
		}
	}
	return types.IP(a)
}
