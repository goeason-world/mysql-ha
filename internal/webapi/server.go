// Package webapi provides the web management API
package webapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"mysql-ha/internal/installer"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// Server is the web management API server
type Server struct {
	router       *mux.Router
	server       *http.Server
	installer    *installer.Installer
	clusters     map[string]*installer.ClusterConfig
	mu           sync.RWMutex
	dataDir      string
	clustersFile string
}

// NewServer creates a new web API server
func NewServer(addr string) *Server {
	dataDir := "./data"
	os.MkdirAll(dataDir, 0755)

	s := &Server{
		router:       mux.NewRouter(),
		installer:    installer.NewInstaller(),
		clusters:     make(map[string]*installer.ClusterConfig),
		dataDir:      dataDir,
		clustersFile: dataDir + "/clusters.json",
	}

	s.server = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	s.setupRoutes()

	// 加载已有的集群配置
	if err := s.loadClusters(); err != nil {
		log.Printf("[WARN] Failed to load clusters: %v", err)
	}

	return s
}

func (s *Server) setupRoutes() {
	// CORS middleware
	s.router.Use(corsMiddleware)

	// API routes
	api := s.router.PathPrefix("/api/v1").Subrouter()

	// Software versions
	api.HandleFunc("/versions", s.handleGetVersions).Methods("GET", "OPTIONS")

	// Host management
	api.HandleFunc("/hosts/test", s.handleTestHost).Methods("POST", "OPTIONS")

	// Cluster management
	api.HandleFunc("/clusters", s.handleListClusters).Methods("GET", "OPTIONS")
	api.HandleFunc("/clusters", s.handleCreateCluster).Methods("POST", "OPTIONS")
	api.HandleFunc("/clusters/{id}", s.handleGetCluster).Methods("GET", "OPTIONS")
	api.HandleFunc("/clusters/{id}", s.handleDeleteCluster).Methods("DELETE", "OPTIONS")

	// Installation
	api.HandleFunc("/clusters/{id}/install", s.handleInstallCluster).Methods("POST", "OPTIONS")
	api.HandleFunc("/clusters/{id}/install/status", s.handleInstallStatus).Methods("GET", "OPTIONS")

	// Cluster operations
	api.HandleFunc("/clusters/{id}/status", s.handleClusterStatus).Methods("GET", "OPTIONS")
	api.HandleFunc("/clusters/{id}/switchover", s.handleSwitchover).Methods("POST", "OPTIONS")
	api.HandleFunc("/clusters/{id}/init-mysql-accounts", s.handleInitMySQLAccounts).Methods("POST", "OPTIONS")
	api.HandleFunc("/clusters/{id}/upgrade-agents", s.handleUpgradeAgents).Methods("POST", "OPTIONS")
	api.HandleFunc("/clusters/{id}/logs", s.handleGetLogs).Methods("GET", "OPTIONS")
	api.HandleFunc("/clusters/{id}/repair-replication", s.handleRepairReplication).Methods("POST", "OPTIONS")
	api.HandleFunc("/clusters/{id}/nodes/{nodeId}/repair", s.handleRepairNode).Methods("POST", "OPTIONS")

	// Static files for frontend (Vue SPA)
	s.router.PathPrefix("/assets/").Handler(http.StripPrefix("/assets/", http.FileServer(http.Dir("./web/dist/assets"))))

	// SPA fallback - serve index.html for all non-API routes
	s.router.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./web/dist/index.html")
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Start starts the server
func (s *Server) Start() error {
	return s.server.ListenAndServe()
}

// Stop stops the server
func (s *Server) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// Handlers

func (s *Server) handleGetVersions(w http.ResponseWriter, r *http.Request) {
	versions := map[string]interface{}{
		"etcd":      installer.EtcdVersions,
		"mysql_5_7": installer.MySQL57Versions,
		"mysql_8_0": installer.MySQL80Versions,
	}
	writeJSON(w, http.StatusOK, versions)
}

func (s *Server) handleTestHost(w http.ResponseWriter, r *http.Request) {
	var host installer.Host
	if err := json.NewDecoder(r.Body).Decode(&host); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Test connection
	if err := installer.TestConnection(host.IP, host.Port); err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("Connection failed: %v", err),
		})
		return
	}

	// Test SSH
	client, err := installer.NewSSHClient(&host)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("SSH connection failed: %v", err),
		})
		return
	}
	defer client.Close()

	// Check sudo
	if err := client.CheckSudo(); err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"message": "User does not have sudo privileges",
		})
		return
	}

	// Get system info
	info, _ := client.GetSystemInfo()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":     true,
		"message":     "Connection successful",
		"system_info": info,
	})
}

func (s *Server) handleListClusters(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	clusters := make([]*installer.ClusterConfig, 0, len(s.clusters))
	for _, c := range s.clusters {
		clusters = append(clusters, c)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"clusters": clusters,
	})
}

func (s *Server) handleCreateCluster(w http.ResponseWriter, r *http.Request) {
	var config installer.ClusterConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate
	if config.Name == "" {
		writeError(w, http.StatusBadRequest, "cluster name is required")
		return
	}
	if len(config.Hosts) == 0 {
		writeError(w, http.StatusBadRequest, "at least one host is required")
		return
	}

	// Generate ID
	config.ID = uuid.New().String()

	// Set defaults
	if config.InstallPath == "" {
		config.InstallPath = "/opt/mysql-ha"
	}
	if config.DataPath == "" {
		config.DataPath = "/var/lib/mysql"
	}
	if config.Settings == nil {
		config.Settings = &installer.ClusterSettings{
			MySQLPort:       3306,
			EtcdClientPort:  2379,
			EtcdPeerPort:    2380,
			ReplicationUser: "replicator",
			ReplicationPass: "repl_password",
			RootPassword:    "root_password",
			HAAgentPort:     8080,
		}
	}

	// Assign host IDs
	for i := range config.Hosts {
		if config.Hosts[i].ID == "" {
			config.Hosts[i].ID = uuid.New().String()
		}
		if config.Hosts[i].Port == 0 {
			config.Hosts[i].Port = 22
		}
		config.Hosts[i].Status = "pending"
	}

	s.mu.Lock()
	s.clusters[config.ID] = &config
	s.mu.Unlock()

	// 保存到磁盘
	if err := s.saveClusters(); err != nil {
		log.Printf("[ERROR] Failed to save clusters: %v", err)
	}

	writeJSON(w, http.StatusCreated, config)
}

func (s *Server) handleGetCluster(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	s.mu.RLock()
	cluster, ok := s.clusters[id]
	s.mu.RUnlock()

	if !ok {
		writeError(w, http.StatusNotFound, "cluster not found")
		return
	}

	writeJSON(w, http.StatusOK, cluster)
}

func (s *Server) handleDeleteCluster(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	s.mu.Lock()
	delete(s.clusters, id)
	s.mu.Unlock()

	// 保存到磁盘
	if err := s.saveClusters(); err != nil {
		log.Printf("[ERROR] Failed to save clusters: %v", err)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleInstallCluster(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	s.mu.RLock()
	cluster, ok := s.clusters[id]
	s.mu.RUnlock()

	if !ok {
		writeError(w, http.StatusNotFound, "cluster not found")
		return
	}

	// Start installation in background
	go s.runInstallation(cluster)

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":  "started",
		"message": "Installation started",
	})
}

func (s *Server) handleInstallStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	s.mu.RLock()
	cluster, ok := s.clusters[id]
	s.mu.RUnlock()

	if !ok {
		writeError(w, http.StatusNotFound, "cluster not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"cluster_id": id,
		"hosts":      cluster.Hosts,
	})
}

// installResult holds the result of a concurrent installation
type installResult struct {
	hostID string
	err    error
}

func (s *Server) runInstallation(cluster *installer.ClusterConfig) {
	log.Printf("[INFO] ========================================")
	log.Printf("[INFO] Starting installation for cluster: %s", cluster.Name)
	log.Printf("[INFO] ========================================")

	// 收集节点信息
	var etcdNodes []installer.Host
	var mysqlNodes []*installer.Host
	for i := range cluster.Hosts {
		if cluster.Hosts[i].IsEtcdNode() {
			etcdNodes = append(etcdNodes, cluster.Hosts[i])
		}
		if cluster.Hosts[i].IsMySQLNode() {
			mysqlNodes = append(mysqlNodes, &cluster.Hosts[i])
		}
	}

	// ========================================
	// 阶段1: etcd 集群部署 (并发)
	// ========================================
	log.Printf("[INFO] ========== 阶段1: etcd 集群部署 (%d 节点并发) ==========", len(etcdNodes))
	s.updateClusterPhase(cluster.ID, "phase1_etcd")

	if !s.installEtcdConcurrent(cluster, etcdNodes) {
		log.Printf("[ERROR] etcd 集群部署失败，停止安装")
		s.updateClusterPhase(cluster.ID, "failed_phase1")
		return
	}

	// 验证 etcd 集群健康
	log.Printf("[INFO] 验证 etcd 集群健康状态...")
	if err := s.installer.VerifyEtcdCluster(etcdNodes); err != nil {
		log.Printf("[ERROR] etcd 集群验证失败: %v", err)
		s.updateClusterPhase(cluster.ID, "failed_phase1_verify")
		return
	}
	log.Printf("[INFO] etcd 集群健康 ✓")

	// ========================================
	// 阶段2: MySQL 部署 (并发)
	// ========================================
	log.Printf("[INFO] ========== 阶段2: MySQL 部署 (%d 节点并发) ==========", len(mysqlNodes))
	s.updateClusterPhase(cluster.ID, "phase2_mysql")

	if !s.installMySQLConcurrent(cluster, mysqlNodes) {
		log.Printf("[ERROR] MySQL 部署失败，停止安装")
		s.updateClusterPhase(cluster.ID, "failed_phase2")
		return
	}

	// 验证所有 MySQL 节点可连接
	log.Printf("[INFO] 验证所有 MySQL 节点...")
	for _, host := range mysqlNodes {
		if err := s.installer.VerifyMySQLConnection(host, cluster.InstallPath, cluster.Settings); err != nil {
			log.Printf("[ERROR] [%s] MySQL 连接验证失败: %v", host.Name, err)
			s.updateHostStatus(cluster.ID, host.ID, fmt.Sprintf("failed_mysql_verify: %v", err))
			s.updateClusterPhase(cluster.ID, "failed_phase2_verify")
			return
		}
		log.Printf("[INFO] [%s] MySQL 连接正常 ✓", host.Name)
	}

	// ========================================
	// 阶段3: MySQL 主从配置
	// ========================================
	log.Printf("[INFO] ========== 阶段3: MySQL 主从配置 ==========")
	s.updateClusterPhase(cluster.ID, "phase3_replication")

	if len(mysqlNodes) > 1 {
		log.Printf("[INFO] 配置 MySQL 主从复制...")
		if err := s.installer.ConfigureReplication(cluster); err != nil {
			log.Printf("[ERROR] MySQL 主从配置失败: %v", err)
			s.updateClusterPhase(cluster.ID, "failed_phase3")
			return
		}
		log.Printf("[INFO] MySQL 主从配置完成 ✓")

		// 验证主从同步状态
		log.Printf("[INFO] 验证主从同步状态...")
		if err := s.installer.VerifyReplication(cluster); err != nil {
			log.Printf("[ERROR] 主从同步验证失败: %v", err)
			s.updateClusterPhase(cluster.ID, "failed_phase3_verify")
			return
		}
		log.Printf("[INFO] 主从同步正常 ✓")
	} else {
		log.Printf("[INFO] 单节点模式，跳过主从配置")
	}

	// ========================================
	// 阶段4: HA Agent 部署 (并发)
	// ========================================
	log.Printf("[INFO] ========== 阶段4: HA Agent 部署 ==========")
	s.updateClusterPhase(cluster.ID, "phase4_agent")

	agentBinary := "./mypatroni"
	if _, err := os.Stat(agentBinary); err != nil {
		log.Printf("[WARN] HA Agent 二进制文件不存在: %s，跳过 Agent 部署", agentBinary)
		s.updateClusterPhase(cluster.ID, "completed_no_agent")
		s.markAllHostsCompleted(cluster)
		return
	}

	if !s.installAgentConcurrent(cluster, mysqlNodes, agentBinary) {
		log.Printf("[ERROR] HA Agent 部署失败")
		s.updateClusterPhase(cluster.ID, "failed_phase4")
		return
	}

	// ========================================
	// 阶段5: 集群验证
	// ========================================
	log.Printf("[INFO] ========== 阶段5: 集群验证 ==========")
	s.updateClusterPhase(cluster.ID, "phase5_verify")

	// 标记所有节点完成
	s.markAllHostsCompleted(cluster)

	log.Printf("[INFO] ========================================")
	log.Printf("[INFO] 集群 %s 安装完成！", cluster.Name)
	log.Printf("[INFO] ========================================")
	s.updateClusterPhase(cluster.ID, "completed")
}

func (s *Server) updateClusterPhase(clusterID, phase string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cluster, ok := s.clusters[clusterID]; ok {
		cluster.Phase = phase
		go s.saveClusters()
	}
}

// installEtcdConcurrent installs etcd on all nodes concurrently
func (s *Server) installEtcdConcurrent(cluster *installer.ClusterConfig, etcdNodes []installer.Host) bool {
	var wg sync.WaitGroup
	results := make(chan installResult, len(etcdNodes))

	for i := range cluster.Hosts {
		host := &cluster.Hosts[i]
		if !host.IsEtcdNode() {
			continue
		}

		wg.Add(1)
		go func(h *installer.Host) {
			defer wg.Done()

			log.Printf("[INFO] [%s] 开始安装 etcd...", h.Name)
			s.updateHostStatus(cluster.ID, h.ID, "installing_etcd")

			err := s.installer.InstallEtcd(h, cluster.EtcdVersion, cluster.InstallPath, etcdNodes, func(progress int, msg string) {
				log.Printf("[INFO] [%s] etcd: %d%% - %s", h.Name, progress, msg)
			})

			if err != nil {
				log.Printf("[ERROR] [%s] etcd 安装失败: %v", h.Name, err)
				s.updateHostStatus(cluster.ID, h.ID, fmt.Sprintf("failed_etcd: %v", err))
				results <- installResult{hostID: h.ID, err: err}
				return
			}

			s.updateHostStatus(cluster.ID, h.ID, "etcd_installed")
			log.Printf("[INFO] [%s] etcd 安装完成 ✓", h.Name)
			results <- installResult{hostID: h.ID, err: nil}
		}(host)
	}

	// 等待所有 goroutine 完成
	go func() {
		wg.Wait()
		close(results)
	}()

	// 检查结果
	hasError := false
	for result := range results {
		if result.err != nil {
			hasError = true
		}
	}

	return !hasError
}

// installMySQLConcurrent installs MySQL on all nodes concurrently
func (s *Server) installMySQLConcurrent(cluster *installer.ClusterConfig, mysqlNodes []*installer.Host) bool {
	var wg sync.WaitGroup
	results := make(chan installResult, len(mysqlNodes))

	for _, host := range mysqlNodes {
		wg.Add(1)
		go func(h *installer.Host) {
			defer wg.Done()

			log.Printf("[INFO] [%s] 开始安装 MySQL...", h.Name)
			s.updateHostStatus(cluster.ID, h.ID, "installing_mysql")

			err := s.installer.InstallMySQL(h, cluster.MySQLVersion, cluster.InstallPath, cluster.DataPath, cluster.Settings, func(progress int, msg string) {
				log.Printf("[INFO] [%s] MySQL: %d%% - %s", h.Name, progress, msg)
			})

			if err != nil {
				log.Printf("[ERROR] [%s] MySQL 安装失败: %v", h.Name, err)
				s.updateHostStatus(cluster.ID, h.ID, fmt.Sprintf("failed_mysql: %v", err))
				results <- installResult{hostID: h.ID, err: err}
				return
			}

			s.updateHostStatus(cluster.ID, h.ID, "mysql_installed")
			log.Printf("[INFO] [%s] MySQL 安装完成 ✓", h.Name)
			results <- installResult{hostID: h.ID, err: nil}
		}(host)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	hasError := false
	for result := range results {
		if result.err != nil {
			hasError = true
		}
	}

	return !hasError
}

// installAgentConcurrent installs HA Agent on all nodes concurrently
func (s *Server) installAgentConcurrent(cluster *installer.ClusterConfig, mysqlNodes []*installer.Host, agentBinary string) bool {
	var wg sync.WaitGroup
	results := make(chan installResult, len(mysqlNodes))

	for _, host := range mysqlNodes {
		wg.Add(1)
		go func(h *installer.Host) {
			defer wg.Done()

			log.Printf("[INFO] [%s] 开始部署 HA Agent...", h.Name)
			s.updateHostStatus(cluster.ID, h.ID, "installing_agent")

			// 创建配置和服务文件
			err := s.installer.InstallHAAgent(h, cluster.InstallPath, cluster, func(progress int, msg string) {
				log.Printf("[INFO] [%s] Agent: %d%% - %s", h.Name, progress, msg)
			})
			if err != nil {
				log.Printf("[ERROR] [%s] HA Agent 配置失败: %v", h.Name, err)
				s.updateHostStatus(cluster.ID, h.ID, fmt.Sprintf("failed_agent: %v", err))
				results <- installResult{hostID: h.ID, err: err}
				return
			}

			// 上传并启动
			err = s.installer.UploadAndStartHAAgent(h, cluster.InstallPath, agentBinary, func(progress int, msg string) {
				log.Printf("[INFO] [%s] Agent: %d%% - %s", h.Name, progress, msg)
			})
			if err != nil {
				log.Printf("[ERROR] [%s] HA Agent 启动失败: %v", h.Name, err)
				s.updateHostStatus(cluster.ID, h.ID, fmt.Sprintf("failed_agent_start: %v", err))
				results <- installResult{hostID: h.ID, err: err}
				return
			}

			s.updateHostStatus(cluster.ID, h.ID, "agent_installed")
			log.Printf("[INFO] [%s] HA Agent 部署完成 ✓", h.Name)
			results <- installResult{hostID: h.ID, err: nil}
		}(host)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	hasError := false
	for result := range results {
		if result.err != nil {
			hasError = true
		}
	}

	return !hasError
}

func (s *Server) markAllHostsCompleted(cluster *installer.ClusterConfig) {
	for i := range cluster.Hosts {
		if !s.isHostFailed(cluster.ID, cluster.Hosts[i].ID) {
			s.updateHostStatus(cluster.ID, cluster.Hosts[i].ID, "completed")
		}
	}
}

func (s *Server) isHostFailed(clusterID, hostID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if cluster, ok := s.clusters[clusterID]; ok {
		for _, host := range cluster.Hosts {
			if host.ID == hostID {
				return len(host.Status) > 6 && host.Status[:6] == "failed"
			}
		}
	}
	return false
}

func (s *Server) updateHostStatus(clusterID, hostID, status string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cluster, ok := s.clusters[clusterID]; ok {
		for i := range cluster.Hosts {
			if cluster.Hosts[i].ID == hostID {
				cluster.Hosts[i].Status = status
				break
			}
		}
		// 异步保存到磁盘（避免阻塞安装流程）
		go func() {
			if err := s.saveClusters(); err != nil {
				log.Printf("[ERROR] Failed to save clusters: %v", err)
			}
		}()
	}
}

func (s *Server) handleClusterStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	s.mu.RLock()
	cluster, ok := s.clusters[id]
	s.mu.RUnlock()

	if !ok {
		writeError(w, http.StatusNotFound, "cluster not found")
		return
	}

	// 收集所有节点的状态
	type NodeStatus struct {
		NodeID         string `json:"node_id"`
		Name           string `json:"name"`
		IP             string `json:"ip"`
		Role           string `json:"role"`
		IsHealthy      bool   `json:"is_healthy"`
		MySQLHealthy   bool   `json:"mysql_healthy"`
		AgentHealthy   bool   `json:"agent_healthy"`
		ReplicationLag int    `json:"replication_lag,omitempty"`
		InstallStatus  string `json:"install_status"`
	}

	var nodes []NodeStatus
	var leaderNode *NodeStatus

	// Step 1: 从 etcd 获取真正的 leader 信息
	var etcdLeaderHost string
	var etcdLeaderNodeID string
	var etcdLeaderInfo string

	// 找到一个 etcd 节点来查询
	for _, host := range cluster.Hosts {
		if host.IsEtcdNode() {
			sshClient, err := installer.NewSSHClient(&host)
			if err != nil {
				log.Printf("[WARN] 无法连接 etcd 节点 %s: %v", host.Name, err)
				continue
			}

			// 查询 etcd 中的 leader_info
			// Agent 使用的 prefix 是 /mypatroni/{cluster_name}，key 是 leader_info
			// 完整路径是 /mypatroni/{cluster_name}/leader_info
			key := fmt.Sprintf("/mypatroni/%s/leader_info", cluster.Name)
			etcdCmd := fmt.Sprintf("etcdctl get '%s' --print-value-only 2>/dev/null || /usr/local/bin/etcdctl get '%s' --print-value-only 2>/dev/null", key, key)
			output, err := sshClient.Run(etcdCmd)
			sshClient.Close()

			if err == nil && strings.TrimSpace(output) != "" {
				etcdLeaderInfo = strings.TrimSpace(output)
				log.Printf("[INFO] 从 etcd 获取 leader_info: key=%s, value=%s", key, etcdLeaderInfo)

				// 解析 leader_info JSON: {"node_id":"xxx","host":"10.211.55.32","port":3306,"timestamp":xxx}
				// 简单解析 host 字段
				if hostStart := strings.Index(etcdLeaderInfo, `"host":"`); hostStart > 0 {
					hostStart += 8
					if hostEnd := strings.Index(etcdLeaderInfo[hostStart:], `"`); hostEnd > 0 {
						etcdLeaderHost = etcdLeaderInfo[hostStart : hostStart+hostEnd]
					}
				}
				// 解析 node_id 字段
				if nodeIDStart := strings.Index(etcdLeaderInfo, `"node_id":"`); nodeIDStart > 0 {
					nodeIDStart += 11
					if nodeIDEnd := strings.Index(etcdLeaderInfo[nodeIDStart:], `"`); nodeIDEnd > 0 {
						etcdLeaderNodeID = etcdLeaderInfo[nodeIDStart : nodeIDStart+nodeIDEnd]
					}
				}
				log.Printf("[INFO] 解析 etcd leader: node_id=%s, host=%s", etcdLeaderNodeID, etcdLeaderHost)
				break
			} else {
				log.Printf("[WARN] etcd 中没有 leader_info 或查询失败: key=%s, err=%v, output=%s", key, err, output)
			}
		}
	}

	// Step 2: 收集各节点状态
	for _, host := range cluster.Hosts {
		if !host.IsMySQLNode() {
			continue
		}

		node := NodeStatus{
			NodeID:        host.ID,
			Name:          host.Name,
			IP:            host.IP,
			InstallStatus: host.Status,
		}

		// 尝试连接HA Agent获取详细状态
		agentURL := fmt.Sprintf("http://%s:%d/state", host.IP, cluster.Settings.HAAgentPort)
		client := &http.Client{Timeout: 2 * time.Second}
		resp, err := client.Get(agentURL)
		if err == nil && resp.StatusCode == http.StatusOK {
			// Agent 端口可达，Agent 是健康的
			node.AgentHealthy = true
			// 解析 agent 返回的状态
			var agentState struct {
				NodeID       string `json:"node_id"`
				Role         string `json:"role"`
				IsHealthy    bool   `json:"is_healthy"` // 这是 MySQL 的健康状态
				GTIDExecuted string `json:"gtid_executed"`
			}
			if json.NewDecoder(resp.Body).Decode(&agentState) == nil {
				// IsHealthy 是 Agent 报告的 MySQL 健康状态
				node.MySQLHealthy = agentState.IsHealthy
				// 使用 agent 报告的角色
				if agentState.Role == "leader" {
					node.Role = "leader"
				} else if agentState.Role == "replica" {
					node.Role = "replica"
				}
				log.Printf("[DEBUG] Agent %s 报告: role=%s, mysql_healthy=%v", host.Name, agentState.Role, agentState.IsHealthy)
			}
			resp.Body.Close()
		} else {
			// Agent 不可用（端口不通或返回错误）
			node.AgentHealthy = false
			log.Printf("[DEBUG] Agent %s 不可达: %v", host.Name, err)
			// 通过 SSH 检查 MySQL 进程
			sshClient, err := installer.NewSSHClient(&host)
			if err == nil {
				output, err := sshClient.Run("systemctl is-active mysql 2>/dev/null || systemctl is-active mysqld 2>/dev/null || echo 'inactive'")
				sshClient.Close()
				if err == nil && strings.TrimSpace(output) == "active" {
					node.MySQLHealthy = true
				}
			}
		}

		// 综合健康状态：Agent 健康即可显示为在线
		node.IsHealthy = node.AgentHealthy

		// 角色判断优先级：agent 报告 > etcd leader_info > 配置文件
		// 如果 agent 已经报告了角色，保持不变（agent 持有锁才会报告 leader）
		if node.Role == "" {
			// agent 没有报告角色，检查 etcd
			if etcdLeaderHost != "" && host.IP == etcdLeaderHost {
				node.Role = "leader"
				log.Printf("[INFO] 根据 etcd leader_info，%s (%s) 是 leader（但 agent 未确认）", host.Name, host.IP)
			} else if etcdLeaderHost != "" {
				node.Role = "replica"
			} else {
				// etcd 也没有信息，使用配置
				if host.HasRole(installer.RoleMaster) {
					node.Role = "leader"
					log.Printf("[WARN] etcd 无 leader_info，根据配置 %s 是 master 角色", host.Name)
				} else {
					node.Role = "replica"
				}
			}
		}

		nodes = append(nodes, node)
	}

	// Step 3: 确定最终的 leader 节点
	// 优先使用 agent 报告的角色（agent 持有 etcd 锁才会报告自己是 leader）
	for i := range nodes {
		if nodes[i].Role == "leader" && nodes[i].AgentHealthy {
			leaderNode = &nodes[i]
			log.Printf("[INFO] 最终 leader (来自 agent 报告): %s (%s)", nodes[i].Name, nodes[i].IP)
			break
		}
	}

	// 如果没有 agent 报告为 leader，使用 etcd 中的 leader_info（可能是旧数据）
	if leaderNode == nil && etcdLeaderHost != "" {
		for i := range nodes {
			if nodes[i].IP == etcdLeaderHost {
				leaderNode = &nodes[i]
				log.Printf("[WARN] 最终 leader (来自 etcd，可能是旧数据): %s (%s)", nodes[i].Name, nodes[i].IP)
				break
			}
		}
	}

	// 最后备选：选择第一个 leader 角色的节点
	if leaderNode == nil {
		for i := range nodes {
			if nodes[i].Role == "leader" {
				leaderNode = &nodes[i]
				log.Printf("[WARN] 最终 leader (后备选择): %s (%s)", nodes[i].Name, nodes[i].IP)
				break
			}
		}
	}

	response := map[string]interface{}{
		"cluster_id":       id,
		"cluster_name":     cluster.Name,
		"leader":           leaderNode,
		"nodes":            nodes,
		"etcd_leader_info": etcdLeaderInfo, // 返回原始 etcd 信息，方便前端调试
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleInitMySQLAccounts(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	s.mu.RLock()
	cluster, ok := s.clusters[id]
	s.mu.RUnlock()

	if !ok {
		writeError(w, http.StatusNotFound, "cluster not found")
		return
	}

	// 在后台初始化所有 MySQL 节点的账号
	go func() {
		for i := range cluster.Hosts {
			host := &cluster.Hosts[i]
			if !host.IsMySQLNode() {
				continue
			}

			log.Printf("[INFO] Initializing MySQL accounts on host: %s (%s)", host.Name, host.IP)
			err := s.installer.InitMySQLAccountsOnHost(host, cluster.InstallPath, cluster.Settings)
			if err != nil {
				log.Printf("[ERROR] [%s] Failed to initialize MySQL accounts: %v", host.Name, err)
			} else {
				log.Printf("[INFO] [%s] MySQL accounts initialized successfully", host.Name)
			}
		}
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":  "started",
		"message": "MySQL account initialization started",
	})
}

func (s *Server) handleSwitchover(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	s.mu.RLock()
	cluster, ok := s.clusters[id]
	s.mu.RUnlock()

	if !ok {
		writeError(w, http.StatusNotFound, "cluster not found")
		return
	}

	var req struct {
		TargetNodeID string `json:"target_node_id"`
		Reason       string `json:"reason"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.TargetNodeID == "" {
		writeError(w, http.StatusBadRequest, "target_node_id is required")
		return
	}

	// 查找目标节点
	var targetHost *installer.Host
	for i := range cluster.Hosts {
		if cluster.Hosts[i].ID == req.TargetNodeID {
			targetHost = &cluster.Hosts[i]
			break
		}
	}

	if targetHost == nil {
		writeError(w, http.StatusNotFound, "target node not found")
		return
	}

	// 在后台执行切换
	go func() {
		log.Printf("[INFO] Starting switchover to node %s (%s), reason: %s", targetHost.Name, targetHost.IP, req.Reason)

		if err := s.installer.PerformSwitchover(cluster, req.TargetNodeID); err != nil {
			log.Printf("[ERROR] Switchover failed: %v", err)
			return
		}

		// 更新集群配置中的角色
		s.mu.Lock()
		for i := range cluster.Hosts {
			if cluster.Hosts[i].IsMySQLNode() {
				// 移除旧的 master/slave 角色
				newRoles := []string{}
				for _, role := range cluster.Hosts[i].Roles {
					if role != installer.RoleMaster && role != installer.RoleSlave {
						newRoles = append(newRoles, role)
					}
				}
				// 添加新角色
				if cluster.Hosts[i].ID == req.TargetNodeID {
					newRoles = append(newRoles, installer.RoleMaster)
				} else {
					newRoles = append(newRoles, installer.RoleSlave)
				}
				cluster.Hosts[i].Roles = newRoles
			}
		}
		s.mu.Unlock()

		// 保存配置
		if err := s.saveClusters(); err != nil {
			log.Printf("[ERROR] Failed to save cluster config after switchover: %v", err)
		}

		log.Printf("[INFO] Switchover completed successfully, new master: %s", targetHost.Name)
	}()

	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"status":  "accepted",
		"message": "switchover initiated",
	})
}

// Helper functions

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// saveClusters saves clusters to disk
func (s *Server) saveClusters() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 确保 data 目录存在
	if err := os.MkdirAll(s.dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	data, err := json.MarshalIndent(s.clusters, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal clusters: %w", err)
	}

	// 直接写入文件（避免跨文件系统重命名问题）
	if err := os.WriteFile(s.clustersFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write clusters file: %w", err)
	}

	log.Printf("[INFO] Saved %d clusters to disk", len(s.clusters))
	return nil
}

// loadClusters loads clusters from disk
func (s *Server) loadClusters() error {
	data, err := os.ReadFile(s.clustersFile)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("[INFO] No existing clusters file found, starting fresh")
			return nil
		}
		return fmt.Errorf("failed to read clusters file: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := json.Unmarshal(data, &s.clusters); err != nil {
		return fmt.Errorf("failed to unmarshal clusters: %w", err)
	}

	log.Printf("[INFO] Loaded %d clusters from disk", len(s.clusters))
	return nil
}

// handleUpgradeAgents handles agent upgrade requests
func (s *Server) handleUpgradeAgents(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	s.mu.RLock()
	cluster, ok := s.clusters[id]
	s.mu.RUnlock()

	if !ok {
		writeError(w, http.StatusNotFound, "cluster not found")
		return
	}

	var req struct {
		NodeIDs []string `json:"node_ids"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	agentBinary := "./mypatroni"
	if _, err := os.Stat(agentBinary); err != nil {
		writeError(w, http.StatusBadRequest, "mypatroni binary not found")
		return
	}

	// 在后台执行更新
	go func() {
		for _, nodeID := range req.NodeIDs {
			var targetHost *installer.Host
			for i := range cluster.Hosts {
				if cluster.Hosts[i].ID == nodeID {
					targetHost = &cluster.Hosts[i]
					break
				}
			}
			if targetHost == nil {
				continue
			}

			log.Printf("[INFO] Upgrading HA Agent on %s (%s)", targetHost.Name, targetHost.IP)
			err := s.installer.UploadAndStartHAAgent(targetHost, cluster.InstallPath, agentBinary, func(progress int, msg string) {
				log.Printf("[INFO] [%s] Agent upgrade: %d%% - %s", targetHost.Name, progress, msg)
			})
			if err != nil {
				log.Printf("[ERROR] [%s] Agent upgrade failed: %v", targetHost.Name, err)
			} else {
				log.Printf("[INFO] [%s] Agent upgrade completed", targetHost.Name)
			}
		}
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":  "started",
		"message": fmt.Sprintf("Upgrading %d agents", len(req.NodeIDs)),
	})
}

// handleRepairReplication handles one-click replication repair
// 一键修复主从：检测实际的 MySQL 主节点，更新 etcd，重新配置所有从节点
func (s *Server) handleRepairReplication(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	s.mu.RLock()
	cluster, ok := s.clusters[id]
	s.mu.RUnlock()

	if !ok {
		writeError(w, http.StatusNotFound, "cluster not found")
		return
	}

	// 在后台执行修复
	go func() {
		log.Printf("[INFO] ========================================")
		log.Printf("[INFO] 开始一键修复主从复制: %s", cluster.Name)
		log.Printf("[INFO] ========================================")

		// Step 1: 检测所有 MySQL 节点的实际状态
		type MySQLNodeInfo struct {
			Host       *installer.Host
			IsReadOnly bool
			HasRepl    bool   // 是否配置了复制
			MasterHost string // 如果是从节点，复制的主节点 IP
			IORunning  bool
			SQLRunning bool
			Error      string
		}

		var nodeInfos []MySQLNodeInfo
		var actualMaster *installer.Host

		for i := range cluster.Hosts {
			host := &cluster.Hosts[i]
			if !host.IsMySQLNode() {
				continue
			}

			info := MySQLNodeInfo{Host: host}

			client, err := installer.NewSSHClient(host)
			if err != nil {
				info.Error = fmt.Sprintf("SSH连接失败: %v", err)
				nodeInfos = append(nodeInfos, info)
				continue
			}

			mysqlCmd := fmt.Sprintf("%s/mysql/bin/mysql -u root -p'%s' --socket=/tmp/mysql.sock",
				cluster.InstallPath, cluster.Settings.RootPassword)

			// 检查 read_only 状态
			output, err := client.Run(fmt.Sprintf("%s -N -e \"SELECT @@read_only\"", mysqlCmd))
			if err != nil {
				info.Error = fmt.Sprintf("无法查询read_only: %v", err)
				client.Close()
				nodeInfos = append(nodeInfos, info)
				continue
			}
			info.IsReadOnly = strings.TrimSpace(output) == "1"

			// 检查复制状态
			output, err = client.Run(fmt.Sprintf("%s -e \"SHOW SLAVE STATUS\\G\"", mysqlCmd))
			if err == nil && strings.Contains(output, "Master_Host") {
				info.HasRepl = true
				// 解析 Master_Host
				for _, line := range strings.Split(output, "\n") {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "Master_Host:") {
						info.MasterHost = strings.TrimSpace(strings.TrimPrefix(line, "Master_Host:"))
					}
					if strings.HasPrefix(line, "Slave_IO_Running:") {
						info.IORunning = strings.Contains(line, "Yes")
					}
					if strings.HasPrefix(line, "Slave_SQL_Running:") {
						info.SQLRunning = strings.Contains(line, "Yes")
					}
				}
			}

			client.Close()
			nodeInfos = append(nodeInfos, info)

			log.Printf("[INFO] [%s] read_only=%v, has_repl=%v, master_host=%s, io=%v, sql=%v",
				host.Name, info.IsReadOnly, info.HasRepl, info.MasterHost, info.IORunning, info.SQLRunning)

			// 判断实际主节点：不是只读 且 没有配置复制
			// 只选择第一个符合条件的节点作为主节点
			if actualMaster == nil && !info.IsReadOnly && !info.HasRepl {
				actualMaster = host
				log.Printf("[INFO] 检测到实际主节点: %s (%s)", host.Name, host.IP)
			}
		}

		if actualMaster == nil {
			// 如果没有找到明确的主节点，选择第一个不是只读的节点
			for _, info := range nodeInfos {
				if !info.IsReadOnly && info.Error == "" {
					actualMaster = info.Host
					log.Printf("[INFO] 选择非只读节点作为主节点: %s (%s)", info.Host.Name, info.Host.IP)
					break
				}
			}
		}

		if actualMaster == nil {
			// 如果所有节点都是只读，选择配置中角色为 master 的节点
			log.Printf("[INFO] 所有节点都是只读状态，尝试根据配置角色选择主节点...")
			for _, info := range nodeInfos {
				if info.Error == "" && info.Host.IsMasterNode() {
					actualMaster = info.Host
					log.Printf("[INFO] 根据配置角色选择主节点: %s (%s)", info.Host.Name, info.Host.IP)
					break
				}
			}
		}

		if actualMaster == nil {
			// 最后的备选：选择第一个没有错误的节点
			log.Printf("[INFO] 尝试选择第一个可用节点作为主节点...")
			for _, info := range nodeInfos {
				if info.Error == "" {
					actualMaster = info.Host
					log.Printf("[INFO] 选择第一个可用节点作为主节点: %s (%s)", info.Host.Name, info.Host.IP)
					break
				}
			}
		}

		if actualMaster == nil {
			log.Printf("[ERROR] 无法确定主节点，修复失败")
			log.Printf("[ERROR] 节点状态汇总:")
			for _, info := range nodeInfos {
				log.Printf("[ERROR]   - %s: read_only=%v, has_repl=%v, error=%s",
					info.Host.Name, info.IsReadOnly, info.HasRepl, info.Error)
			}
			return
		}

		log.Printf("[INFO] 确定主节点: %s (%s)", actualMaster.Name, actualMaster.IP)

		// Step 2: 更新 etcd 中的 leader_info
		log.Printf("[INFO] 更新 etcd leader_info...")
		etcdHost := ""
		for _, host := range cluster.Hosts {
			if host.IsEtcdNode() {
				etcdHost = host.IP
				break
			}
		}

		if etcdHost != "" {
			// 通过 SSH 到 etcd 节点执行 etcdctl 命令
			for i := range cluster.Hosts {
				host := &cluster.Hosts[i]
				if host.IsEtcdNode() {
					client, err := installer.NewSSHClient(host)
					if err != nil {
						continue
					}

					leaderInfo := fmt.Sprintf(`{"node_id":"%s","host":"%s","port":%d,"timestamp":%d}`,
						actualMaster.Name, actualMaster.IP, cluster.Settings.MySQLPort, time.Now().Unix())
					key := fmt.Sprintf("/%s/leader_info", cluster.Name)

					etcdCmd := fmt.Sprintf("etcdctl put '%s' '%s' 2>/dev/null || /usr/local/bin/etcdctl put '%s' '%s'",
						key, leaderInfo, key, leaderInfo)
					output, err := client.Run(etcdCmd)
					client.Close()

					if err != nil {
						log.Printf("[WARN] 更新 etcd leader_info 失败: %v, output: %s", err, output)
					} else {
						log.Printf("[INFO] etcd leader_info 已更新: %s -> %s", key, actualMaster.IP)
					}
					break
				}
			}
		}

		// Step 3: 确保主节点是 read-write 且没有复制配置
		log.Printf("[INFO] 配置主节点 %s...", actualMaster.Name)
		masterClient, err := installer.NewSSHClient(actualMaster)
		if err != nil {
			log.Printf("[ERROR] 无法连接主节点: %v", err)
			return
		}

		mysqlCmd := fmt.Sprintf("%s/mysql/bin/mysql -u root -p'%s' --socket=/tmp/mysql.sock",
			cluster.InstallPath, cluster.Settings.RootPassword)

		// 停止复制（如果有）并设置为可写
		masterClient.Run(fmt.Sprintf("%s -e \"STOP SLAVE; RESET SLAVE ALL;\"", mysqlCmd))
		masterClient.Run(fmt.Sprintf("%s -e \"SET GLOBAL read_only = OFF;\"", mysqlCmd))
		masterClient.Close()
		log.Printf("[INFO] 主节点 %s 已配置为 read-write", actualMaster.Name)

		// Step 4: 重新配置所有从节点
		for i := range cluster.Hosts {
			host := &cluster.Hosts[i]
			if !host.IsMySQLNode() || host.ID == actualMaster.ID {
				continue
			}

			log.Printf("[INFO] 配置从节点 %s 复制到 %s...", host.Name, actualMaster.IP)

			client, err := installer.NewSSHClient(host)
			if err != nil {
				log.Printf("[ERROR] [%s] 无法连接: %v", host.Name, err)
				continue
			}

			mysqlCmd := fmt.Sprintf("%s/mysql/bin/mysql -u root -p'%s' --socket=/tmp/mysql.sock",
				cluster.InstallPath, cluster.Settings.RootPassword)

			// 停止现有复制并完全重置
			log.Printf("[INFO] [%s] 停止并重置复制...", host.Name)
			client.Run(fmt.Sprintf("%s -e \"STOP SLAVE;\"", mysqlCmd))
			client.Run(fmt.Sprintf("%s -e \"RESET SLAVE ALL;\"", mysqlCmd))

			// 设置只读
			client.Run(fmt.Sprintf("%s -e \"SET GLOBAL read_only = ON;\"", mysqlCmd))

			// 重新配置复制 - 使用单独的命令确保每步都执行
			log.Printf("[INFO] [%s] 配置 CHANGE MASTER...", host.Name)
			changeMasterCmd := fmt.Sprintf(`%s -e "CHANGE MASTER TO MASTER_HOST='%s', MASTER_PORT=%d, MASTER_USER='%s', MASTER_PASSWORD='%s', MASTER_AUTO_POSITION=1;"`,
				mysqlCmd, actualMaster.IP, cluster.Settings.MySQLPort,
				cluster.Settings.ReplicationUser, cluster.Settings.ReplicationPass)

			output, err := client.Run(changeMasterCmd)
			if err != nil {
				log.Printf("[ERROR] [%s] CHANGE MASTER 失败: %v, output: %s", host.Name, err, output)
				client.Close()
				continue
			}
			log.Printf("[INFO] [%s] CHANGE MASTER 成功", host.Name)

			// 启动复制
			log.Printf("[INFO] [%s] 启动 SLAVE...", host.Name)
			output, err = client.Run(fmt.Sprintf("%s -e \"START SLAVE;\"", mysqlCmd))
			if err != nil {
				log.Printf("[ERROR] [%s] START SLAVE 失败: %v, output: %s", host.Name, err, output)
				client.Close()
				continue
			}
			log.Printf("[INFO] [%s] START SLAVE 成功", host.Name)

			// 验证复制状态
			time.Sleep(3 * time.Second)
			output, _ = client.Run(fmt.Sprintf("%s -e \"SHOW SLAVE STATUS\\G\"", mysqlCmd))
			log.Printf("[DEBUG] [%s] SHOW SLAVE STATUS 输出:\n%s", host.Name, output)

			if strings.Contains(output, "Slave_IO_Running: Yes") && strings.Contains(output, "Slave_SQL_Running: Yes") {
				log.Printf("[INFO] [%s] 复制配置成功 ✓", host.Name)
			} else {
				// 提取错误信息
				for _, line := range strings.Split(output, "\n") {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "Last_IO_Error:") || strings.HasPrefix(line, "Last_SQL_Error:") {
						if len(line) > 15 {
							log.Printf("[WARN] [%s] %s", host.Name, line)
						}
					}
				}
				log.Printf("[WARN] [%s] 复制可能未正常运行，请检查", host.Name)
			}

			client.Close()
		}

		// Step 5: 更新集群配置中的角色
		s.mu.Lock()
		for i := range cluster.Hosts {
			if cluster.Hosts[i].IsMySQLNode() {
				newRoles := []string{}
				for _, role := range cluster.Hosts[i].Roles {
					if role != installer.RoleMaster && role != installer.RoleSlave {
						newRoles = append(newRoles, role)
					}
				}
				if cluster.Hosts[i].ID == actualMaster.ID {
					newRoles = append(newRoles, installer.RoleMaster)
				} else {
					newRoles = append(newRoles, installer.RoleSlave)
				}
				cluster.Hosts[i].Roles = newRoles
			}
		}
		s.mu.Unlock()

		if err := s.saveClusters(); err != nil {
			log.Printf("[ERROR] 保存集群配置失败: %v", err)
		}

		log.Printf("[INFO] ========================================")
		log.Printf("[INFO] 主从修复完成！主节点: %s (%s)", actualMaster.Name, actualMaster.IP)
		log.Printf("[INFO] ========================================")
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":  "started",
		"message": "主从修复已开始",
	})
}

// handleGetLogs handles log retrieval requests
func (s *Server) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	nodeID := r.URL.Query().Get("node_id")
	filter := r.URL.Query().Get("filter") // "all" 显示所有日志，默认过滤掉高频日志

	s.mu.RLock()
	cluster, ok := s.clusters[id]
	s.mu.RUnlock()

	if !ok {
		writeError(w, http.StatusNotFound, "cluster not found")
		return
	}

	type LogEntry struct {
		Time    string `json:"time"`
		Level   string `json:"level"`
		Node    string `json:"node,omitempty"`
		Message string `json:"message"`
	}

	var logs []LogEntry

	// 获取指定节点或所有节点的日志
	var targetHosts []*installer.Host
	for i := range cluster.Hosts {
		if !cluster.Hosts[i].IsMySQLNode() {
			continue
		}
		if nodeID == "" || cluster.Hosts[i].ID == nodeID {
			targetHosts = append(targetHosts, &cluster.Hosts[i])
		}
	}

	for _, host := range targetHosts {
		client, err := installer.NewSSHClient(host)
		if err != nil {
			logs = append(logs, LogEntry{
				Time:    time.Now().Format("15:04:05"),
				Level:   "ERROR",
				Node:    host.Name,
				Message: fmt.Sprintf("无法连接到节点: %v", err),
			})
			continue
		}

		// 获取日志 - 默认过滤掉高频的 GET /state 日志，只显示重要日志
		var logCmd string
		if filter == "all" {
			// 显示所有日志
			logCmd = "tail -50 /var/log/mypatroni/mypatroni.log 2>/dev/null || echo 'No logs available'"
		} else {
			// 过滤掉 GET /state 等高频日志，只显示重要日志
			// 包括：failover、leader、replica、MySQL、repair、error、warn、state change、acquired、promoting、connected、environment
			logCmd = "grep -iE 'failover|leader|replica|mysql|repair|error|warn|state.change|acquired|promoting|connected|environment|reconfigure|detected|storing' /var/log/mypatroni/mypatroni.log 2>/dev/null | tail -50 || tail -20 /var/log/mypatroni/mypatroni.log 2>/dev/null || echo 'No logs available'"
		}
		output, err := client.Run(logCmd)
		client.Close()

		if err == nil && output != "" {
			lines := strings.Split(strings.TrimSpace(output), "\n")
			for _, line := range lines {
				if line == "" || line == "No logs available" {
					continue
				}
				// 跳过 GET /state 日志（即使在 all 模式下也可以选择跳过）
				if filter != "all" && strings.Contains(line, "GET /state") {
					continue
				}
				level := "INFO"
				if strings.Contains(line, "ERROR") {
					level = "ERROR"
				} else if strings.Contains(line, "WARN") {
					level = "WARN"
				}
				// 解析 JSON 日志中的时间戳
				logTime := time.Now().Format("15:04:05")
				if tsStart := strings.Index(line, `"timestamp":"`); tsStart > 0 {
					tsStart += 13
					if tsEnd := strings.Index(line[tsStart:], `"`); tsEnd > 0 {
						ts := line[tsStart : tsStart+tsEnd]
						// 解析时间戳，格式如 2025-12-21T11:01:40.303+0800
						if len(ts) >= 19 {
							logTime = ts[11:19] // 提取 HH:MM:SS
						}
					}
				}
				logs = append(logs, LogEntry{
					Time:    logTime,
					Level:   level,
					Node:    host.Name,
					Message: line,
				})
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"logs": logs,
	})
}

// handleRepairNode handles single node repair requests
// 修复单个异常节点：拉起所有服务（etcd、MySQL、mypatroni）、智能判断角色并配置
func (s *Server) handleRepairNode(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clusterID := vars["id"]
	nodeID := vars["nodeId"]

	s.mu.RLock()
	cluster, ok := s.clusters[clusterID]
	s.mu.RUnlock()

	if !ok {
		writeError(w, http.StatusNotFound, "cluster not found")
		return
	}

	// 查找目标节点
	var targetHost *installer.Host
	for i := range cluster.Hosts {
		if cluster.Hosts[i].ID == nodeID {
			targetHost = &cluster.Hosts[i]
			break
		}
	}

	if targetHost == nil {
		writeError(w, http.StatusNotFound, "node not found")
		return
	}

	// 在后台执行修复
	go func() {
		log.Printf("[INFO] ========================================")
		log.Printf("[INFO] 开始修复节点: %s (%s)", targetHost.Name, targetHost.IP)
		log.Printf("[INFO] ========================================")

		client, err := installer.NewSSHClient(targetHost)
		if err != nil {
			log.Printf("[ERROR] [%s] 无法连接到节点: %v", targetHost.Name, err)
			return
		}
		defer client.Close()

		// Step 1: 检查并启动 etcd 服务（如果该节点是 etcd 节点）
		if targetHost.IsEtcdNode() {
			log.Printf("[INFO] [%s] 检查 etcd 服务状态...", targetHost.Name)
			output, err := client.Run("systemctl is-active etcd 2>/dev/null || echo 'inactive'")
			etcdActive := err == nil && strings.TrimSpace(output) == "active"

			if !etcdActive {
				log.Printf("[INFO] [%s] etcd 服务未运行，尝试启动...", targetHost.Name)
				output, err = client.RunWithSudo("systemctl start etcd 2>&1")
				if err != nil {
					log.Printf("[ERROR] [%s] 启动 etcd 失败: %v, output: %s", targetHost.Name, err, output)
				} else {
					time.Sleep(3 * time.Second)
					log.Printf("[INFO] [%s] etcd 服务已启动 ✓", targetHost.Name)
				}
			} else {
				log.Printf("[INFO] [%s] etcd 服务正在运行 ✓", targetHost.Name)
			}
		}

		// Step 2: 检查并启动 MySQL 服务（如果该节点是 MySQL 节点）
		if !targetHost.IsMySQLNode() {
			log.Printf("[INFO] [%s] 该节点不是 MySQL 节点，跳过 MySQL 修复", targetHost.Name)
			log.Printf("[INFO] ========================================")
			log.Printf("[INFO] 节点 %s 修复完成！", targetHost.Name)
			log.Printf("[INFO] ========================================")
			return
		}

		mysqlCmd := fmt.Sprintf("%s/mysql/bin/mysql -u root -p'%s' --socket=/tmp/mysql.sock",
			cluster.InstallPath, cluster.Settings.RootPassword)

		log.Printf("[INFO] [%s] 检查 MySQL 服务状态...", targetHost.Name)
		output, err := client.Run("systemctl is-active mysql 2>/dev/null || systemctl is-active mysqld 2>/dev/null || echo 'inactive'")
		mysqlActive := err == nil && strings.TrimSpace(output) == "active"

		if !mysqlActive {
			log.Printf("[INFO] [%s] MySQL 服务未运行，尝试启动...", targetHost.Name)
			output, err = client.RunWithSudo("systemctl start mysql 2>/dev/null || systemctl start mysqld 2>/dev/null")
			if err != nil {
				log.Printf("[ERROR] [%s] 启动 MySQL 失败: %v, output: %s", targetHost.Name, err, output)
				return
			}
			time.Sleep(5 * time.Second)

			// 验证 MySQL 是否启动成功
			output, err = client.Run(fmt.Sprintf("%s -e 'SELECT 1' 2>&1", mysqlCmd))
			if err != nil {
				log.Printf("[ERROR] [%s] MySQL 启动后无法连接: %v, output: %s", targetHost.Name, err, output)
				return
			}
			log.Printf("[INFO] [%s] MySQL 服务已启动 ✓", targetHost.Name)
		} else {
			log.Printf("[INFO] [%s] MySQL 服务正在运行 ✓", targetHost.Name)
		}

		// Step 3: 查找当前集群的主节点
		log.Printf("[INFO] [%s] 查找集群主节点...", targetHost.Name)
		var masterHost *installer.Host
		var hasOtherLeader bool

		// 方法1: 通过 agent API 查找
		for i := range cluster.Hosts {
			host := &cluster.Hosts[i]
			if !host.IsMySQLNode() || host.ID == targetHost.ID {
				continue
			}

			agentURL := fmt.Sprintf("http://%s:%d/state", host.IP, cluster.Settings.HAAgentPort)
			httpClient := &http.Client{Timeout: 2 * time.Second}
			resp, err := httpClient.Get(agentURL)
			if err == nil && resp.StatusCode == http.StatusOK {
				var agentState struct {
					Role string `json:"role"`
				}
				if json.NewDecoder(resp.Body).Decode(&agentState) == nil {
					if agentState.Role == "leader" {
						masterHost = host
						hasOtherLeader = true
						log.Printf("[INFO] [%s] 找到主节点: %s (%s)", targetHost.Name, host.Name, host.IP)
						resp.Body.Close()
						break
					}
				}
				resp.Body.Close()
			}
		}

		// 方法2: 如果通过 agent 找不到，检查 MySQL read_only 状态
		if masterHost == nil {
			log.Printf("[INFO] [%s] 通过 agent 未找到主节点，检查 MySQL 状态...", targetHost.Name)
			for i := range cluster.Hosts {
				host := &cluster.Hosts[i]
				if !host.IsMySQLNode() || host.ID == targetHost.ID {
					continue
				}

				tmpClient, err := installer.NewSSHClient(host)
				if err != nil {
					continue
				}

				tmpMysqlCmd := fmt.Sprintf("%s/mysql/bin/mysql -u root -p'%s' --socket=/tmp/mysql.sock",
					cluster.InstallPath, cluster.Settings.RootPassword)
				output, err := tmpClient.Run(fmt.Sprintf("%s -N -e 'SELECT @@read_only' 2>&1", tmpMysqlCmd))
				tmpClient.Close()

				if err == nil && strings.TrimSpace(output) == "0" {
					masterHost = host
					hasOtherLeader = true
					log.Printf("[INFO] [%s] 找到主节点（read_only=0）: %s (%s)", targetHost.Name, host.Name, host.IP)
					break
				}
			}
		}

		// Step 4: 检查并启动 mypatroni 服务
		log.Printf("[INFO] [%s] 检查 mypatroni 服务状态...", targetHost.Name)
		output, err = client.Run("systemctl is-active mypatroni 2>/dev/null || echo 'inactive'")
		agentActive := err == nil && strings.TrimSpace(output) == "active"

		if !agentActive {
			log.Printf("[INFO] [%s] mypatroni 服务未运行，尝试启动...", targetHost.Name)
			output, err = client.RunWithSudo("systemctl start mypatroni 2>&1")
			if err != nil {
				log.Printf("[ERROR] [%s] 启动 mypatroni 失败: %v, output: %s", targetHost.Name, err, output)
			} else {
				log.Printf("[INFO] [%s] mypatroni 服务已启动 ✓", targetHost.Name)
				agentActive = true
			}
		} else {
			log.Printf("[INFO] [%s] mypatroni 服务正在运行 ✓", targetHost.Name)
		}

		// Step 5: 判断节点角色并配置
		// 如果集群中已经有其他主节点，则将此节点配置为从节点
		if hasOtherLeader && masterHost != nil {
			log.Printf("[INFO] [%s] 集群已有主节点 %s，将此节点配置为从节点", targetHost.Name, masterHost.Name)

			// 如果 agent 已启动，等待它自动配置
			if agentActive {
				log.Printf("[INFO] [%s] 等待 mypatroni 自动配置复制（15秒）...", targetHost.Name)
				time.Sleep(15 * time.Second)

				// 检查 agent 是否自动配置了复制
				output, _ = client.Run(fmt.Sprintf("%s -e 'SHOW SLAVE STATUS\\G' 2>&1", mysqlCmd))
				if strings.Contains(output, "Slave_IO_Running: Yes") && strings.Contains(output, "Slave_SQL_Running: Yes") {
					log.Printf("[INFO] [%s] mypatroni 已自动配置复制 ✓", targetHost.Name)
					log.Printf("[INFO] ========================================")
					log.Printf("[INFO] 节点 %s 修复完成（由 agent 自动配置）", targetHost.Name)
					log.Printf("[INFO] ========================================")
					return
				}
				log.Printf("[WARN] [%s] mypatroni 未自动配置复制，手动配置...", targetHost.Name)
			}

			// 手动配置为从节点
			log.Printf("[INFO] [%s] 手动配置为从节点，主节点: %s (%s)", targetHost.Name, masterHost.Name, masterHost.IP)

			// 停止现有复制
			log.Printf("[INFO] [%s] 停止现有复制...", targetHost.Name)
			client.Run(fmt.Sprintf("%s -e 'STOP SLAVE;' 2>&1", mysqlCmd))
			client.Run(fmt.Sprintf("%s -e 'RESET SLAVE ALL;' 2>&1", mysqlCmd))

			// 设置只读
			log.Printf("[INFO] [%s] 设置为只读模式...", targetHost.Name)
			output, err = client.Run(fmt.Sprintf("%s -e 'SET GLOBAL read_only = ON; SET GLOBAL super_read_only = ON;' 2>&1", mysqlCmd))
			if err != nil {
				log.Printf("[WARN] [%s] 设置只读失败: %v, output: %s", targetHost.Name, err, output)
			}

			// 配置 CHANGE MASTER
			log.Printf("[INFO] [%s] 配置 CHANGE MASTER...", targetHost.Name)
			changeMasterCmd := fmt.Sprintf(`%s -e "CHANGE MASTER TO MASTER_HOST='%s', MASTER_PORT=%d, MASTER_USER='%s', MASTER_PASSWORD='%s', MASTER_AUTO_POSITION=1;" 2>&1`,
				mysqlCmd, masterHost.IP, cluster.Settings.MySQLPort,
				cluster.Settings.ReplicationUser, cluster.Settings.ReplicationPass)

			output, err = client.Run(changeMasterCmd)
			if err != nil {
				log.Printf("[ERROR] [%s] CHANGE MASTER 失败: %v, output: %s", targetHost.Name, err, output)
				return
			}
			log.Printf("[INFO] [%s] CHANGE MASTER 成功", targetHost.Name)

			// 启动复制
			log.Printf("[INFO] [%s] 启动复制...", targetHost.Name)
			output, err = client.Run(fmt.Sprintf("%s -e 'START SLAVE;' 2>&1", mysqlCmd))
			if err != nil {
				log.Printf("[ERROR] [%s] START SLAVE 失败: %v, output: %s", targetHost.Name, err, output)
				return
			}

			// 验证复制状态
			time.Sleep(3 * time.Second)
			output, _ = client.Run(fmt.Sprintf("%s -e 'SHOW SLAVE STATUS\\G' 2>&1", mysqlCmd))

			ioRunning := strings.Contains(output, "Slave_IO_Running: Yes")
			sqlRunning := strings.Contains(output, "Slave_SQL_Running: Yes")

			if ioRunning && sqlRunning {
				log.Printf("[INFO] [%s] 复制配置成功 ✓", targetHost.Name)
				log.Printf("[INFO] [%s]   - Slave_IO_Running: Yes", targetHost.Name)
				log.Printf("[INFO] [%s]   - Slave_SQL_Running: Yes", targetHost.Name)
			} else {
				log.Printf("[WARN] [%s] 复制可能未正常运行:", targetHost.Name)
				log.Printf("[WARN] [%s]   - Slave_IO_Running: %v", targetHost.Name, ioRunning)
				log.Printf("[WARN] [%s]   - Slave_SQL_Running: %v", targetHost.Name, sqlRunning)

				// 提取错误信息
				for _, line := range strings.Split(output, "\n") {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "Last_IO_Error:") || strings.HasPrefix(line, "Last_SQL_Error:") {
						if len(line) > 15 && !strings.HasSuffix(line, ": ") {
							log.Printf("[WARN] [%s] %s", targetHost.Name, line)
						}
					}
				}
			}

			// 重启 mypatroni 确保配置生效
			if agentActive {
				log.Printf("[INFO] [%s] 重启 mypatroni 服务确保配置生效...", targetHost.Name)
				client.RunWithSudo("systemctl restart mypatroni 2>&1")
			}
		} else {
			// 没有其他主节点，让 agent 自动处理（可能会竞选为主节点）
			log.Printf("[INFO] [%s] 集群中没有其他主节点，让 mypatroni 自动处理角色", targetHost.Name)
			if agentActive {
				log.Printf("[INFO] [%s] 等待 mypatroni 自动配置（20秒）...", targetHost.Name)
				time.Sleep(20 * time.Second)
			}
		}

		log.Printf("[INFO] ========================================")
		log.Printf("[INFO] 节点 %s 修复完成！", targetHost.Name)
		log.Printf("[INFO] ========================================")
	}()

	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"status":  "started",
		"message": fmt.Sprintf("节点 %s 修复已开始", targetHost.Name),
	})
}
