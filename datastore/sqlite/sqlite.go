package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"

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
	db *sqlx.DB
}

// New is
func New(ctx context.Context, dsn string) (datastore.Datastore, error) {
	db, err := sqlx.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite connection: %w", err)
	}

	err = createTable(db)
	if err != nil {
		return nil, err
	}

	return &SQLite{
		db: db,
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

// GetLease is
func (s *SQLite) GetLease(ctx context.Context, mac types.HardwareAddr) (*dhcpd.Lease, error) {
	query := `SELECT id, mac_address, ip_address, subnet_id FROM lease WHERE mac_address = ?`
	stmt, err := s.db.Preparex(query)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare stetment: %w", err)
	}
	var lease dhcpd.Lease
	err = stmt.GetContext(ctx, &lease, mac)
	if err != nil {
		return nil, fmt.Errorf("failed to get lease: %w", err)
	}

	return &lease, nil
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
