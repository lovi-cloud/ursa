package ursa

import (
	"context"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/whywaita/ursa/config"
	"github.com/whywaita/ursa/datastore/sqlite"
	"github.com/whywaita/ursa/dhcpd/godhcpd"
	"github.com/whywaita/ursa/httpd/gohttpd"
	"github.com/whywaita/ursa/tftpd/gotftpd"
)

// Run the ursa
func Run(ctx context.Context) error {
	logger, err := zap.NewProduction()
	if err != nil {
		return err
	}

	conf, err := config.LoadConfig("./config.yml")
	if err != nil {
		return err
	}

	dsn := "file:test.db?cache=shared"
	ds, err := sqlite.New(ctx, dsn, *conf)
	if err != nil {
		return err
	}
	defer ds.Close()

	eg, ctx := errgroup.WithContext(ctx)

	dhcpd, err := godhcpd.New("0.0.0.0:67", ds, logger)
	if err != nil {
		return err
	}
	eg.Go(func() error {
		logger.Info("starting dhcpd", zap.String("addr", "0.0.0.0:67"))
		return dhcpd.Serve(ctx)
	})

	tftpd, err := gotftpd.New("0.0.0.0:69", logger)
	if err != nil {
		return err
	}
	eg.Go(func() error {
		logger.Info("starting tftpd", zap.String("addr", "0.0.0.0:69"))
		return tftpd.Serve(ctx)
	})

	httpd, err := gohttpd.New("0.0.0.0:80", ds, logger)
	if err != nil {
		return err
	}
	eg.Go(func() error {
		logger.Info("starting httpd", zap.String("addr", "0.0.0.0:80"))
		return httpd.Serve(ctx)
	})

	return eg.Wait()
}
