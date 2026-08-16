package handler

import (
	"encoding/json"
	"html"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	"frankenphp-manager/internal/app"
	"frankenphp-manager/internal/server"
)

// Handler struct
type Handler struct {
	App *app.App
}

// NewHandler creates a new Handler
func NewHandler(a *app.App) *Handler {
	return &Handler{App: a}
}

// jsonError sends a JSON error response
func jsonError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// HandleGetServers handles the GET /api/servers endpoint
func (h *Handler) HandleGetServers(w http.ResponseWriter, r *http.Request) {
	servers := h.App.GetServers()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(servers)
}

// HandleCreateServer handles the POST /api/servers endpoint
func (h *Handler) HandleCreateServer(w http.ResponseWriter, r *http.Request) {
	var serverData struct {
		Name      string `json:"name"`
		Host      string `json:"host"`
		Port      string `json:"port"`
		Directory string `json:"directory"`
		Command   string `json:"command"`
	}

	if err := json.NewDecoder(r.Body).Decode(&serverData); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if serverData.Name == "" || serverData.Port == "" || serverData.Directory == "" {
		jsonError(w, "Name, port, and directory are required", http.StatusBadRequest)
		return
	}

	// Sanitize name (prevent XSS)
	serverData.Name = html.EscapeString(serverData.Name)

	// Validate port range
	if _, err := strconv.Atoi(serverData.Port); err != nil {
		jsonError(w, "Port must be a number", http.StatusBadRequest)
		return
	}
	if sanitized, err := server.SanitizePort(serverData.Port); err != nil {
		jsonError(w, "Invalid port: "+err.Error(), http.StatusBadRequest)
		return
	} else {
		serverData.Port = sanitized
	}

	// Validate host
	if serverData.Host != "" {
		if sanitized, err := server.SanitizeHost(serverData.Host); err != nil {
			jsonError(w, "Invalid host: "+err.Error(), http.StatusBadRequest)
			return
		} else {
			serverData.Host = sanitized
		}
	}

	// Validate directory (prevent path traversal)
	if sanitized, err := server.SanitizeDirectory(serverData.Directory); err != nil {
		jsonError(w, "Invalid directory: "+err.Error(), http.StatusBadRequest)
		return
	} else {
		serverData.Directory = sanitized
	}

	id := h.App.CreateServer(serverData.Name, serverData.Host, serverData.Port, serverData.Directory, serverData.Command)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": id})
}

// HandleUpdateServer handles the PUT /api/servers/{id} endpoint
func (h *Handler) HandleUpdateServer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var serverData struct {
		Name      string `json:"name"`
		Host      string `json:"host"`
		Port      string `json:"port"`
		Directory string `json:"directory"`
		Command   string `json:"command"`
	}

	if err := json.NewDecoder(r.Body).Decode(&serverData); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if serverData.Name == "" || serverData.Port == "" || serverData.Directory == "" {
		jsonError(w, "Name, port, and directory are required", http.StatusBadRequest)
		return
	}

	// Sanitize name (prevent XSS)
	serverData.Name = html.EscapeString(serverData.Name)

	// Validate port
	if _, err := strconv.Atoi(serverData.Port); err != nil {
		jsonError(w, "Port must be a number", http.StatusBadRequest)
		return
	}
	if sanitized, err := server.SanitizePort(serverData.Port); err != nil {
		jsonError(w, "Invalid port: "+err.Error(), http.StatusBadRequest)
		return
	} else {
		serverData.Port = sanitized
	}

	// Validate host
	if serverData.Host != "" {
		if sanitized, err := server.SanitizeHost(serverData.Host); err != nil {
			jsonError(w, "Invalid host: "+err.Error(), http.StatusBadRequest)
			return
		} else {
			serverData.Host = sanitized
		}
	}

	// Validate directory
	if sanitized, err := server.SanitizeDirectory(serverData.Directory); err != nil {
		jsonError(w, "Invalid directory: "+err.Error(), http.StatusBadRequest)
		return
	} else {
		serverData.Directory = sanitized
	}

	success := h.App.UpdateServer(id, serverData.Name, serverData.Host, serverData.Port, serverData.Directory, serverData.Command)
	if !success {
		jsonError(w, "Server not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// HandleDeleteServer handles the DELETE /api/servers/{id} endpoint
func (h *Handler) HandleDeleteServer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	success := h.App.DeleteServer(id)
	if !success {
		jsonError(w, "Server not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// HandleStartServer handles the POST /api/servers/{id}/start endpoint
func (h *Handler) HandleStartServer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	success := h.App.StartServer(id)
	if !success {
		jsonError(w, "Failed to start server or server is already running", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// HandleStopServer handles the POST /api/servers/{id}/stop endpoint
func (h *Handler) HandleStopServer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	success := h.App.StopServer(id)
	if !success {
		jsonError(w, "Failed to stop server or server is already stopped", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// HandleServerStatus handles the GET /api/servers/{id}/status endpoint
func (h *Handler) HandleServerStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	exists, running := h.App.GetServerStatus(id)
	if !exists {
		jsonError(w, "Server not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"running": running})
}

// HandleGetServerSettings handles the GET /api/settings endpoint
func (h *Handler) HandleGetServerSettings(w http.ResponseWriter, r *http.Request) {
	settings := h.App.GetServerSettings()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
}

// HandleUpdateServerSettings handles the PUT /api/settings endpoint
func (h *Handler) HandleUpdateServerSettings(w http.ResponseWriter, r *http.Request) {
	var settingsData struct {
		Host     string `json:"host"`
		HostIPv4 string `json:"host_ipv4"`
		HostIPv6 string `json:"host_ipv6"`
		Port     string `json:"port"`
	}

	if err := json.NewDecoder(r.Body).Decode(&settingsData); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if settingsData.Port != "" {
		if _, err := strconv.Atoi(settingsData.Port); err != nil {
			jsonError(w, "Port must be a number", http.StatusBadRequest)
			return
		}
		if sanitized, err := server.SanitizePort(settingsData.Port); err != nil {
			jsonError(w, "Invalid port: "+err.Error(), http.StatusBadRequest)
			return
		} else {
			settingsData.Port = sanitized
		}
	}

	// Validate each host field
	for _, h := range []struct{ name, value string }{
		{"host", settingsData.Host},
		{"host_ipv4", settingsData.HostIPv4},
		{"host_ipv6", settingsData.HostIPv6},
	} {
		if h.value != "" {
			if _, err := server.SanitizeHost(h.value); err != nil {
				jsonError(w, "Invalid "+h.name+": "+err.Error(), http.StatusBadRequest)
				return
			}
		}
	}

	success := h.App.UpdateServerSettings(settingsData.Host, settingsData.HostIPv4, settingsData.HostIPv6, settingsData.Port)
	if !success {
		jsonError(w, "Failed to update server settings", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Settings updated successfully. Restart the application to apply changes."})
}

// HandleUpdateAuth handles the PUT /api/auth endpoint
func (h *Handler) HandleUpdateAuth(w http.ResponseWriter, r *http.Request) {
	var authData struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&authData); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if authData.Username == "" || authData.Password == "" {
		jsonError(w, "Username and password are required", http.StatusBadRequest)
		return
	}

	if len(authData.Username) > 64 {
		jsonError(w, "Username too long (max 64 characters)", http.StatusBadRequest)
		return
	}

	if len(authData.Password) < 8 {
		jsonError(w, "Password must be at least 8 characters", http.StatusBadRequest)
		return
	}

	if err := h.App.UpdateAuth(authData.Username, authData.Password); err != nil {
		jsonError(w, "Failed to update auth settings", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Auth settings updated successfully."})
}
