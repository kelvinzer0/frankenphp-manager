package server

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
)

// Server represents a PHP server configuration
type Server struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Host            string   `json:"host"`
	Port            string   `json:"port"`
	Directory       string   `json:"directory"`
	Command         string   `json:"command"`
	Running         bool     `json:"running"`
	ACMEEnabled     bool     `json:"acme_enabled"`
	ACMECertEmail   string   `json:"acme_cert_email"`
	ACMEDomains     []string `json:"acme_domains"`
	ACMEStoragePath string   `json:"acme_storage_path"`
}

// validHostRegex allows alphanumeric, dots, hyphens, colons (IPv6), brackets
var validHostRegex = regexp.MustCompile(`^[a-zA-Z0-9\.\-\[\]:]+$`)

// validPortRegex allows only digits
var validPortRegex = regexp.MustCompile(`^[0-9]+$`)

// SanitizeHost validates and sanitizes a host value
func SanitizeHost(host string) (string, error) {
	if host == "" {
		return "localhost", nil
	}
	// Check for command injection characters
	if !validHostRegex.MatchString(host) {
		return "", fmt.Errorf("invalid host format: contains disallowed characters")
	}
	// Must be valid IP or hostname
	if net.ParseIP(host) == nil && host != "localhost" && host != "0.0.0.0" && host != "::" && host != "[::]" {
		// Validate as hostname
		if len(host) > 253 {
			return "", fmt.Errorf("hostname too long")
		}
		for _, part := range strings.Split(host, ".") {
			if len(part) == 0 || len(part) > 63 {
				return "", fmt.Errorf("invalid hostname part")
			}
		}
	}
	return host, nil
}

// SanitizePort validates a port number
func SanitizePort(port string) (string, error) {
	if !validPortRegex.MatchString(port) {
		return "", fmt.Errorf("port must contain only digits")
	}
	portNum := 0
	for _, c := range port {
		portNum = portNum*10 + int(c-'0')
		if portNum > 65535 {
			return "", fmt.Errorf("port must be between 1 and 65535")
		}
	}
	if portNum < 1 || portNum > 65535 {
		return "", fmt.Errorf("port must be between 1 and 65535")
	}
	return port, nil
}

// SanitizeDirectory validates and cleans a directory path
func SanitizeDirectory(dir string) (string, error) {
	if dir == "" {
		return "", fmt.Errorf("directory cannot be empty")
	}
	// Clean the path to resolve .. and other tricks
	cleaned := filepath.Clean(dir)
	// Must be absolute path
	if !filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("directory must be an absolute path")
	}
	// Block access to sensitive system directories
	blocked := []string{"/etc", "/root", "/proc", "/sys", "/dev", "/boot", "/var/log", "/usr"}
	for _, b := range blocked {
		if cleaned == b || strings.HasPrefix(cleaned, b+"/") {
			return "", fmt.Errorf("access to system directory %s is not allowed", b)
		}
	}
	return cleaned, nil
}

// Start starts a PHP server
func Start(s *Server, processes map[string]*exec.Cmd, mu *sync.Mutex) bool {
	bindHost := FormatHostForBinding(s.Host)
	listenAddr := bindHost + ":" + s.Port
	if s.ACMEEnabled {
		listenAddr = "https://" + listenAddr
	}

	var args []string
	var binPath string

	if s.Command != "" {
		// Custom command: parse into binary + args safely (no shell injection)
		parts := splitCommand(s.Command)
		if len(parts) == 0 {
			fmt.Printf("Error: empty custom command for server %s\n", s.ID)
			return false
		}
		binPath = parts[0]
		args = parts[1:]

		// Replace placeholders in args only (not in binary path)
		for i, arg := range args {
			args[i] = strings.ReplaceAll(arg, "{host}", s.Host)
			args[i] = strings.ReplaceAll(args[i], "{port}", s.Port)
			args[i] = strings.ReplaceAll(args[i], "{directory}", s.Directory)
			args[i] = strings.ReplaceAll(args[i], "{listen_addr}", listenAddr)
		}
	} else {
		binPath = "frankenphp"
		args = []string{"php-server", "--listen", listenAddr, "-r", s.Directory}
	}

	// Resolve binary path
	resolvedBin, err := exec.LookPath(binPath)
	if err != nil {
		fmt.Printf("Error: binary %q not found in PATH: %v\n", binPath, err)
		return false
	}

	cmd := exec.Command(resolvedBin, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Dir = s.Directory

	// Set clean environment (don't inherit mutable parent env blindly)
	cmd.Env = append(os.Environ(), "PATH=/usr/local/bin:/usr/bin:/bin")

	err = cmd.Start()
	if err != nil {
		fmt.Printf("Error starting server: %v\n", err)
		return false
	}

	mu.Lock()
	processes[s.ID] = cmd
	s.Running = true
	mu.Unlock()

	go func() {
		cmd.Wait()
		mu.Lock()
		delete(processes, s.ID)
		s.Running = false
		mu.Unlock()
	}()

	return true
}

// Stop stops a running PHP server
func Stop(s *Server, processes map[string]*exec.Cmd, mu *sync.Mutex) bool {
	mu.Lock()
	cmd, exists := processes[s.ID]
	if !exists {
		s.Running = false
		mu.Unlock()
		return true
	}
	mu.Unlock()

	// Kill entire process group
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM); err != nil {
		// If SIGTERM fails, force kill
		if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
			fmt.Printf("Error stopping server: %v\n", err)
			return false
		}
	}

	mu.Lock()
	delete(processes, s.ID)
	s.Running = false
	mu.Unlock()

	return true
}

// splitCommand splits a command string into binary and arguments.
// Respects quoted strings. Does NOT use shell interpretation.
func splitCommand(cmd string) []string {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return nil
	}

	var parts []string
	var current strings.Builder
	inQuote := false
	quoteChar := byte(0)

	for i := 0; i < len(cmd); i++ {
		ch := cmd[i]

		if inQuote {
			if ch == quoteChar {
				inQuote = false
			} else {
				current.WriteByte(ch)
			}
		} else {
			if ch == '"' || ch == '\'' {
				inQuote = true
				quoteChar = ch
			} else if ch == ' ' || ch == '\t' {
				if current.Len() > 0 {
					parts = append(parts, current.String())
					current.Reset()
				}
			} else {
				current.WriteByte(ch)
			}
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// FormatHostForBinding wraps IPv6 addresses in brackets for use in listen addresses.
// Examples:
//
//	"::1"       → "[::1]"
//	"::"        → "[::]"
//	"[::]"      → "[::]"  (already wrapped)
//	"0.0.0.0"  → "0.0.0.0"
//	"localhost" → "localhost"
func FormatHostForBinding(host string) string {
	// Already wrapped
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		return host
	}
	// IPv6 address needs brackets
	if strings.Contains(host, ":") {
		if ip := net.ParseIP(host); ip != nil {
			return "[" + host + "]"
		}
	}
	return host
}
