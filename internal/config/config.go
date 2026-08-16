package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

// Config struct holds all configuration for our application
type Config struct {
	Server           ServerConfig `yaml:"server"`
	Auth             Auth         `yaml:"auth"`
	ServersConfigPath string      `yaml:"servers_config_path"`
}

// ServerConfig struct holds server configuration
type ServerConfig struct {
	Host     string `yaml:"host"`
	HostIPv4 string `yaml:"host_ipv4"`
	HostIPv6 string `yaml:"host_ipv6"`
	Port     string `yaml:"port"`
}

// GetListenAddresses returns the listen addresses based on config.
// If host_ipv4 and/or host_ipv6 are set, they take priority over host.
// Returns one or two addresses for dual-stack binding.
func (s *ServerConfig) GetListenAddresses() []string {
	var addrs []string

	if s.HostIPv4 != "" {
		addrs = append(addrs, s.HostIPv4)
	}
	if s.HostIPv6 != "" {
		addrs = append(addrs, s.HostIPv6)
	}

	// Legacy single host field (fallback)
	if len(addrs) == 0 && s.Host != "" {
		addrs = append(addrs, s.Host)
	}

	// Default: dual-stack
	if len(addrs) == 0 {
		addrs = append(addrs, "[::]")
	}

	return addrs
}

// Auth struct holds authentication configuration
type Auth struct {
	Username     string `yaml:"username"`
	PasswordHash string `yaml:"password_hash"`
}

// NewConfig returns a new decoded Config struct
func NewConfig(configPath string) (*Config, error) {
	config := &Config{}

	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	d := yaml.NewDecoder(file)
	if err := d.Decode(&config); err != nil {
		return nil, err
	}

	return config, nil
}

// ValidateConfigPath just makes sure, that the path provided is a file,
// that can be read
func ValidateConfigPath(path string) error {
	s, err := os.Stat(path)
	if err != nil {
		return err
	}
	if s.IsDir() {
		return fmt.Errorf("'%s' is a directory, not a normal file", path)
	}
	return nil
}
