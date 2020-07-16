package godhcpd

import (
	"context"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"net"

	"go.uber.org/zap"
	"go.universe.tf/netboot/dhcp4"

	"github.com/whywaita/ursa/datastore"
	"github.com/whywaita/ursa/dhcpd"
	"github.com/whywaita/ursa/types"
)

// GoDHCPd is
type GoDHCPd struct {
	ds     datastore.Datastore
	logger *zap.Logger
}

// New is
func New(ds datastore.Datastore, logger *zap.Logger) (dhcpd.DHCPd, error) {
	return &GoDHCPd{
		ds:     ds,
		logger: logger,
	}, nil
}

// Serve serve dhcp daemon.
func (n *GoDHCPd) Serve(ctx context.Context, addr net.IP, iface string) error {
	conn, err := dhcp4.NewConn(fmt.Sprintf("%s:67", addr))
	if err != nil {
		return fmt.Errorf("failed to create nwe connection: %w", err)
	}
	defer conn.Close()

	for {
		req, riface, err := conn.RecvDHCP()
		if err != nil {
			n.logger.Error("failed to receive dhcp request", zap.Error(err))
			continue
		}
		if riface.Name != iface {
			continue
		}
		n.logger.Info("received request", zap.String("req", fmt.Sprintf("%+v", req)))

		subnet, err := n.ds.GetManagementSubnet(ctx)
		if err != nil {
			n.logger.Error("failed to get subnet", zap.Error(err))
			continue
		}
		var lease *dhcpd.Lease
		lease, err = n.ds.GetLeaseFromManagementSubnet(ctx, types.HardwareAddr(req.HardwareAddr))
		if err != nil && errors.Is(err, sql.ErrNoRows) {
			lease, err = n.ds.CreateLeaseFromManagementSubnet(ctx, types.HardwareAddr(req.HardwareAddr))
		}
		if err != nil {
			n.logger.Error("failed to get lease", zap.Error(err))
			continue
		}
		resp, err := makeResponse(addr, *req, *subnet, *lease)
		if err != nil {
			n.logger.Error("failed to make response", zap.Error(err))
			continue
		}
		err = conn.SendDHCP(resp, riface)
		if err != nil {
			n.logger.Error("failed to send dhcp response", zap.Error(err))
			continue
		}
		n.logger.Info("send DCHP response", zap.String("resp", fmt.Sprintf("%+v", resp)))
	}
}

func makeResponse(addr net.IP, req dhcp4.Packet, subnet dhcpd.Subnet, lease dhcpd.Lease) (*dhcp4.Packet, error) {
	serverAddr := addr.To4()
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
	options[dhcp4.OptSubnetMask] = subnet.Network.Mask
	options[dhcp4.OptServerIdentifier] = serverAddr

	options[dhcp4.OptTFTPServer] = serverAddr
	userClass, err := req.Options.String(77)
	if err == nil && userClass == "iPXE" {
		options[dhcp4.OptBootFile] = []byte(fmt.Sprintf("http://%s/ipxe?uuid=${uuid}&mac=${mac:hexhyp}", serverAddr.String()))
	} else {
		options[dhcp4.OptBootFile] = []byte("ipxe.efi")
	}

	buff := make([]byte, 4)
	binary.BigEndian.PutUint32(buff, 4294967295)
	options[dhcp4.OptLeaseTime] = buff

	if subnet.DNSServer != nil {
		options[dhcp4.OptDNSServers] = net.IP(*subnet.DNSServer).To4()
	}

	if subnet.Gateway != nil {
		gw := net.IP(*subnet.Gateway).To4()
		options[dhcp4.OptRouters] = gw
		options[121] = append(options[121], []byte{0, gw[0], gw[1], gw[2], gw[3]}...)
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

var _ dhcpd.DHCPd = &GoDHCPd{}
