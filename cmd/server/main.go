package main

import (
	"bufio"
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gorilla/mux"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v2"

	"frankenphp-manager/internal/app"
	"frankenphp-manager/internal/config"
	"frankenphp-manager/internal/handler"
	"frankenphp-manager/internal/middleware"
	"frankenphp-manager/internal/server"
)

//go:embed web/static
var staticFS embed.FS

func main() {
	configDir := getConfigDir()
	configPath := filepath.Join(configDir, "config.yaml")

	// Ensure the config directory exists
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		if err := os.MkdirAll(configDir, 0700); err != nil {
			log.Fatalf("Failed to create config directory: %v", err)
		}
	}

	// Check if config file exists, if not, run setup
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Println("No configuration file found. Starting first-time setup...")
		if err := setupConfig(configPath); err != nil {
			log.Fatalf("Failed to complete setup: %v", err)
		}
		fmt.Println("Setup complete. Starting frankenphp-manager...")
	}

	// Load configuration
	cfg, err := loadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize the App
	application := app.NewApp(cfg, configPath)
	application.Startup(context.Background())
	defer application.Shutdown(context.Background())

	// Initialize the handlers
	h := handler.NewHandler(application)

	// Create router
	r := mux.NewRouter()

	// Create middleware stack
	authMiddleware := middleware.Auth(cfg.Auth)
	rateLimitMiddleware := middleware.RateLimit(application.RateLimiter())

	// API endpoints
	api := r.PathPrefix("/api").Subrouter()
	api.Use(middleware.CORSMiddleware)
	api.Use(rateLimitMiddleware)
	api.Use(authMiddleware)
	api.HandleFunc("/servers", h.HandleGetServers).Methods("GET")
	api.HandleFunc("/servers", h.HandleCreateServer).Methods("POST")
	api.HandleFunc("/servers/{id}", h.HandleUpdateServer).Methods("PUT")
	api.HandleFunc("/servers/{id}", h.HandleDeleteServer).Methods("DELETE")
	api.HandleFunc("/servers/{id}/start", h.HandleStartServer).Methods("POST")
	api.HandleFunc("/servers/{id}/stop", h.HandleStopServer).Methods("POST")
	api.HandleFunc("/servers/{id}/status", h.HandleServerStatus).Methods("GET")
	api.HandleFunc("/settings", h.HandleGetServerSettings).Methods("GET")
	api.HandleFunc("/settings", h.HandleUpdateServerSettings).Methods("PUT")
	api.HandleFunc("/auth", h.HandleUpdateAuth).Methods("PUT")

	// Static files
	staticContent, err := fs.Sub(staticFS, "web/static")
	if err != nil {
		log.Fatal(err)
	}
	r.PathPrefix("/").Handler(http.FileServer(http.FS(staticContent)))

	// Start listeners (dual-stack support)
	addresses := cfg.Server.GetListenAddresses()
	fmt.Printf("frankenphp-manager starting...\n")

	// Start one listener per address
	errCh := make(chan error, len(addresses))
	for _, addr := range addresses {
		addr = server.FormatHostForBinding(addr)
		bindAddr := fmt.Sprintf("%s:%s", addr, cfg.Server.Port)

		go func(bindAddr string) {
			listener, err := net.Listen("tcp", bindAddr)
			if err != nil {
				errCh <- fmt.Errorf("failed to bind %s: %w", bindAddr, err)
				return
			}
			fmt.Printf("  ✓ Listening on %s\n", bindAddr)
			errCh <- http.Serve(listener, r)
		}(bindAddr)
	}

	// Wait for first error
	log.Fatal(<-errCh)
}

func getConfigDir() string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "frankenphp-manager")
	case "linux":
		return filepath.Join("/etc", "frankenphp-manager")
	default:
		return filepath.Join(".", "frankenphp-manager")
	}
}

func loadConfig(path string) (*config.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg config.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func setupConfig(path string) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Enter initial username: ")
	username, _ := reader.ReadString('\n')
	username = strings.TrimSpace(username)

	fmt.Print("Enter initial password: ")
	password, _ := reader.ReadString('\n')
	password = strings.TrimSpace(password)

	if username == "" || password == "" {
		return fmt.Errorf("username and password cannot be empty")
	}

	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}

	fmt.Println("")
	fmt.Println("Network binding options:")
	fmt.Println("  [::]      — Dual-stack: IPv4 + IPv6 (recommended)")
	fmt.Println("  0.0.0.0   — IPv4 only")
	fmt.Println("  Specific  — e.g., 127.0.0.1, ::1, 192.168.1.100")
	fmt.Println("")
	fmt.Println("For explicit dual-stack with separate addresses,")
	fmt.Println("edit config.yaml after setup: host_ipv4 + host_ipv6")
	fmt.Println("")

	fmt.Printf("Enter server host (default: [::] for dual-stack): ")
	host, _ := reader.ReadString('\n')
	host = strings.TrimSpace(host)
	if host == "" {
		host = "[::]"
	}
	// Validate host format
	if _, err := server.SanitizeHost(host); err != nil {
		return fmt.Errorf("invalid host: %w", err)
	}

	fmt.Printf("Enter server port (default: 8080): ")
	port, _ := reader.ReadString('\n')
	port = strings.TrimSpace(port)
	if port == "" {
		port = "8080"
	}

	configDir := filepath.Dir(path)
	fmt.Printf("Enter path for servers data (default: %s/servers.json): ", configDir)
	serversConfigPath, _ := reader.ReadString('\n')
	serversConfigPath = strings.TrimSpace(serversConfigPath)
	if serversConfigPath == "" {
		serversConfigPath = filepath.Join(configDir, "servers.json")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	cfg := config.Config{
		Server: config.ServerConfig{
			Host: host,
			Port: port,
		},
		Auth: config.Auth{
			Username:     username,
			PasswordHash: string(hashedPassword),
		},
		ServersConfigPath: serversConfigPath,
	}

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
