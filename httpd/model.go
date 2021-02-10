package httpd

import (
	uuid "github.com/satori/go.uuid"

	"github.com/lovi-cloud/ursa/types"
)

// Host is
type Host struct {
	ID                int       `db:"id"`
	UUID              uuid.UUID `db:"uuid"`
	Name              string    `db:"name"`
	ServiceLeaseID    int       `db:"service_lease_id"`
	ManagementLeaseID int       `db:"management_lease_id"`
}

// Lease is
type Lease struct {
	ID        int         `db:"id"`
	IPAddress types.IP    `db:"ip_address"`
	Network   types.IPNet `db:"network"`
	Gateway   *types.IP   `db:"gateway"`
	DNSServer *types.IP   `db:"dns_server"`
}

// User is
type User struct {
	ID   int    `db:"id"`
	Name string `db:"name"`
}

// Key is
type Key struct {
	ID     int    `db:"id"`
	Key    string `db:"key"`
	UserID int    `db:"user_id"`
}
