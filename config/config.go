package config

import (
	"os"

	yaml "gopkg.in/yaml.v2"

	"github.com/lovi-cloud/ursa/dhcpd"
)

// Config is usra config struct.
type Config struct {
	Subnets []dhcpd.Subnet `yaml:"subnets"`
}

// LoadConfig is
func LoadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	d := yaml.NewDecoder(f)

	var c Config
	err = d.Decode(&c)
	if err != nil {
		return nil, err
	}

	return &c, nil
}
