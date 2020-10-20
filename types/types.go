package types

import (
	"database/sql/driver"
	"fmt"
	"net"
)

// IPNet is net.IPNet with the implementation of the Valuer and Scanner interface.
type IPNet net.IPNet

// Value implements the database/sql/driver Valuer interface.
func (i IPNet) Value() (driver.Value, error) {
	return driver.Value(i.String()), nil
}

// Scan implements the database/sql Scanner interface.
func (i *IPNet) Scan(src interface{}) error {
	var ipNet *IPNet
	var err error
	switch src := src.(type) {
	case string:
		ipNet, err = ParseCIDR(src)
	case []uint8:
		ipNet, err = ParseCIDR(fmt.Sprintf("%s", src))
	default:
		return fmt.Errorf("incompatible type for IPNet: %T", src)
	}
	if err != nil {
		return err
	}
	*i = *ipNet
	return nil
}

func (i *IPNet) String() string {
	ipNet := net.IPNet(*i)
	return ipNet.String()
}

// IPMask is net.IPMask with the implementation of the Valuer and Scanner interface.
type IPMask net.IPMask

// Value implements the database/sql/driver Valuer interface.
func (i IPMask) Value() (driver.Value, error) {
	return driver.Value(i.String()), nil
}

// Scan implements the database/sql Scanner interface.
func (i *IPMask) Scan(src interface{}) error {
	var ipMask *IPMask
	var err error
	switch src := src.(type) {
	case nil:
		ipMask = nil
	case string:
		ipMask, err = ParseIPMask(src)
	case []uint8:
		ipMask, err = ParseIPMask(fmt.Sprintf("%s", src))
	default:
		return fmt.Errorf("incompatible type for IPMask: %T", src)
	}
	if err != nil {
		return err
	}
	*i = *ipMask
	return nil
}

func (i IPMask) String() string {
	return net.IP(i).String()
}

// MarshalYAML is
func (i IPMask) MarshalYAML() (interface{}, error) {
	return net.IP(i).String(), nil
}

// UnmarshalYAML is
func (i *IPMask) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var buff string
	if err := unmarshal(&buff); err != nil {
		return err
	}
	tmp, err := ParseIPMask(buff)
	if err != nil {
		return fmt.Errorf("failed to unmarshal IPMask: imput=\"%s\"", buff)
	}
	*i = *tmp
	return nil
}

// IP is net.IP with the implementation of the Valuer and Scanner interface.
type IP net.IP

// Value implements the database/sql/driver Valuer interface.
func (i IP) Value() (driver.Value, error) {
	return driver.Value(i.String()), nil
}

// Scan implements the database/sql Scanner interface.
func (i *IP) Scan(src interface{}) error {
	var ip *IP
	var err error
	switch src := src.(type) {
	case nil:
		ip = nil
	case string:
		ip, err = ParseIP(src)
	case []uint8:
		ip, err = ParseIP(fmt.Sprintf("%s", src))
	default:
		return fmt.Errorf("incompatible type for IP: %T", src)
	}
	if err != nil {
		return err
	}
	*i = *ip
	return nil
}

func (i IP) String() string {
	return net.IP(i).String()
}

// MarshalYAML is
func (i IP) MarshalYAML() (interface{}, error) {
	return net.IP(i).String(), nil
}

// UnmarshalYAML is
func (i *IP) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var buff string
	if err := unmarshal(&buff); err != nil {
		return err
	}
	tmp, err := ParseIP(buff)
	if err != nil {
		return fmt.Errorf("failed to unmarshal IP: input=\"%s\"", buff)
	}
	*i = *tmp
	return nil
}

// HardwareAddr is net.HardwareAddr with the implementation of the Valuer and Scanner interface.
type HardwareAddr net.HardwareAddr

// Value implements the database/sql/driver Valuer interface.
func (h HardwareAddr) Value() (driver.Value, error) {
	return driver.Value(h.String()), nil
}

// Scan implements the database/sql Scanner interface.
func (h *HardwareAddr) Scan(src interface{}) error {
	var mac *HardwareAddr
	var err error
	switch src := src.(type) {
	case string:
		mac, err = ParseMAC(src)
	case []uint8:
		mac, err = ParseMAC(fmt.Sprintf("%s", src))
	default:
		return fmt.Errorf("incompatible type for HardwareAddr: %T", src)
	}
	if err != nil {
		return err
	}
	*h = *mac
	return nil
}

func (h HardwareAddr) String() string {
	return net.HardwareAddr(h).String()
}

// ParseCIDR is
func ParseCIDR(s string) (*IPNet, error) {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		return nil, err
	}
	ipNet := IPNet(*n)
	return &ipNet, nil
}

// ParseIPMask is
func ParseIPMask(s string) (*IPMask, error) {
	m := net.IPMask(net.ParseIP(s).To4())
	if m == nil {
		return nil, fmt.Errorf("failed to parse IPMask: input=\"%s\"", s)
	}
	mask := IPMask(m)
	return &mask, nil
}

// ParseIP is
func ParseIP(s string) (*IP, error) {
	i := net.ParseIP(s)
	if i == nil {
		return nil, fmt.Errorf("failed to parse IP: input=\"%s\"", s)
	}
	ip := IP(i)
	return &ip, nil
}

// ParseMAC is
func ParseMAC(s string) (*HardwareAddr, error) {
	m, err := net.ParseMAC(s)
	if err != nil {
		return nil, err
	}
	mac := HardwareAddr(m)
	return &mac, nil
}
