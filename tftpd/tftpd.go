//go:generate statik -src=./firmware -include=*.efi

package tftpd

import "context"

// TFTPd is the interface for usra to provide the TFTP daemon.
type TFTPd interface {
	Serve(ctx context.Context) error
}
