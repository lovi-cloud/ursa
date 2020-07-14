package godhcpd

import (
	"context"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"net"

	"github.com/whywaita/ursa/types"

	"go.uber.org/zap"
	"go.universe.tf/netboot/dhcp4"

	"github.com/whywaita/ursa/datastore"
	"github.com/whywaita/ursa/dhcpd"
)

var errAddressNotFound = fmt.Errorf("failed to find address")

// Netboot is
type Netboot struct {
	addr   string
	ds     datastore.Datastore
	logger *zap.Logger
}

// New is
func New(addr string, ds datastore.Datastore, logger *zap.Logger) (dhcpd.DHCPd, error) {
	return &Netboot{
		addr:   addr,
		ds:     ds,
		logger: logger,
	}, nil
}

// Serve serve dhcp daemon.
func (n *Netboot) Serve(ctx context.Context) error {
	conn, err := dhcp4.NewConn(n.addr)
	if err != nil {
		return fmt.Errorf("failed to create nwe connection: %w", err)
	}
	defer conn.Close()

	for {
		req, intf, err := conn.RecvDHCP()
		if err != nil {
			n.logger.Error("failed to receive dhcp request", zap.Error(err))
			continue
		}
		n.logger.Info("received request", zap.String("req", fmt.Sprintf("%+v", req)))
		addr, err := getAddr(intf)
		if err == errAddressNotFound {
			continue
		}
		if err != nil {
			n.logger.Error("failed to get interface address", zap.Error(err))
			continue
		}

		subnet, err := n.ds.GetSubnetByMyAddress(ctx, types.IP(*addr))
		if err != nil {
			n.logger.Error("failed to get subnet", zap.Error(err))
			continue
		}
		var lease *dhcpd.Lease
		lease, err = n.ds.GetLease(ctx, types.HardwareAddr(req.HardwareAddr))
		if err != nil && errors.Is(err, sql.ErrNoRows) {
			lease, err = n.ds.CreateLease(ctx, subnet.ID, types.HardwareAddr(req.HardwareAddr))
		}
		if err != nil {
			n.logger.Error("failed to get lease", zap.Error(err))
			continue
		}
		resp, err := makeResponse(*req, *subnet, *lease)
		if err != nil {
			n.logger.Error("failed to make response", zap.Error(err))
			continue
		}
		err = conn.SendDHCP(resp, intf)
		if err != nil {
			n.logger.Error("failed to send dhcp response", zap.Error(err))
			continue
		}
		n.logger.Info("send DCHP response", zap.String("resp", fmt.Sprintf("%+v", resp)))
	}
}

func makeResponse(req dhcp4.Packet, subnet dhcpd.Subnet, lease dhcpd.Lease) (*dhcp4.Packet, error) {
	serverAddr := net.IP(subnet.MyAddress).To4()
	yourAddr := net.IP(lease.IPAddress)

	resp := &dhcp4.Packet{
		TransactionID:  req.TransactionID,
		Broadcast:      req.Broadcast,
		HardwareAddr:   req.HardwareAddr,
		YourAddr:       yourAddr,
		ServerAddr:     serverAddr,
		RelayAddr:      req.RelayAddr,
		BootServerName: serverAddr.String(),
	}
	options := make(dhcp4.Options)
	options[dhcp4.OptSubnetMask] = net.IPMask(subnet.Netmask)
	options[dhcp4.OptServerIdentifier] = serverAddr

	buff := make([]byte, 4)
	binary.BigEndian.PutUint32(buff, 4294967295)
	options[dhcp4.OptLeaseTime] = buff

	options[dhcp4.OptDNSServers] = net.IP(subnet.DNSServer).To4()
	gw := net.IP(subnet.Gateway).To4()
	options[dhcp4.OptRouters] = gw
	options[121] = append(options[121], []byte{0, gw[0], gw[1], gw[2], gw[3]}...)

	options[dhcp4.OptTFTPServer] = serverAddr

	userClass, err := req.Options.String(77)
	if err == nil && userClass == "iPXE" {
		options[dhcp4.OptBootFile] = []byte(fmt.Sprintf("http://%s/ipxe/${uuid}", serverAddr.String()))
	} else {
		options[dhcp4.OptBootFile] = []byte("ipxe.efi")
	}

	resp.Options = options

	switch req.Type {
	case dhcp4.MsgDiscover:
		resp.Type = dhcp4.MsgOffer
	case dhcp4.MsgRequest:
		resp.Type = dhcp4.MsgAck
	}

	return resp, nil
}

func getAddr(intf *net.Interface) (*net.IP, error) {
	addrs, err := intf.Addrs()
	if err != nil {
		return nil, fmt.Errorf("failed to get interface address: %w", err)
	}
	var serverAddr *net.IP
	for _, addr := range addrs {
		ip, _, err := net.ParseCIDR(addr.String())
		if err != nil || ip.To4() == nil {
			continue
		}
		serverAddr = &ip
	}
	if serverAddr == nil {
		return nil, errAddressNotFound
	}
	return serverAddr, nil
}

var _ dhcpd.DHCPd = &Netboot{}
