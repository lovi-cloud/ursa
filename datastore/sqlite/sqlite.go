package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"

	"github.com/go-test/deep"

	// SQLite driver
	_ "github.com/mattn/go-sqlite3"

	"github.com/jmoiron/sqlx"

	"github.com/whywaita/ursa/config"
	"github.com/whywaita/ursa/datastore"
	"github.com/whywaita/ursa/dhcpd"
	"github.com/whywaita/ursa/types"
)

// SQLite is
type SQLite struct {
	db *sqlx.DB
}

// New is
func New(ctx context.Context, dsn string, conf config.Config) (datastore.Datastore, error) {
	db, err := sqlx.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite connection: %w", err)
	}

	err = createTable(db)
	if err != nil {
		return nil, err
	}

	ds := &SQLite{
		db: db,
	}
	for _, subnet := range conf.Subnets {
		currentSubnet, err := ds.GetSubnetByID(ctx, subnet.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				_, err = ds.CreateSubnet(ctx, subnet)
				if err != nil {
					return nil, err
				}
				continue
			}
			return nil, err
		}
		if diff := deep.Equal(subnet, *currentSubnet); len(diff) != 0 {
			return nil, fmt.Errorf("invalid: %+v", diff)
		}
	}

	return ds, nil
}

// GetSubnetByID is
func (s *SQLite) GetSubnetByID(ctx context.Context, subnetID int) (*dhcpd.Subnet, error) {
	query := `SELECT id, gateway, netmask, my_address, start, end, dns_server FROM subnet WHERE id = ?`
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

// GetSubnetByMyAddress is
func (s *SQLite) GetSubnetByMyAddress(ctx context.Context, myAddr types.IP) (*dhcpd.Subnet, error) {
	query := `SELECT id, gateway, netmask, my_address, start, end, dns_server FROM subnet WHERE my_address = ?`
	stmt, err := s.db.Preparex(query)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare statement: %w", err)
	}
	var subnet dhcpd.Subnet
	err = stmt.GetContext(ctx, &subnet, myAddr)
	if err != nil {
		fmt.Printf("%+v\n", myAddr)
		return nil, fmt.Errorf("failed to get subnet: %w", err)
	}
	return &subnet, nil
}

// CreateSubnet is
func (s *SQLite) CreateSubnet(ctx context.Context, subnet dhcpd.Subnet) (*dhcpd.Subnet, error) {
	query := `INSERT INTO subnet(id, gateway, netmask, my_address, start, end, dns_server) VALUES(?, ?, ?, ?, ?, ?, ?)`
	stmt, err := s.db.Preparex(query)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare stetment: %w", err)
	}
	_, err = stmt.ExecContext(ctx, subnet.ID, subnet.Gateway, subnet.Netmask, subnet.MyAddress, subnet.Start, subnet.End, subnet.DNSServer)
	if err != nil {
		return nil, fmt.Errorf("failed to create new subnet: %w", err)
	}
	return &subnet, nil
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

// CreateLease is
func (s *SQLite) CreateLease(ctx context.Context, subnetID int, mac types.HardwareAddr) (*dhcpd.Lease, error) {
	subnet, err := s.GetSubnetByID(ctx, subnetID)
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
