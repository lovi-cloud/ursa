package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"

	"github.com/safchain/ethtool"
	"github.com/vishvananda/netlink"
)

const (
	bondingConfigPath    = "/etc/network/interfaces.d/bond.cfg"
	interfacesConfigPath = "/etc/network/interfaces"
	interfacesConfig     = `# generated by ursa-bonder
source /etc/network/interfaces.d/*.cfg

auto lo
iface lo inet loopback
`
)

var (
	driver  string
	min     int
	vlan    int
	address string
	netmask string
	gateway string
	dns     string
	dryRun  bool
)

var configTmpl = template.Must(template.New("tmpl").Parse(`# generated by ursa-bonder
auto bond0.{{ .VLAN }}
iface bond0.{{ .VLAN }} inet static
        address {{ .Address }}
        gateway {{ .Gateway }}
        netmask {{ .Netmask }}
        dns-nameservers {{ .DNS }}
        vlan-raw-device bond0

auto bond0
iface bond0 inet manual
        bond-mode 4
        bond-miimon 100
        bond-lacp-rate 1
        bond-slaves {{ range $val := .Slaves }}{{ $val.Name }} {{ end }}
        bond-xmit-hash-policy layer2+3
{{ range $val := .Slaves }}
auto {{ $val.Name }}
iface {{ $val.Name }} inet manual
        bond-master bond0
{{ end }}
`))

type tmplParams struct {
	Slaves  []net.Interface
	VLAN    int
	Address string
	Netmask string
	Gateway string
	DNS     string
}

func main() {
	log.SetFlags(0)
	if err := run(context.Background(), os.Args[1:], os.Stdout, os.Stderr); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

func run(ctx context.Context, argv []string, outStream, errStream io.Writer) error {
	log.SetOutput(errStream)
	log.SetPrefix("[ursa-bonder] ")

	fs := flag.NewFlagSet(fmt.Sprintf("ursa-bonder (v%s rev:%s)", version, revision), flag.ContinueOnError)
	fs.SetOutput(errStream)
	fs.StringVar(&driver, "driver", "i40e", "target driver name")
	fs.IntVar(&min, "min", 2, "minimum bonding slaves")
	fs.IntVar(&vlan, "vlan", 0, "bonding interface vlan id")
	fs.StringVar(&address, "addr", "192.0.2.100", "interface address")
	fs.StringVar(&netmask, "mask", "255.255.254.0", "interface netmask")
	fs.StringVar(&gateway, "gw", "192.0.2.1", "interface gateway")
	fs.StringVar(&dns, "dns", "8.8.8.8", "interface dns")
	fs.BoolVar(&dryRun, "dry-run", false, "dry run")
	fs.Parse(argv)

	ethHandle, err := ethtool.NewEthtool()
	if err != nil {
		return fmt.Errorf("failed to get ethtool: %w", err)
	}
	defer ethHandle.Close()

	ifaces, err := net.Interfaces()
	if err != nil {
		return fmt.Errorf("failed to get interface list: %w", err)
	}

	var slaves []net.Interface
	for _, iface := range ifaces {
		name, err := ethHandle.DriverName(iface.Name)
		if err != nil {
			log.Printf("failed to get driver name interface=%s: %+v", iface.Name, err)
			continue
		}
		if name == driver {
			slaves = append(slaves, iface)
			_, err := netlink.LinkByName(iface.Name)
			if err != nil {
				log.Printf("failed to get link interaface=%s: %+v", iface.Name, err)
				continue
			}
			/*err = netlink.LinkSetDown(link)
			if err != nil {
				log.Printf("failed to link set down interface=%s: %+v", iface.Name, err)
			}*/
		}
	}
	if len(slaves) < min {
		return fmt.Errorf("could not get enough slave interfaces")
	}

	params := tmplParams{
		Slaves:  slaves,
		VLAN:    vlan,
		Address: address,
		Netmask: netmask,
		Gateway: gateway,
		DNS:     dns,
	}
	if !dryRun {
		var buff bytes.Buffer
		err = configTmpl.Execute(&buff, params)
		if err != nil {
			return fmt.Errorf("failed to execute template: %w", err)
		}
		err = ioutil.WriteFile(bondingConfigPath, buff.Bytes(), 0644)
		if err != nil {
			return fmt.Errorf("failed to write %s: %w", bondingConfigPath, err)
		}
		err = ioutil.WriteFile(interfacesConfigPath, []byte(interfacesConfig), 0644)
		if err != nil {
			return fmt.Errorf("failed to write %s: %w", interfacesConfigPath, err)
		}
		_, err = systemctlCmd(ctx, "restart", "networking")
		if err != nil {
			return err
		}
		_, err = ipCmd(ctx, "route", "replace", "default", "via", gateway)
		if err != nil {
			return err
		}
	} else {
		err = configTmpl.Execute(os.Stdout, params)
		if err != nil {
			return fmt.Errorf("failed to execute template: %w", err)
		}
	}

	return nil
}

func systemctlCmd(ctx context.Context, args ...string) (string, error) {
	return runCmd(ctx, "systemctl", args...)
}

func ipCmd(ctx context.Context, args ...string) (string, error) {
	return runCmd(ctx, "ip", args...)
}

func runCmd(ctx context.Context, cmd string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, cmd, args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to exec [%s %s] msg=%s: %w", cmd, strings.Join(args, " "), string(out), err)
	}
	return string(out), nil
}
