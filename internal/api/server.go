// Package api provides the REST API server
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"mysql-ha/internal/log"

	"github.com/gorilla/mux"
)

// Server is the HTTP API server
type Server struct {
	router     *mux.Router
	httpServer *http.Server
	logger     *log.Logger
	apiKey     string
	agent      AgentInterface
}

// AgentInterface defines the interface for agent operations
type AgentInterface interface {
	GetState() interface{}
	IsLeader() bool
	RequestSwitchover(reason string) error // 请求成为 leader
	RequestDemote(reason string) error     // 请求释放 leader 锁
}

// AgentWrapper wraps an agent to implement AgentInterface
type AgentWrapper struct {
	agent interface {
		GetState() interface{}
		IsLeader() bool
	}
}

// Config holds API server configuration
type Config struct {
	Listen string
	Port   int
	APIKey string
}

// NewServer creates a new API server
func NewServer(cfg Config, agent AgentInterface, logger *log.Logger) *Server {
	s := &Server{
		router: mux.NewRouter(),
		logger: logger,
		apiKey: cfg.APIKey,
		agent:  agent,
	}

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Listen, cfg.Port),
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	s.setupRoutes()
	return s
}

// setupRoutes configures API routes
func (s *Server) setupRoutes() {
	// Apply middleware
	s.router.Use(s.loggingMiddleware)
	s.router.Use(s.corsMiddleware)

	// API routes
	api := s.router.PathPrefix("/api/v1").Subrouter()

	// Auth middleware for API routes
	if s.apiKey != "" {
		api.Use(s.authMiddleware)
	}

	api.HandleFunc("/cluster", s.handleGetCluster).Methods("GET")
	api.HandleFunc("/nodes", s.handleGetNodes).Methods("GET")
	api.HandleFunc("/nodes/{id}", s.handleGetNode).Methods("GET")
	api.HandleFunc("/switchover", s.handleSwitchover).Methods("POST")
	api.HandleFunc("/demote", s.handleDemote).Methods("POST")
	api.HandleFunc("/history", s.handleGetHistory).Methods("GET")

	// Health check and state (no auth) - used by webadmin
	s.router.HandleFunc("/health", s.handleHealth).Methods("GET")
	s.router.HandleFunc("/state", s.handleState).Methods("GET")
}

// Start starts the HTTP server
func (s *Server) Start() error {
	if s.logger != nil {
		s.logger.Info("starting API server")
	}
	return s.httpServer.ListenAndServe()
}

// Stop stops the HTTP server
func (s *Server) Stop(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// Middleware

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		if s.logger != nil {
			s.logger.Info(fmt.Sprintf("%s %s %v", r.Method, r.URL.Path, time.Since(start)))
		}
	})
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			apiKey = r.URL.Query().Get("api_key")
		}

		if apiKey != s.apiKey {
			s.writeError(w, http.StatusUnauthorized, "invalid or missing API key")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Handlers

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	// Return detailed node state for monitoring
	state := s.agent.GetState()
	s.writeJSON(w, http.StatusOK, state)
}

func (s *Server) handleGetCluster(w http.ResponseWriter, r *http.Request) {
	// Return cluster status
	state := s.agent.GetState()
	s.writeJSON(w, http.StatusOK, state)
}

func (s *Server) handleGetNodes(w http.ResponseWriter, r *http.Request) {
	// Return all nodes
	state := s.agent.GetState()
	s.writeJSON(w, http.StatusOK, map[string]interface{}{"nodes": []interface{}{state}})
}

func (s *Server) handleGetNode(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	nodeID := vars["id"]

	if nodeID == "" {
		s.writeError(w, http.StatusBadRequest, "node ID is required")
		return
	}

	state := s.agent.GetState()
	s.writeJSON(w, http.StatusOK, state)
}

func (s *Server) handleSwitchover(w http.ResponseWriter, r *http.Request) {
	var req SwitchoverRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := req.Validate(); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// 执行 switchover - 请求当前节点成为 leader
	if err := s.agent.RequestSwitchover(req.Reason); err != nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("switchover failed: %v", err))
		return
	}

	s.writeJSON(w, http.StatusAccepted, map[string]string{
		"status":  "accepted",
		"message": "switchover initiated, this node is now the leader",
	})
}

func (s *Server) handleGetHistory(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]interface{}{"events": []interface{}{}})
}

func (s *Server) handleDemote(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Reason string `json:"reason,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Ignore decode error, reason is optional
	}

	// 执行 demote - 请求当前节点释放 leader 锁
	if err := s.agent.RequestDemote(req.Reason); err != nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("demote failed: %v", err))
		return
	}

	s.writeJSON(w, http.StatusAccepted, map[string]string{
		"status":  "accepted",
		"message": "demote completed, this node released leadership",
	})
}

// Helper methods

func (s *Server) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (s *Server) writeError(w http.ResponseWriter, status int, message string) {
	s.writeJSON(w, status, map[string]string{"error": message})
}

// CheckAuth checks if the request has valid authentication
func CheckAuth(apiKey, providedKey string) bool {
	if apiKey == "" {
		return true // No auth required
	}
	return apiKey == providedKey
}
