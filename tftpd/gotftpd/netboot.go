package gotftpd

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"

	"github.com/rakyll/statik/fs"
	"go.uber.org/zap"
	"go.universe.tf/netboot/tftp"

	// import ipxe.efi
	_ "github.com/whywaita/ursa/tftpd/statik"

	"github.com/whywaita/ursa/tftpd"
)

// Netboot is
type Netboot struct {
	addr   string
	fs     http.FileSystem
	logger *zap.Logger
}

// New is
func New(addr string, logger *zap.Logger) (tftpd.TFTPd, error) {
	statikFS, err := fs.New()
	if err != nil {
		return nil, err
	}

	return &Netboot{
		addr:   addr,
		fs:     statikFS,
		logger: logger,
	}, nil
}

// Serve is
func (n *Netboot) Serve(ctx context.Context) error {
	l, err := net.ListenPacket("udp4", n.addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", n.addr, err)
	}
	defer l.Close()

	server := &tftp.Server{
		Handler: n.handler,
		InfoLog: func(msg string) {
			n.logger.Info("info log", zap.String("msg", msg))
		},
		TransferLog: func(clientAddr net.Addr, path string, err error) {
			if err != nil {
				n.logger.Error("transfer", zap.String("path", path), zap.String("client", clientAddr.String()), zap.Error(err))
			} else {
				n.logger.Info("transfer", zap.String("path", path), zap.String("client", clientAddr.String()))
			}
		},
	}

	return server.Serve(l)
}

func (n *Netboot) handler(path string, clientAddr net.Addr) (io.ReadCloser, int64, error) {
	f, err := n.fs.Open(filepath.Join("/", path))
	if err != nil {
		return nil, -1, fmt.Errorf("failed to open path %s: %w", path, err)
	}
	s, err := f.Stat()
	if err != nil {
		return nil, -1, fmt.Errorf("faield to get %s stat: %w", path, err)
	}
	return f, s.Size(), nil
}
