package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"mysql-ha/internal/config"
	"mysql-ha/internal/dcs"
	"mysql-ha/internal/election"
	"mysql-ha/internal/log"
	"mysql-ha/internal/mysql"
)

// Agent is the core HA agent running on each MySQL node
type Agent struct {
	config         *config.Config
	dcs            dcs.DCS
	mysql          *mysql.Manager
	lockManager    *election.LockManager
	logger         *log.Logger
	state          *NodeState
	stopCh         chan struct{}
	mu             sync.RWMutex
	mysqlConnected bool
	dcsConnected   bool
	lastLeaderHost string // 缓存上次的 leader host，用于检测 leader 变更
}

// NewAgent creates a new HA Agent
func NewAgent(cfg *config.Config, d dcs.DCS, mysqlMgr *mysql.Manager, logger *log.Logger) *Agent {
	hostname, _ := os.Hostname()
	return &Agent{
		config: cfg,
		dcs:    d,
		mysql:  mysqlMgr,
		logger: logger,
		state: &NodeState{
			NodeID:   cfg.Name,
			Hostname: hostname,
			Role:     RoleUnknown,
		},
		stopCh: make(chan struct{}),
	}
}

// Start starts the agent
func (a *Agent) Start(ctx context.Context) error {
	a.logger.Info("starting HA agent")

	// 启动时先检查和修复 MySQL 环境
	a.logger.Info("performing initial MySQL environment check...")
	if err := a.repairMySQLEnvironment(); err != nil {
		a.logger.Warn(fmt.Sprintf("initial MySQL environment repair failed: %v, will retry during connection", err))
	}

	// Connect to DCS with retry
	if err := a.connectDCSWithRetry(ctx); err != nil {
		return fmt.Errorf("failed to connect to DCS after retries: %w", err)
	}

	// Connect to MySQL with retry (non-blocking, will retry in background)
	go a.connectMySQLWithRetry(ctx)

	// Initialize lock manager - use Scope (cluster name) as lock key so all nodes compete for the same lock
	ttl := time.Duration(a.config.HA.TTL) * time.Second
	lockKey := a.config.Scope
	if lockKey == "" {
		lockKey = "default"
	}
	a.lockManager = election.NewLockManager(a.dcs, lockKey, ttl)

	// Register with DCS
	if err := a.register(ctx); err != nil {
		a.logger.Warn(fmt.Sprintf("failed to register initially: %v, will retry", err))
	}

	// Start main loop
	go a.runLoop(ctx)

	a.logger.Info("HA agent started")
	return nil
}

// connectDCSWithRetry connects to DCS with exponential backoff
func (a *Agent) connectDCSWithRetry(ctx context.Context) error {
	maxRetries := 30
	baseDelay := 2 * time.Second
	maxDelay := 30 * time.Second

	for i := range maxRetries {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := a.dcs.Connect(ctx); err != nil {
			delay := min(baseDelay*time.Duration(1<<uint(i)), maxDelay)
			a.logger.Warn(fmt.Sprintf("failed to connect to DCS (attempt %d/%d): %v, retrying in %v", i+1, maxRetries, err, delay))
			time.Sleep(delay)
			continue
		}

		a.mu.Lock()
		a.dcsConnected = true
		a.mu.Unlock()
		a.logger.Info("connected to DCS")
		return nil
	}
	return fmt.Errorf("max retries exceeded")
}

// connectMySQLWithRetry connects to MySQL with retry, runs in background
func (a *Agent) connectMySQLWithRetry(ctx context.Context) {
	maxRetries := 0 // unlimited retries
	baseDelay := 5 * time.Second
	maxDelay := 60 * time.Second
	attempt := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-a.stopCh:
			return
		default:
		}

		// 在尝试连接 MySQL 前，先确保 MySQL 环境正常
		if attempt > 0 && attempt%3 == 0 {
			// 每 3 次连接失败后，尝试修复 MySQL 环境
			a.logger.Info("attempting to repair MySQL environment before retry")
			if err := a.repairMySQLEnvironment(); err != nil {
				a.logger.Warn(fmt.Sprintf("failed to repair MySQL environment: %v", err))
			}
		}

		if err := a.mysql.Connect(); err != nil {
			attempt++
			delay := min(baseDelay*time.Duration(1<<uint(min(attempt, 5))), maxDelay)
			if maxRetries > 0 && attempt >= maxRetries {
				a.logger.Error(fmt.Sprintf("failed to connect to MySQL after %d attempts, giving up", attempt))
				return
			}
			a.logger.Warn(fmt.Sprintf("failed to connect to MySQL (attempt %d): %v, retrying in %v", attempt, err, delay))
			time.Sleep(delay)
			continue
		}

		// Get MySQL version
		version, err := a.mysql.GetVersion()
		if err != nil {
			a.logger.Warn(fmt.Sprintf("connected to MySQL but failed to get version: %v", err))
		} else {
			a.mu.Lock()
			a.state.MySQLVersion = version
			a.mu.Unlock()
		}

		a.mu.Lock()
		a.mysqlConnected = true
		a.mu.Unlock()
		a.logger.Info("connected to MySQL")
		return
	}
}

// Stop stops the agent
func (a *Agent) Stop() error {
	a.logger.Info("stopping HA agent")

	close(a.stopCh)

	if a.lockManager != nil {
		a.lockManager.StopRenewal()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		a.lockManager.Release(ctx)
	}

	if a.mysql != nil {
		a.mysql.Close()
	}

	if a.dcs != nil {
		a.dcs.Close()
	}

	a.logger.Info("HA agent stopped")
	return nil
}

// GetState returns the current node state
func (a *Agent) GetState() any {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state
}

// IsLeader returns true if this node is the leader
func (a *Agent) IsLeader() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state.Role == RoleLeader
}

// getMyHost returns this node's advertise host
func (a *Agent) getMyHost() string {
	if a.config.AdvertiseHost != "" {
		a.logger.Debug(fmt.Sprintf("getMyHost: using AdvertiseHost=%s", a.config.AdvertiseHost))
		return a.config.AdvertiseHost
	}
	a.logger.Debug(fmt.Sprintf("getMyHost: AdvertiseHost is empty, falling back to MySQL.Host=%s", a.config.MySQL.Host))
	return a.config.MySQL.Host
}

// Promote promotes this node to leader
func (a *Agent) Promote() error {
	a.logger.Info("promoting to leader")

	// Check if we are currently a replica
	replStatus, err := a.mysql.GetReplicationStatus()
	if err != nil {
		a.logger.Warn(fmt.Sprintf("failed to get replication status: %v", err))
	}

	// If we are a replica, we need to stop replication and promote
	if replStatus != nil {
		a.logger.Info(fmt.Sprintf("stopping replication from %s:%d for promotion", replStatus.MasterHost, replStatus.MasterPort))

		// Stop replication
		if err := a.mysql.StopReplication(); err != nil {
			a.logger.Warn(fmt.Sprintf("failed to stop replication: %v", err))
		}

		// Reset replication configuration completely
		if err := a.mysql.ResetReplication(); err != nil {
			a.logger.Warn(fmt.Sprintf("failed to reset replication: %v", err))
		}

		a.logger.Info("replication stopped and reset for promotion")
	}

	// Set MySQL to read-write
	if err := a.mysql.SetReadOnly(false); err != nil {
		return fmt.Errorf("failed to set read-write: %w", err)
	}

	a.mu.Lock()
	oldRole := a.state.Role
	a.state.Role = RoleLeader
	a.state.ReplicationInfo = nil // Clear replication info since we're now leader
	a.mu.Unlock()

	// Store leader info in etcd so other nodes can find us
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := a.storeLeaderInfo(ctx); err != nil {
		a.logger.Error(fmt.Sprintf("failed to store leader info: %v", err))
		return fmt.Errorf("failed to store leader info: %w", err)
	}

	a.logger.StateChange(string(oldRole), string(RoleLeader), "promotion")
	a.logger.Info(fmt.Sprintf("successfully promoted to leader, leader info stored in etcd"))
	return nil
}

// storeLeaderInfo stores the current leader's connection info in etcd
func (a *Agent) storeLeaderInfo(ctx context.Context) error {
	host := a.getMyHost()
	leaderInfo := fmt.Sprintf(`{"node_id":"%s","host":"%s","port":%d,"timestamp":%d}`,
		a.config.Name, host, a.config.MySQL.Port, time.Now().Unix())
	// 使用 "leader_info" 作为 key，DCS 会自动加上 prefix
	key := "leader_info"
	a.logger.Info(fmt.Sprintf("storing leader info in etcd: key=%s, host=%s, value=%s", key, host, leaderInfo))
	return a.dcs.Set(ctx, key, []byte(leaderInfo))
}

// getLeaderInfo retrieves the current leader's connection info from etcd
func (a *Agent) getLeaderInfo(ctx context.Context) (string, string, int, error) {
	// 使用 "leader_info" 作为 key，DCS 会自动加上 prefix
	key := "leader_info"
	data, err := a.dcs.Get(ctx, key)
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to get leader info from etcd: %w", err)
	}
	if data == nil {
		return "", "", 0, fmt.Errorf("no leader info found at key %s", key)
	}

	// Parse JSON manually
	str := string(data)
	var nodeID, host string
	var port int

	// Find node_id
	nodeIDStart := findSubstring(str, `"node_id":"`) + 11
	if nodeIDStart > 10 {
		nodeIDEnd := findSubstring(str[nodeIDStart:], `"`)
		if nodeIDEnd > 0 {
			nodeID = str[nodeIDStart : nodeIDStart+nodeIDEnd]
		}
	}
	// Find host
	hostStart := findSubstring(str, `"host":"`) + 8
	if hostStart > 7 {
		hostEnd := findSubstring(str[hostStart:], `"`)
		if hostEnd > 0 {
			host = str[hostStart : hostStart+hostEnd]
		}
	}
	// Find port
	portStart := findSubstring(str, `"port":`) + 7
	if portStart > 6 {
		fmt.Sscanf(str[portStart:], "%d", &port)
	}

	if host == "" || port == 0 {
		return "", "", 0, fmt.Errorf("invalid leader info format: %s", str)
	}

	return nodeID, host, port, nil
}

// findSubstring returns the index of substr in s, or -1 if not found
func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// configureAsReplica configures this node as a replica of the specified leader
func (a *Agent) configureAsReplica(ctx context.Context, leaderHost string, leaderPort int) error {
	myHost := a.getMyHost()

	// Don't configure replication to ourselves
	if leaderHost == myHost || leaderHost == "127.0.0.1" {
		return fmt.Errorf("cannot configure replication to ourselves")
	}

	a.logger.Info(fmt.Sprintf("configuring as replica of %s:%d", leaderHost, leaderPort))

	// Stop any existing replication first
	if err := a.mysql.StopReplication(); err != nil {
		a.logger.Warn(fmt.Sprintf("failed to stop existing replication: %v", err))
	}

	// Reset replication to clear old master info
	if err := a.mysql.ResetReplication(); err != nil {
		a.logger.Warn(fmt.Sprintf("failed to reset replication: %v", err))
	}

	// Set read-only first
	if err := a.mysql.SetReadOnly(true); err != nil {
		a.logger.Warn(fmt.Sprintf("failed to set read-only: %v", err))
	}

	// Configure replication to the leader
	if err := a.mysql.StartReplication(leaderHost, leaderPort,
		a.config.MySQL.ReplicationUser, a.config.MySQL.ReplicationPass); err != nil {
		return fmt.Errorf("failed to start replication to %s:%d: %w", leaderHost, leaderPort, err)
	}

	// Wait a bit and verify replication is running
	time.Sleep(2 * time.Second)
	replStatus, err := a.mysql.GetReplicationStatus()
	if err != nil {
		a.logger.Warn(fmt.Sprintf("failed to verify replication status: %v", err))
		// Still update state to indicate we tried to configure replication
		a.mu.Lock()
		a.state.Role = RoleReplica
		a.state.ReplicationInfo = &ReplicationInfo{
			MasterHost: leaderHost,
			MasterPort: leaderPort,
		}
		a.mu.Unlock()
		return fmt.Errorf("failed to verify replication status: %w", err)
	}
	if replStatus == nil {
		a.logger.Warn("replication not configured after setup")
		return fmt.Errorf("replication not configured after setup")
	}

	// Update state regardless of whether replication is fully running
	// This allows the next tick to detect and retry if needed
	a.mu.Lock()
	a.state.Role = RoleReplica
	a.state.ReplicationInfo = &ReplicationInfo{
		MasterHost:      leaderHost,
		MasterPort:      leaderPort,
		SlaveIORunning:  replStatus.SlaveIORunning,
		SlaveSQLRunning: replStatus.SlaveSQLRunning,
	}
	a.mu.Unlock()

	if !replStatus.SlaveIORunning || !replStatus.SlaveSQLRunning {
		a.logger.Warn(fmt.Sprintf("replication configured but not fully running: IO=%v, SQL=%v, error=%s",
			replStatus.SlaveIORunning, replStatus.SlaveSQLRunning, replStatus.LastError))
		// Return nil to indicate configuration was done, next tick will check and retry if needed
		return nil
	}

	a.logger.Info(fmt.Sprintf("successfully configured as replica of %s:%d", leaderHost, leaderPort))
	return nil
}

// Demote demotes this node to replica and configures replication to the new leader
func (a *Agent) Demote(ctx context.Context) error {
	a.logger.Info("demoting to replica")

	// Get current leader info from etcd
	_, leaderHost, leaderPort, err := a.getLeaderInfo(ctx)
	if err != nil {
		a.logger.Warn(fmt.Sprintf("failed to get leader info for demotion: %v", err))
		// Just set read-only if we can't get leader info
		if err := a.mysql.SetReadOnly(true); err != nil {
			return fmt.Errorf("failed to set read-only: %w", err)
		}
		a.mu.Lock()
		a.state.Role = RoleReplica
		a.mu.Unlock()
		return nil
	}

	// Configure as replica of the new leader
	if err := a.configureAsReplica(ctx, leaderHost, leaderPort); err != nil {
		a.logger.Error(fmt.Sprintf("failed to configure as replica: %v", err))
		// At least set read-only
		a.mysql.SetReadOnly(true)
		a.mu.Lock()
		a.state.Role = RoleReplica
		a.mu.Unlock()
		return err
	}

	a.logger.StateChange(string(RoleLeader), string(RoleReplica), "demotion")
	return nil
}

// register registers this node with DCS
func (a *Agent) register(ctx context.Context) error {
	member := &dcs.Member{
		NodeID:   a.state.NodeID,
		Hostname: a.state.Hostname,
		APIAddr:  fmt.Sprintf("%s:%d", a.config.API.Listen, a.config.API.Port),
		Role:     string(a.state.Role),
	}
	return a.dcs.RegisterMember(ctx, member)
}

// runLoop is the main control loop
func (a *Agent) runLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(a.config.HA.LoopWait) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-a.stopCh:
			return
		case <-ticker.C:
			a.tick(ctx)
		}
	}
}

// tick performs one iteration of the control loop
// 核心逻辑：
// 1. etcd 锁是唯一的 leader 判断标准
// 2. 持有锁的节点是 leader，必须确保 MySQL 是 read-write 且 leader_info 正确
// 3. 没有锁的节点是 replica，必须确保 MySQL 是 read-only 且复制指向正确的 leader
// 4. 异常节点恢复后，如果不持有锁，必须降级为 replica
func (a *Agent) tick(ctx context.Context) {
	// 更新 MySQL 连接状态
	if err := a.mysql.Ping(); err != nil {
		a.mu.Lock()
		wasConnected := a.mysqlConnected
		a.mysqlConnected = false
		a.state.IsHealthy = false
		a.mu.Unlock()

		if wasConnected {
			a.logger.Warn(fmt.Sprintf("MySQL connection lost: %v", err))
			go a.connectMySQLWithRetry(ctx)
		}

		// 如果我们持有锁但 MySQL 挂了，必须释放锁让其他节点接管
		if a.lockManager.IsLeader() {
			a.logger.Warn("MySQL not connected but holding leader lock, releasing for failover")
			a.lockManager.StopRenewal()
			if err := a.lockManager.Release(ctx); err != nil {
				a.logger.Error(fmt.Sprintf("failed to release lock: %v", err))
			}
			a.mu.Lock()
			a.state.Role = RoleUnknown
			a.mu.Unlock()
		}
		return
	}

	// MySQL 连接正常
	a.mu.Lock()
	if !a.mysqlConnected {
		a.logger.Info("MySQL connection restored")
	}
	a.mysqlConnected = true
	a.state.IsHealthy = true
	a.state.LastUpdated = time.Now()
	a.mu.Unlock()

	// 获取当前 etcd 中的 leader 信息
	leaderNodeID, leaderHost, leaderPort, leaderErr := a.getLeaderInfo(ctx)
	myHost := a.getMyHost()
	amILeaderInEtcd := leaderErr == nil && leaderHost == myHost

	// 检查我是否持有 etcd 锁
	holdingLock := a.lockManager.IsLeader()

	a.logger.Debug(fmt.Sprintf("tick: holdingLock=%v, amILeaderInEtcd=%v, leaderHost=%s, myHost=%s",
		holdingLock, amILeaderInEtcd, leaderHost, myHost))

	if holdingLock {
		// 我持有锁，我应该是 leader
		a.handleAsLeader(ctx, leaderHost, myHost)
	} else {
		// 我不持有锁，尝试获取或作为 replica
		a.handleAsNonLeader(ctx, leaderNodeID, leaderHost, leaderPort, leaderErr, myHost)
	}

	// 更新 etcd 注册信息
	a.register(ctx)
}

// handleAsLeader 处理持有锁的情况（我是 leader）
func (a *Agent) handleAsLeader(ctx context.Context, leaderHost, myHost string) {
	// 续约锁
	if err := a.lockManager.Renew(ctx); err != nil {
		a.logger.Error(fmt.Sprintf("failed to renew lock: %v, will demote", err))
		a.lockManager.StopRenewal()
		a.Demote(ctx)
		return
	}

	// 确保 etcd 中的 leader_info 指向我（无论当前值是什么）
	// 这是关键：即使 leaderHost 为空或指向其他节点，都要更新
	if leaderHost != myHost {
		a.logger.Info(fmt.Sprintf("leader_info points to '%s' but I hold the lock (my host: %s), updating leader_info", leaderHost, myHost))
		if err := a.storeLeaderInfo(ctx); err != nil {
			a.logger.Error(fmt.Sprintf("failed to update leader info: %v", err))
		} else {
			a.logger.Info(fmt.Sprintf("successfully updated leader_info to point to me: %s", myHost))
		}
	}

	// 确保 MySQL 是 read-write 模式
	readOnly, err := a.mysql.IsReadOnly()
	if err != nil {
		a.logger.Warn(fmt.Sprintf("failed to check read-only status: %v", err))
	} else if readOnly {
		a.logger.Info("leader MySQL is read-only, setting to read-write")
		if err := a.mysql.SetReadOnly(false); err != nil {
			a.logger.Error(fmt.Sprintf("failed to set read-write: %v", err))
		}
	}

	// 确保没有复制配置（leader 不应该是 slave）
	replStatus, _ := a.mysql.GetReplicationStatus()
	if replStatus != nil {
		a.logger.Warn("leader has replication configured, stopping and resetting")
		a.mysql.StopReplication()
		a.mysql.ResetReplication()
	}

	a.mu.Lock()
	a.state.Role = RoleLeader
	a.state.ReplicationInfo = nil
	a.mu.Unlock()
}

// handleAsNonLeader 处理不持有锁的情况
func (a *Agent) handleAsNonLeader(ctx context.Context, leaderNodeID, leaderHost string, leaderPort int, leaderErr error, myHost string) {
	// 尝试获取锁
	acquired, err := a.lockManager.TryAcquire(ctx)
	if err != nil {
		a.logger.Debug(fmt.Sprintf("failed to acquire lock: %v", err))
	}

	if acquired {
		// 成功获取锁，执行故障转移
		a.logger.Info("acquired leadership lock, performing failover")
		a.performFailover(ctx)
		return
	}

	// 没有获取到锁，我应该是 replica
	// 如果 etcd 中没有 leader 信息，尝试通过查询所有成员找到 leader
	if leaderErr != nil {
		a.logger.Debug(fmt.Sprintf("no leader info in etcd: %v, trying to find leader from members", leaderErr))

		// 检查当前复制状态，如果旧 master 不可达，需要等待新 leader 出现
		replStatus, err := a.mysql.GetReplicationStatus()
		if err == nil && replStatus != nil {
			if !replStatus.SlaveIORunning || !replStatus.SlaveSQLRunning {
				a.logger.Warn(fmt.Sprintf("replication not running (IO=%v, SQL=%v) and no leader info available, waiting for new leader",
					replStatus.SlaveIORunning, replStatus.SlaveSQLRunning))
			}
		}

		// 至少确保是只读模式
		if err := a.mysql.SetReadOnly(true); err != nil {
			a.logger.Error(fmt.Sprintf("failed to set read-only: %v", err))
		}
		a.mu.Lock()
		a.state.Role = RoleReplica
		a.mu.Unlock()
		return
	}

	// 如果 leader 是我自己（但我没有锁），说明我是旧的 leader，需要降级
	if leaderHost == myHost {
		a.logger.Warn("leader_info points to me but I don't hold the lock, I'm a stale leader, setting read-only")
		// 设置为只读模式，等待新 leader 更新 leader_info
		if err := a.mysql.SetReadOnly(true); err != nil {
			a.logger.Error(fmt.Sprintf("failed to set read-only: %v", err))
		}
		a.mu.Lock()
		a.state.Role = RoleReplica
		a.mu.Unlock()
		return
	}

	// 确保我配置为 replica，复制指向正确的 leader
	a.ensureReplicaOf(ctx, leaderNodeID, leaderHost, leaderPort)
}

// performFailover 执行故障转移，将自己提升为 leader
func (a *Agent) performFailover(ctx context.Context) {
	a.logger.Info("performing failover - promoting to leader")

	// 检查当前复制状态
	replStatus, _ := a.mysql.GetReplicationStatus()
	if replStatus != nil {
		a.logger.Info(fmt.Sprintf("stopping replication from %s:%d", replStatus.MasterHost, replStatus.MasterPort))
	}

	// 执行提升
	if err := a.Promote(); err != nil {
		a.logger.Error(fmt.Sprintf("failed to promote: %v, releasing lock", err))
		a.lockManager.Release(ctx)
		return
	}

	// 提升成功后，立即更新 leader_info 并重试确保成功
	// 这是关键步骤，必须成功才能让其他节点知道新 leader
	maxRetries := 5
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		if err := a.storeLeaderInfo(ctx); err != nil {
			lastErr = err
			a.logger.Warn(fmt.Sprintf("failed to store leader info (attempt %d/%d): %v", i+1, maxRetries, err))
			time.Sleep(time.Second * time.Duration(i+1)) // 递增延迟
			continue
		}
		a.logger.Info("successfully stored leader info in etcd")
		lastErr = nil
		break
	}

	if lastErr != nil {
		a.logger.Error(fmt.Sprintf("failed to store leader info after %d attempts: %v, but continuing as leader", maxRetries, lastErr))
		// 不释放锁，因为我们已经提升成功了，只是 etcd 写入有问题
		// 继续作为 leader 运行，后续的 tick 会继续尝试更新 leader_info
	}

	// 启动锁续约
	a.lockManager.StartRenewal(ctx)
	a.logger.Info("failover completed, I am now the leader")
}

// ensureReplicaOf 确保本节点是指定 leader 的 replica
func (a *Agent) ensureReplicaOf(ctx context.Context, leaderNodeID, leaderHost string, leaderPort int) {
	myHost := a.getMyHost()

	// 不能复制自己
	if leaderHost == myHost {
		a.logger.Debug(fmt.Sprintf("skipping replication setup: leader host %s is myself", leaderHost))
		return
	}

	a.logger.Debug(fmt.Sprintf("ensureReplicaOf: checking replication to leader %s (node: %s) at %s:%d",
		leaderNodeID, leaderHost, leaderHost, leaderPort))

	// 检查当前复制状态
	replStatus, err := a.mysql.GetReplicationStatus()
	if err != nil {
		a.logger.Warn(fmt.Sprintf("failed to get replication status: %v", err))
		return
	}

	needReconfigure := false
	reason := ""

	if replStatus == nil {
		// 没有配置复制，需要配置
		needReconfigure = true
		reason = "no replication configured"
	} else {
		// 记录当前复制状态用于调试
		a.logger.Debug(fmt.Sprintf("current replication: master=%s:%d, IO=%v, SQL=%v, expected leader=%s:%d",
			replStatus.MasterHost, replStatus.MasterPort,
			replStatus.SlaveIORunning, replStatus.SlaveSQLRunning,
			leaderHost, leaderPort))

		if replStatus.MasterHost != leaderHost {
			// 复制指向错误的 master
			needReconfigure = true
			reason = fmt.Sprintf("replication points to %s but leader is %s", replStatus.MasterHost, leaderHost)
		} else if !replStatus.SlaveIORunning || !replStatus.SlaveSQLRunning {
			// 复制没有正常运行，可能是旧 master 宕机了
			// 即使 MasterHost 相同，如果复制线程不运行，也需要重新配置
			needReconfigure = true
			reason = fmt.Sprintf("replication not running properly: IO=%v, SQL=%v, LastError=%s",
				replStatus.SlaveIORunning, replStatus.SlaveSQLRunning, replStatus.LastError)
		}
	}

	// 检查 leader 是否变更
	a.mu.RLock()
	lastLeader := a.lastLeaderHost
	a.mu.RUnlock()

	if lastLeader != "" && lastLeader != leaderHost {
		needReconfigure = true
		reason = fmt.Sprintf("leader changed from %s to %s", lastLeader, leaderHost)
		a.logger.Info(fmt.Sprintf("detected leader change: %s -> %s", lastLeader, leaderHost))
	}

	if needReconfigure {
		a.logger.Info(fmt.Sprintf("need to reconfigure replication: %s", reason))
		if err := a.configureAsReplica(ctx, leaderHost, leaderPort); err != nil {
			a.logger.Error(fmt.Sprintf("failed to configure as replica of %s:%d: %v", leaderHost, leaderPort, err))

			// 即使配置失败，也要确保节点是只读的，防止脑裂
			if err := a.mysql.SetReadOnly(true); err != nil {
				a.logger.Error(fmt.Sprintf("failed to set read-only after replication config failure: %v", err))
			}

			// 更新状态为 replica，即使复制配置失败
			a.mu.Lock()
			a.state.Role = RoleReplica
			a.state.ReplicationInfo = &ReplicationInfo{
				MasterHost: leaderHost,
				MasterPort: leaderPort,
			}
			a.mu.Unlock()
			return
		}
		// 重新获取复制状态，因为我们刚刚重新配置了
		replStatus, _ = a.mysql.GetReplicationStatus()
	}

	// 更新缓存的 leader
	a.mu.Lock()
	a.lastLeaderHost = leaderHost
	a.state.Role = RoleReplica
	if replStatus != nil {
		a.state.ReplicationInfo = &ReplicationInfo{
			MasterHost:      replStatus.MasterHost,
			MasterPort:      replStatus.MasterPort,
			SlaveIORunning:  replStatus.SlaveIORunning,
			SlaveSQLRunning: replStatus.SlaveSQLRunning,
		}
	} else {
		// 如果刚配置完复制但获取状态失败，至少记录目标 leader
		a.state.ReplicationInfo = &ReplicationInfo{
			MasterHost: leaderHost,
			MasterPort: leaderPort,
		}
	}
	a.mu.Unlock()

	// 确保是 read-only
	readOnly, _ := a.mysql.IsReadOnly()
	if !readOnly {
		a.logger.Info("replica is not read-only, setting read-only")
		a.mysql.SetReadOnly(true)
	}
}

// updateState updates the node state (called for health checks)
func (a *Agent) updateState(ctx context.Context) {
	// Get GTID
	gtid, err := a.mysql.GetGTIDExecuted()
	if err == nil {
		a.mu.Lock()
		a.state.GTIDExecuted = gtid
		a.mu.Unlock()
	}

	// Get replication lag
	lag, err := a.mysql.GetReplicationLag()
	if err == nil {
		a.mu.Lock()
		a.state.ReplicationLag = lag
		a.mu.Unlock()
	}
}

// RequestSwitchover requests this node to become the leader
// This is called via the API when a manual switchover is requested
func (a *Agent) RequestSwitchover(reason string) error {
	a.logger.Info(fmt.Sprintf("switchover requested: %s", reason))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 检查 MySQL 是否健康
	if err := a.mysql.Ping(); err != nil {
		return fmt.Errorf("MySQL is not healthy: %w", err)
	}

	// 检查是否已经是 leader
	if a.lockManager.IsLeader() {
		a.logger.Info("already the leader, no switchover needed")
		return nil
	}

	// 获取当前 leader 信息
	_, currentLeaderHost, _, err := a.getLeaderInfo(ctx)
	if err != nil {
		a.logger.Warn(fmt.Sprintf("failed to get current leader info: %v", err))
	} else {
		a.logger.Info(fmt.Sprintf("current leader is %s, requesting switchover", currentLeaderHost))
	}

	// 尝试获取 etcd 锁
	// 首先强制释放当前锁（如果有的话），然后获取新锁
	// 这是手动切换，所以我们需要强制获取锁
	a.logger.Info("attempting to acquire leadership lock for switchover")

	// 尝试多次获取锁
	maxRetries := 10
	var lastErr error
	for i := range maxRetries {
		acquired, err := a.lockManager.TryAcquire(ctx)
		if err != nil {
			lastErr = err
			a.logger.Warn(fmt.Sprintf("failed to acquire lock (attempt %d/%d): %v", i+1, maxRetries, err))
			time.Sleep(time.Second)
			continue
		}

		if acquired {
			a.logger.Info("successfully acquired leadership lock")
			// 执行提升
			if err := a.Promote(); err != nil {
				a.logger.Error(fmt.Sprintf("failed to promote: %v, releasing lock", err))
				a.lockManager.Release(ctx)
				return fmt.Errorf("failed to promote to leader: %w", err)
			}

			// 启动锁续约
			a.lockManager.StartRenewal(ctx)
			a.logger.Info(fmt.Sprintf("switchover completed successfully, I am now the leader (reason: %s)", reason))
			return nil
		}

		// 锁被其他节点持有，等待一下再试
		a.logger.Debug(fmt.Sprintf("lock held by another node, waiting... (attempt %d/%d)", i+1, maxRetries))
		time.Sleep(time.Second)
	}

	if lastErr != nil {
		return fmt.Errorf("failed to acquire leadership lock after %d attempts: %w", maxRetries, lastErr)
	}
	return fmt.Errorf("failed to acquire leadership lock after %d attempts: lock held by another node", maxRetries)
}

// repairMySQLEnvironment checks and repairs MySQL runtime environment
// This handles issues like missing directories after system reboot
func (a *Agent) repairMySQLEnvironment() error {
	a.logger.Info("checking MySQL environment...")

	// 需要检查和修复的目录
	dirs := []struct {
		path  string
		owner string
		mode  os.FileMode
	}{
		{"/var/run/mysqld", "mysql", 0755},
		{"/var/log/mysql", "mysql", 0755},
	}

	// 从配置中获取数据目录（如果有的话）
	// 默认数据目录
	dataDir := "/var/lib/mysql"
	dirs = append(dirs, struct {
		path  string
		owner string
		mode  os.FileMode
	}{dataDir, "mysql", 0750})

	for _, dir := range dirs {
		// 检查目录是否存在
		if _, err := os.Stat(dir.path); os.IsNotExist(err) {
			a.logger.Info(fmt.Sprintf("creating missing directory: %s", dir.path))
			// 使用 sudo 创建目录
			cmd := exec.Command("sudo", "mkdir", "-p", dir.path)
			if output, err := cmd.CombinedOutput(); err != nil {
				a.logger.Warn(fmt.Sprintf("failed to create directory %s: %v, output: %s", dir.path, err, string(output)))
				continue
			}
		}

		// 设置目录权限
		cmd := exec.Command("sudo", "chown", fmt.Sprintf("%s:%s", dir.owner, dir.owner), dir.path)
		if output, err := cmd.CombinedOutput(); err != nil {
			a.logger.Warn(fmt.Sprintf("failed to chown %s: %v, output: %s", dir.path, err, string(output)))
		}

		cmd = exec.Command("sudo", "chmod", fmt.Sprintf("%o", dir.mode), dir.path)
		if output, err := cmd.CombinedOutput(); err != nil {
			a.logger.Warn(fmt.Sprintf("failed to chmod %s: %v, output: %s", dir.path, err, string(output)))
		}
	}

	// 检查 MySQL 服务状态
	cmd := exec.Command("systemctl", "is-active", "mysqld")
	output, err := cmd.CombinedOutput()
	mysqlActive := err == nil && string(output) == "active\n"

	if !mysqlActive {
		a.logger.Info("MySQL service is not active, attempting to start...")

		// 先尝试重启 MySQL 服务
		cmd = exec.Command("sudo", "systemctl", "restart", "mysqld")
		if output, err := cmd.CombinedOutput(); err != nil {
			a.logger.Warn(fmt.Sprintf("failed to restart mysqld: %v, output: %s", err, string(output)))

			// 如果 mysqld 服务不存在，尝试 mysql 服务
			cmd = exec.Command("sudo", "systemctl", "restart", "mysql")
			if output, err := cmd.CombinedOutput(); err != nil {
				a.logger.Warn(fmt.Sprintf("failed to restart mysql: %v, output: %s", err, string(output)))
				return fmt.Errorf("failed to start MySQL service")
			}
		}

		// 等待 MySQL 启动
		a.logger.Info("waiting for MySQL to start...")
		time.Sleep(5 * time.Second)
	}

	a.logger.Info("MySQL environment check completed")
	return nil
}
