//go:generate statik -m -src=../assets/ipxe

package tftpd

import "context"

// TFTPd is the interface for usra to provide the TFTP daemon.
type TFTPd interface {
	Serve(ctx context.Context, addr string) error
}
