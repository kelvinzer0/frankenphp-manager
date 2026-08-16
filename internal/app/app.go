package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/caddyserver/certmagic"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/time/rate"
	"gopkg.in/yaml.v2"

	"frankenphp-manager/internal/config"
	"frankenphp-manager/internal/server"
)

// App struct
type App struct {
	ctx                context.Context
	servers            map[string]*server.Server
	nextID             int
	mu                 sync.Mutex
	processes          map[string]*exec.Cmd
	serversConfigPath  string
	configPath         string
	serverHost         string
	serverHostIPv4     string
	serverHostIPv6     string
	serverPort         string
	auth               config.Auth
	certmagicInstances map[string]*certmagic.Config
	rateLimiter        *rate.Limiter
}

// NewApp creates a new App application struct
func NewApp(cfg *config.Config, configPath string) *App {
	return &App{
		servers:            make(map[string]*server.Server),
		nextID:             1,
		processes:          make(map[string]*exec.Cmd),
		serversConfigPath:  cfg.ServersConfigPath,
		configPath:         configPath,
		serverHost:         cfg.Server.Host,
		serverHostIPv4:     cfg.Server.HostIPv4,
		serverHostIPv6:     cfg.Server.HostIPv6,
		serverPort:         cfg.Server.Port,
		auth:               cfg.Auth,
		certmagicInstances: make(map[string]*certmagic.Config),
		rateLimiter:        rate.NewLimiter(rate.Every(1), 5),
	}
}

// RateLimiter returns the app's rate limiter
func (a *App) RateLimiter() *rate.Limiter {
	return a.rateLimiter
}

// Startup is called when the app starts
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	configDir := filepath.Dir(a.serversConfigPath)
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		os.MkdirAll(configDir, 0700)
	}
	a.loadConfig()
}

// Shutdown is called when the app is about to exit
func (a *App) Shutdown(ctx context.Context) {
	for id, s := range a.servers {
		if s.Running {
			a.StopServer(id)
		}
	}
	a.saveConfig()
}

// loadConfig loads the saved configuration from disk
func (a *App) loadConfig() {
	data, err := os.ReadFile(a.serversConfigPath)
	if err != nil {
		return
	}

	var configData struct {
		Servers      map[string]*server.Server `json:"servers"`
		NextID       int                       `json:"nextID"`
		ServerHost   string                    `json:"serverHost"`
		ServerHostv4 string                    `json:"serverHostIPv4"`
		ServerHostv6 string                    `json:"serverHostIPv6"`
		ServerPort   string                    `json:"serverPort"`
	}
	if err := json.Unmarshal(data, &configData); err != nil {
		fmt.Printf("Error loading configuration: %v\n", err)
		return
	}

	a.servers = configData.Servers
	a.nextID = configData.NextID
	if configData.ServerHost != "" {
		a.serverHost = configData.ServerHost
	}
	if configData.ServerHostv4 != "" {
		a.serverHostIPv4 = configData.ServerHostv4
	}
	if configData.ServerHostv6 != "" {
		a.serverHostIPv6 = configData.ServerHostv6
	}
	if configData.ServerPort != "" {
		a.serverPort = configData.ServerPort
	}

	for _, s := range a.servers {
		s.Running = false
		if s.Host == "" {
			s.Host = "localhost"
		}
	}
}

// saveConfig saves the current configuration to disk
func (a *App) saveConfig() {
	a.mu.Lock()
	defer a.mu.Unlock()

	configData := struct {
		Servers      map[string]*server.Server `json:"servers"`
		NextID       int                       `json:"nextID"`
		ServerHost   string                    `json:"serverHost"`
		ServerHostv4 string                    `json:"serverHostIPv4"`
		ServerHostv6 string                    `json:"serverHostIPv6"`
		ServerPort   string                    `json:"serverPort"`
	}{
		Servers:      a.servers,
		NextID:       a.nextID,
		ServerHost:   a.serverHost,
		ServerHostv4: a.serverHostIPv4,
		ServerHostv6: a.serverHostIPv6,
		ServerPort:   a.serverPort,
	}

	data, err := json.MarshalIndent(configData, "", "  ")
	if err != nil {
		fmt.Printf("Error serializing configuration: %v\n", err)
		return
	}

	if err := os.WriteFile(a.serversConfigPath, data, 0600); err != nil {
		fmt.Printf("Error saving configuration: %v\n", err)
	}
}

// scheduleSaveConfig saves config asynchronously without holding the caller's lock
func (a *App) scheduleSaveConfig() {
	go a.saveConfig()
}

// GetServers returns all configured servers
func (a *App) GetServers() []*server.Server {
	a.mu.Lock()
	defer a.mu.Unlock()

	servers := make([]*server.Server, 0, len(a.servers))
	for _, s := range a.servers {
		servers = append(servers, s)
	}
	return servers
}

// CreateServer adds a new server configuration
func (a *App) CreateServer(name, host, port, directory, command string) string {
	a.mu.Lock()
	defer a.mu.Unlock()

	id := strconv.Itoa(a.nextID)
	a.nextID++

	if host == "" {
		host = "localhost"
	}

	s := &server.Server{
		ID:        id,
		Name:      name,
		Host:      host,
		Port:      port,
		Directory: directory,
		Command:   command,
		Running:   false,
	}

	a.servers[id] = s
	a.scheduleSaveConfig()
	return id
}

// UpdateServer updates an existing server configuration
func (a *App) UpdateServer(id, name, host, port, directory, command string) bool {
	a.mu.Lock()

	s, exists := a.servers[id]
	if !exists {
		a.mu.Unlock()
		return false
	}

	wasRunning := s.Running
	a.mu.Unlock()

	if wasRunning {
		a.StopServer(id)
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// Re-check after re-acquiring lock
	s, exists = a.servers[id]
	if !exists {
		return false
	}

	if host == "" {
		host = "localhost"
	}

	s.Name = name
	s.Host = host
	s.Port = port
	s.Directory = directory
	s.Command = command
	a.scheduleSaveConfig()
	return true
}

// DeleteServer removes a server configuration
func (a *App) DeleteServer(id string) bool {
	a.mu.Lock()

	s, exists := a.servers[id]
	if !exists {
		a.mu.Unlock()
		return false
	}

	wasRunning := s.Running
	a.mu.Unlock()

	if wasRunning {
		a.StopServer(id)
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	delete(a.servers, id)
	a.scheduleSaveConfig()
	return true
}

// UpdateServerSettings updates the management server host and port
func (a *App) UpdateServerSettings(host, hostIPv4, hostIPv6, port string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	if port == "" {
		port = "8080"
	}

	a.serverHost = host
	a.serverHostIPv4 = hostIPv4
	a.serverHostIPv6 = hostIPv6
	a.serverPort = port
	a.scheduleSaveConfig()
	return true
}

// GetServerSettings returns the current server settings as a map
func (a *App) GetServerSettings() map[string]string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return map[string]string{
		"host":      a.serverHost,
		"host_ipv4": a.serverHostIPv4,
		"host_ipv6": a.serverHostIPv6,
		"port":      a.serverPort,
	}
}

// StartServer starts a PHP server
func (a *App) StartServer(id string) bool {
	a.mu.Lock()
	s, exists := a.servers[id]
	if !exists || s.Running {
		a.mu.Unlock()
		return false
	}
	a.mu.Unlock()

	if s.ACMEEnabled && len(s.ACMEDomains) > 0 {
		cfg := certmagic.NewDefault()
		cfg.Storage = &certmagic.FileStorage{Path: s.ACMEStoragePath}

		go func() {
			err := cfg.ManageSync(a.ctx, s.ACMEDomains)
			if err != nil {
				fmt.Printf("CertMagic error for server %s (%s): %v\n", s.Name, s.ID, err)
			}
		}()
		a.mu.Lock()
		a.certmagicInstances[s.ID] = cfg
		a.mu.Unlock()
	}

	return server.Start(s, a.processes, &a.mu)
}

// StopServer stops a running PHP server
func (a *App) StopServer(id string) bool {
	a.mu.Lock()
	s, exists := a.servers[id]
	if !exists || !s.Running {
		a.mu.Unlock()
		return false
	}
	a.mu.Unlock()

	if s.ACMEEnabled {
		a.mu.Lock()
		if _, ok := a.certmagicInstances[s.ID]; ok {
			fmt.Printf("Stopping cert management for %s\n", s.ID)
			delete(a.certmagicInstances, s.ID)
		}
		a.mu.Unlock()
	}

	return server.Stop(s, a.processes, &a.mu)
}

// GetServerStatus returns the status of a specific server
func (a *App) GetServerStatus(id string) (bool, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	s, exists := a.servers[id]
	if !exists {
		return false, false
	}
	return true, s.Running
}

// UpdateAuth updates the auth settings in the config file
func (a *App) UpdateAuth(username, password string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	a.auth.Username = username
	a.auth.PasswordHash = string(hashedPassword)

	// Read the actual config file (not hardcoded path)
	data, err := os.ReadFile(a.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var configData map[string]interface{}
	if err := yaml.Unmarshal(data, &configData); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	authData, ok := configData["auth"].(map[interface{}]interface{})
	if !ok {
		authData = make(map[interface{}]interface{})
	}
	authData["username"] = username
	authData["password_hash"] = string(hashedPassword)
	configData["auth"] = authData

	newData, err := yaml.Marshal(&configData)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(a.configPath, newData, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
