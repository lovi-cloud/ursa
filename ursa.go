package ursa

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"

	sqlite3 "github.com/mattn/go-sqlite3"

	"github.com/rakyll/statik/fs"

	"github.com/whywaita/ursa/types"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

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

	var (
		dsn       string
		iface     string
		dhcpRange string
	)
	flags := flag.NewFlagSet(fmt.Sprintf("ursa (v%s rev:%s)", version, revision), flag.ContinueOnError)
	flags.StringVar(&dsn, "dsn", "file:ursa.db?cache=shared", "sqlite3 dsn")
	flags.StringVar(&iface, "iface", "eth0", "ursa listening interface")
	flags.StringVar(&dhcpRange, "dhcp-range", "192.0.2.100:192.0.2.200", "START:END")
	flags.Parse(os.Args[1:])

	ip, inet, err := getInterfaceAddress(iface)
	if err != nil {
		return err
	}
	start, end, err := parseDHCPRange(dhcpRange, inet)
	if err != nil {
		return err
	}

	ds, err := sqlite.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer ds.Close()
	_, err = ds.CreateManagementSubnet(ctx, types.IPNet(*inet), types.IP(start), types.IP(end))
	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) {
		if sqliteErr.ExtendedCode != sqlite3.ErrConstraintPrimaryKey {
			return err
		}
		logger.Warn("management subnet already exists")
	} else if err != nil {
		return err
	}

	eg, ctx := errgroup.WithContext(ctx)

	dhcpd, err := godhcpd.New(ds, logger)
	if err != nil {
		return err
	}
	eg.Go(func() error {
		logger.Info("starting dhcpd", zap.String("addr", fmt.Sprintf("%s:67", ip)))
		return dhcpd.Serve(ctx, ip, iface)
	})

	statikFS, err := fs.New()
	if err != nil {
		return err
	}
	tftpd, err := gotftpd.New(statikFS, logger)
	if err != nil {
		return err
	}
	eg.Go(func() error {
		addr := fmt.Sprintf("%s:69", ip)
		logger.Info("starting tftpd", zap.String("addr", addr))
		return tftpd.Serve(ctx, addr)
	})

	httpd, err := gohttpd.New(ds, logger)
	if err != nil {
		return err
	}
	eg.Go(func() error {
		addr := fmt.Sprintf("%s:80", ip)
		logger.Info("starting httpd", zap.String("addr", addr))
		return httpd.Serve(ctx, addr)
	})

	return eg.Wait()
}

func getInterfaceAddress(name string) (net.IP, *net.IPNet, error) {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find interface %s: %w", name, err)
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get interface addresses %s: %w", name, err)
	}
	for _, addr := range addrs {
		ip, inet, err := net.ParseCIDR(addr.String())
		if err != nil {
			continue
		}
		if ip.To4() != nil {
			return ip, inet, nil
		}
	}

	return nil, nil, fmt.Errorf("failed to find interface address %s", name)
}

func parseDHCPRange(dhcpRange string, inet *net.IPNet) (net.IP, net.IP, error) {
	words := strings.Split(dhcpRange, ":")
	if len(words) != 2 {
		return nil, nil, fmt.Errorf("invalid format")
	}
	start := net.ParseIP(words[0])
	if start == nil {
		return nil, nil, fmt.Errorf("failed to parse start address %s", start)
	}
	if !inet.Contains(start) {
		return nil, nil, fmt.Errorf("invalid addrass range %s", start)
	}
	end := net.ParseIP(words[1])
	if end == nil {
		return nil, nil, fmt.Errorf("failed to parse end address %s", end)
	}
	if !inet.Contains(end) {
		return nil, nil, fmt.Errorf("invalid address range %s", end)
	}
	return start, end, nil
}
