package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// ── Constants ────────────────────────────────────────────────────────────────

const (
	NetworkName    = "frankenphp-net"
	MySQLContainer = "frankenphp-mysql"
	RedisContainer = "frankenphp-redis"
	MySQLImage     = "mysql:8.0"
	RedisImage     = "redis:7-alpine"
)

var SupportedPHP = []string{"8.4", "8.3", "8.2", "8.1"}
var SupportedHTTPModes = []string{"http", "selfsigned", "acme"}

// ── Project ──────────────────────────────────────────────────────────────────

type Project struct {
	Name       string   `yaml:"name"        json:"name"`
	Dir        string   `yaml:"directory"   json:"directory"`
	PHPVer     string   `yaml:"php_version"  json:"php_version"`
	Iface      string   `yaml:"iface"       json:"iface"`         // network interface: eth0, ens3, lo, etc.
	BindIPv4   string   `yaml:"bind_ipv4"   json:"bind_ipv4"`     // auto-resolved from iface
	BindIPv6   string   `yaml:"bind_ipv6"   json:"bind_ipv6"`     // auto-resolved from iface
	Port       string   `yaml:"port"        json:"port"`
	DBName     string   `yaml:"db_name"     json:"db_name"`
	UseRedis   bool     `yaml:"use_redis"   json:"use_redis"`
	HTTPMode   string   `yaml:"http_mode"   json:"http_mode"`    // http | selfsigned | acme
	Domain     string   `yaml:"domain"      json:"domain"`       // for acme: example.com
	ACMEEmail  string   `yaml:"acme_email"  json:"acme_email"`   // for acme: admin@example.com
}

// ── Config ───────────────────────────────────────────────────────────────────

type Config struct {
	MySQLRootPass string              `yaml:"mysql_root_pass"`
	MySQLUser     string              `yaml:"mysql_user"`
	MySQLPass     string              `yaml:"mysql_pass"`
	MySQLPort     string              `yaml:"mysql_port"`
	RedisPort     string              `yaml:"redis_port"`
	Projects      map[string]*Project `yaml:"projects"`
	mu            sync.RWMutex        `yaml:"-"`
}

func ConfigDir() string   { h, _ := os.UserHomeDir(); return filepath.Join(h, ".frankenphp") }
func ConfigPath() string  { return filepath.Join(ConfigDir(), "config.yaml") }
func ProjectDir(name string) string { return filepath.Join(ConfigDir(), "projects", name) }
func ComposePath(name string) string { return filepath.Join(ProjectDir(name), "docker-compose.yml") }

func LoadConfig() (*Config, error) {
	data, err := os.ReadFile(ConfigPath())
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, err
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	if c.Projects == nil {
		c.Projects = make(map[string]*Project)
	}
	defaults := map[string]*string{
		"MySQLRootPass": &c.MySQLRootPass, "MySQLUser": &c.MySQLUser,
		"MySQLPass": &c.MySQLPass, "MySQLPort": &c.MySQLPort, "RedisPort": &c.RedisPort,
	}
	defVals := map[string]string{
		"MySQLRootPass": "frankenphp", "MySQLUser": "frankenphp",
		"MySQLPass": "frankenphp", "MySQLPort": "13306", "RedisPort": "16379",
	}
	for k, ptr := range defaults {
		if *ptr == "" {
			*ptr = defVals[k]
		}
	}
	return &c, nil
}

func DefaultConfig() *Config {
	return &Config{
		MySQLRootPass: "frankenphp", MySQLUser: "frankenphp", MySQLPass: "frankenphp",
		MySQLPort: "13306", RedisPort: "16379", Projects: make(map[string]*Project),
	}
}

func (c *Config) Save() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if err := os.MkdirAll(ConfigDir(), 0700); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigPath(), data, 0600)
}

func (c *Config) AddProject(p *Project)    { c.mu.Lock(); c.Projects[p.Name] = p; c.mu.Unlock() }
func (c *Config) RemoveProject(name string) { c.mu.Lock(); delete(c.Projects, name); c.mu.Unlock() }
func (c *Config) GetProject(name string) (*Project, bool) {
	c.mu.RLock(); defer c.mu.RUnlock(); p, ok := c.Projects[name]; return p, ok
}
func (c *Config) ProjectList() []*Project {
	c.mu.RLock(); defer c.mu.RUnlock()
	list := make([]*Project, 0, len(c.Projects))
	for _, p := range c.Projects { list = append(list, p) }
	return list
}
