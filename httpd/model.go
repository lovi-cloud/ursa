package httpd

import uuid "github.com/satori/go.uuid"

// Host is
type Host struct {
	ID      int       `db:"id"`
	UUID    uuid.UUID `db:"uuid"`
	Name    string    `db:"name"`
	LeaseID int       `db:"lease_id"`
}
