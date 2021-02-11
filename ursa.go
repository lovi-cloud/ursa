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


	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/lovi-cloud/ursa/datastore/sqlite"
	"github.com/lovi-cloud/ursa/types"
	"github.com/lovi-cloud/ursa/dhcpd/godhcpd"
	"github.com/lovi-cloud/ursa/httpd/gohttpd"
	"github.com/lovi-cloud/ursa/tftpd/gotftpd"
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
		staticDir string

		serviceNetwork string
		serviceRange   string
		serviceGateway string
		serviceDNS     string
		hostnamePrefix string
	)
	flags := flag.NewFlagSet(fmt.Sprintf("ursa (v%s rev:%s)", version, revision), flag.ContinueOnError)
	flags.StringVar(&dsn, "dsn", "file:ursa.db?cache=shared", "sqlite3 dsn")
	flags.StringVar(&iface, "iface", "eth0", "ursa listening interface")
	flags.StringVar(&dhcpRange, "dhcp-range", "192.0.2.100:192.0.2.200", "START:END")
	flags.StringVar(&staticDir, "static-dir", "./static", "static assets directory path")
	flags.StringVar(&serviceNetwork, "service-nw", "198.51.100.0/24", "service network CIDR")
	flags.StringVar(&serviceRange, "service-range", "198.51.100.100:198.51.100.200", "START:END")
	flags.StringVar(&serviceGateway, "service-gw", "198.51.100.1", "service network gateway")
	flags.StringVar(&serviceDNS, "service-dns", "8.8.8.8", "service network dns server")
	flags.StringVar(&hostnamePrefix, "hostname-prefix", "cn", "hostname prefix (prefixNNNN)")
	flags.Parse(os.Args[1:])

	ip, inet, err := getInterfaceAddress(iface)
	if err != nil {
		return err
	}
	dhspStart, dhcpEnd, err := parseRange(dhcpRange, inet)
	if err != nil {
		return err
	}
	_, serviceNet, err := net.ParseCIDR(serviceNetwork)
	if err != nil {
		return err
	}
	serviceStart, serviceEnd, err := parseRange(serviceRange, serviceNet)
	if err != nil {
		return err
	}
	serviceGW := net.ParseIP(serviceGateway)
	if serviceGW == nil {
		return fmt.Errorf("failed to parse service-gw %s", serviceGateway)
	}
	if !serviceNet.Contains(serviceGW) {
		return fmt.Errorf("invalid service-gw %s", serviceGateway)
	}
	dns := net.ParseIP(serviceDNS)
	if dns == nil {
		return fmt.Errorf("failed to parse service-dns %s", serviceDNS)
	}

	ds, err := sqlite.New(ctx, dsn, hostnamePrefix)
	if err != nil {
		return err
	}
	defer ds.Close()
	_, err = ds.CreateManagementSubnet(ctx, types.IPNet(*inet), types.IP(dhspStart), types.IP(dhcpEnd))
	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) {
		if sqliteErr.ExtendedCode != sqlite3.ErrConstraintPrimaryKey {
			return err
		}
		logger.Warn("management subnet already exists")
	} else if err != nil {
		return err
	}
	_, err = ds.CreateServiceSubnet(ctx, types.IPNet(*serviceNet), types.IP(serviceStart), types.IP(serviceEnd), types.IP(serviceGW), types.IP(dns))
	if errors.As(err, &sqliteErr) {
		if sqliteErr.ExtendedCode != sqlite3.ErrConstraintPrimaryKey {
			return err
		}
		logger.Warn("service subnet already exists")
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

func parseRange(input string, inet *net.IPNet) (net.IP, net.IP, error) {
	words := strings.Split(input, ":")
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
