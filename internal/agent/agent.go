package agent

import (
	"context"
	"fmt"
	"net"
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

// DefaultVersion is the fallback version if not configured
const DefaultVersion = "1.0.0"

// Agent is the core HA agent running on each MySQL node
type Agent struct {
	config          *config.Config
	dcs             dcs.DCS
	mysql           *mysql.Manager
	lockManager     *election.LockManager
	logger          *log.Logger
	state           *NodeState
	stopCh          chan struct{}
	mu              sync.RWMutex
	mysqlConnected  bool
	dcsConnected    bool
	lastLeaderHost  string    // 缓存上次的 leader host，用于检测 leader 变更
	startTime       time.Time // Agent 启动时间，用于判断是否刚启动
	wasLeader       bool      // 标记自己之前是否是 leader（通过 leader_info 判断）
	mysqlRecovering bool      // 标记是否正在进行 MySQL 自动恢复
}

// getVersion returns the agent version from config or default
func getVersion(cfg *config.Config) string {
	if cfg.Version != "" {
		return cfg.Version
	}
	return DefaultVersion
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
			NodeID:       cfg.Name,
			Hostname:     hostname,
			Role:         RoleUnknown,
			AgentVersion: getVersion(cfg),
		},
		stopCh:    make(chan struct{}),
		startTime: time.Now(),
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

	// 快速恢复检查：如果 etcd 中的 leader_info 指向自己，说明自己之前是 leader
	// 应该立即尝试获取锁，避免不必要的选举
	a.tryQuickLeaderRecovery(ctx)

	// Register with DCS
	if err := a.register(ctx); err != nil {
		a.logger.Warn(fmt.Sprintf("failed to register initially: %v, will retry", err))
	}

	// Start main loop
	go a.runLoop(ctx)

	a.logger.Info("HA agent started")
	return nil
}

// tryQuickLeaderRecovery 尝试快速恢复 leader 身份
// 如果 etcd 中的 leader_info 指向自己，说明自己之前是 leader，应该立即尝试获取锁
// 这个函数是防止 Agent 重启触发不必要选举的关键
func (a *Agent) tryQuickLeaderRecovery(ctx context.Context) {
	myHost := a.getMyHost()

	// 检查 etcd 中的 leader_info
	_, leaderHost, _, err := a.getLeaderInfo(ctx)
	if err != nil {
		a.logger.Debug(fmt.Sprintf("no leader info in etcd: %v", err))
		return
	}

	if leaderHost != myHost {
		a.logger.Debug(fmt.Sprintf("leader_info points to %s, not me (%s)", leaderHost, myHost))
		return
	}

	// leader_info 指向自己，标记自己之前是 leader
	a.mu.Lock()
	a.wasLeader = true
	a.mu.Unlock()

	a.logger.Info(fmt.Sprintf("leader_info points to me (%s), attempting quick leader recovery", myHost))

	// 等待 MySQL 连接（最多 30 秒，给足够时间让 MySQL 启动）
	maxWait := 60
	for i := 0; i < maxWait; i++ {
		a.mu.RLock()
		connected := a.mysqlConnected
		a.mu.RUnlock()
		if connected {
			break
		}
		if i%10 == 0 {
			a.logger.Info(fmt.Sprintf("waiting for MySQL connection for leader recovery... (%d/%d)", i, maxWait))
		}
		time.Sleep(500 * time.Millisecond)
	}

	// 检查 MySQL 是否健康
	if err := a.mysql.Ping(); err != nil {
		a.logger.Warn(fmt.Sprintf("MySQL not healthy, cannot recover as leader: %v", err))
		return
	}

	// 检查 MySQL 是否是 read-write 模式（之前是 master）
	readOnly, err := a.mysql.IsReadOnly()
	if err != nil {
		a.logger.Warn(fmt.Sprintf("failed to check read-only status: %v", err))
	} else if readOnly {
		a.logger.Info("MySQL is read-only, was likely a replica, not recovering as leader")
		a.mu.Lock()
		a.wasLeader = false
		a.mu.Unlock()
		return
	}

	// 多次尝试获取锁（旧锁可能还没过期，需要等待）
	maxRetries := 10
	for i := 0; i < maxRetries; i++ {
		acquired, err := a.lockManager.TryAcquire(ctx)
		if err != nil {
			a.logger.Warn(fmt.Sprintf("failed to acquire lock during quick recovery (attempt %d/%d): %v", i+1, maxRetries, err))
			time.Sleep(time.Second)
			continue
		}

		if acquired {
			a.logger.Info("quick leader recovery successful, acquired leadership lock")
			// 确保 MySQL 是 read-write
			if err := a.mysql.SetReadOnly(false); err != nil {
				a.logger.Error(fmt.Sprintf("failed to set read-write: %v", err))
			}
			// 更新状态
			a.mu.Lock()
			a.state.Role = RoleLeader
			a.state.ReplicationInfo = nil
			a.mu.Unlock()
			// 启动锁续约
			a.lockManager.StartRenewal(ctx)
			// 更新 leader_info（刷新时间戳）
			a.storeLeaderInfo(ctx)
			return
		}

		// 锁被其他节点持有，等待一下再试
		a.logger.Debug(fmt.Sprintf("lock held by another node during recovery, waiting... (attempt %d/%d)", i+1, maxRetries))
		time.Sleep(time.Second)
	}

	a.logger.Info("quick leader recovery failed after retries, will participate in normal election")
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
// 当 MySQL 连接丢失时，会自动尝试修复 MySQL 服务
func (a *Agent) connectMySQLWithRetry(ctx context.Context) {
	// 标记正在恢复
	a.mu.Lock()
	if a.mysqlRecovering {
		a.mu.Unlock()
		a.logger.Debug("MySQL recovery already in progress, skipping")
		return
	}
	a.mysqlRecovering = true
	a.mu.Unlock()

	// 确保退出时清除标志
	defer func() {
		a.mu.Lock()
		a.mysqlRecovering = false
		a.mu.Unlock()
	}()

	maxRetries := 0 // unlimited retries
	baseDelay := 5 * time.Second
	maxDelay := 60 * time.Second
	attempt := 0
	lastRepairAttempt := time.Time{}
	repairInterval := 30 * time.Second // 每 30 秒最多尝试修复一次

	for {
		select {
		case <-ctx.Done():
			return
		case <-a.stopCh:
			return
		default:
		}

		// 先尝试连接 MySQL
		if err := a.mysql.Connect(); err != nil {
			attempt++

			// 在连接失败后尝试修复 MySQL 环境
			// 第一次失败后立即尝试修复，之后每 30 秒尝试一次
			shouldRepair := attempt == 1 || time.Since(lastRepairAttempt) >= repairInterval
			if shouldRepair {
				a.logger.Info(fmt.Sprintf("========== MySQL Auto-Repair (attempt %d) ==========", attempt))
				a.logger.Info("attempting to repair MySQL environment and restart service...")
				lastRepairAttempt = time.Now()
				if repairErr := a.repairMySQLEnvironment(); repairErr != nil {
					a.logger.Warn(fmt.Sprintf("failed to repair MySQL environment: %v", repairErr))
				} else {
					a.logger.Info("MySQL environment repair completed, waiting for service to start...")
					// 修复后等待一下让 MySQL 启动
					time.Sleep(5 * time.Second)
					// 修复后立即重试连接
					continue
				}
			}

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
		a.logger.Info("========== MySQL connection restored ==========")
		return
	}
}

// Stop stops the agent
// 优雅关闭时不释放 etcd 锁，避免触发不必要的选举
// 锁会在 TTL 过期后自动释放，或者 Agent 重启后继续持有
func (a *Agent) Stop() error {
	a.logger.Info("stopping HA agent (graceful shutdown, keeping etcd lock)")

	close(a.stopCh)

	if a.lockManager != nil {
		// 只停止续约，不释放锁
		// 这样如果是升级重启，Agent 可以在 TTL 内重新启动并继续持有锁
		// 如果是真正的故障，锁会在 TTL 后自动过期，其他节点可以接管
		a.lockManager.StopRenewal()
		// 注意：不调用 Release()，让锁自然过期或重启后继续持有
		a.logger.Info("stopped lock renewal, lock will expire naturally if agent doesn't restart")
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
// 在返回状态前，主动检查 MySQL 连接状态，确保返回最新的健康状态
func (a *Agent) GetState() any {
	// 主动检查 MySQL 连接状态
	if err := a.mysql.Ping(); err != nil {
		a.mu.Lock()
		a.mysqlConnected = false
		a.state.IsHealthy = false
		a.mu.Unlock()
	} else {
		a.mu.Lock()
		a.mysqlConnected = true
		a.state.IsHealthy = true
		a.mu.Unlock()
	}

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

// isLeaderMySQLReachable 检查 leader 的 MySQL 是否可达
// 用于判断原 leader 是否还活着，避免不必要的选举
func (a *Agent) isLeaderMySQLReachable(host string, port int) bool {
	// 使用短超时检查 MySQL 端口是否可达
	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// isEligibleForLeadership 检查自己是否有资格成为 leader
// 基于 GTID 比较：只有 GTID 最新（或并列最新）的节点才有资格
// 返回 (是否有资格, 原因)
func (a *Agent) isEligibleForLeadership(ctx context.Context) (bool, string) {
	a.logger.Info("========== Replication position based election check ==========")

	// 获取自己的复制位置（优先 GTID，其次 binlog position）
	myPosition, myTxCount, err := a.mysql.GetReplicationPosition()
	if err != nil {
		a.logger.Warn(fmt.Sprintf("[ELECTION] failed to get my replication position: %v", err))
		// 如果无法获取位置，不参与选举
		return false, "cannot get my replication position"
	}

	a.logger.Info(fmt.Sprintf("[ELECTION] My position: %s (value: %d)", truncateGTID(myPosition), myTxCount))

	if myTxCount == 0 {
		// 位置为 0 可能是新初始化的节点，允许参与选举
		a.logger.Info("[ELECTION] My position value is 0, allowing election participation (new node)")
		return true, "position is 0 (new node)"
	}

	// 获取所有成员的位置信息
	members, err := a.dcs.GetMembers(ctx)
	if err != nil {
		a.logger.Warn(fmt.Sprintf("[ELECTION] failed to get members from DCS: %v", err))
		// 如果无法获取成员信息，允许参与选举（可能是第一个节点）
		return true, "cannot get members from DCS"
	}

	a.logger.Info(fmt.Sprintf("[ELECTION] Found %d members in cluster", len(members)))

	// 找出位置最新的节点
	var maxPosition int64
	var maxPositionNodeID string
	staleThreshold := int64(30) // 30秒内更新过的成员才参与比较

	now := time.Now().Unix()
	for _, member := range members {
		age := now - member.UpdatedAt
		// 跳过不健康或过期的成员
		if !member.IsHealthy || age > staleThreshold {
			a.logger.Info(fmt.Sprintf("[ELECTION] Skipping member %s: healthy=%v, age=%ds (stale threshold: %ds)",
				member.NodeID, member.IsHealthy, age, staleThreshold))
			continue
		}

		if member.GTIDExecuted == "" {
			a.logger.Info(fmt.Sprintf("[ELECTION] Member %s has empty position, skipping", member.NodeID))
			continue
		}

		// 解析成员的位置
		memberTxCount := parseMemberPosition(member.GTIDExecuted)
		a.logger.Info(fmt.Sprintf("[ELECTION] Member %s: position=%s (value: %d)",
			member.NodeID, truncateGTID(member.GTIDExecuted), memberTxCount))

		if memberTxCount > maxPosition {
			maxPosition = memberTxCount
			maxPositionNodeID = member.NodeID
		}
	}

	// 如果没有找到有效的位置，允许参与选举
	if maxPosition == 0 {
		a.logger.Info("[ELECTION] No valid position found in cluster, allowing election participation")
		return true, "no valid position found in cluster"
	}

	a.logger.Info(fmt.Sprintf("[ELECTION] Max position holder: %s with value %d", maxPositionNodeID, maxPosition))

	// 比较自己的位置和最大位置
	if myTxCount >= maxPosition {
		// 自己的位置 >= 最大位置，有资格
		a.logger.Info(fmt.Sprintf("[ELECTION] ✓ ELIGIBLE: my position (%d) >= max position (%d from %s)",
			myTxCount, maxPosition, maxPositionNodeID))
		a.logger.Info("========== Election check passed, can participate in election ==========")
		return true, "position is up to date"
	}

	// 自己的位置落后，没有资格
	a.logger.Info(fmt.Sprintf("[ELECTION] ✗ NOT ELIGIBLE: my position (%d) < max position (%d from %s)",
		myTxCount, maxPosition, maxPositionNodeID))
	a.logger.Info("========== Election check failed, waiting for node with more data ==========")
	return false, fmt.Sprintf("position behind node %s", maxPositionNodeID)
}

// parseMemberPosition 解析成员的位置值
func parseMemberPosition(position string) int64 {
	if position == "" {
		return 0
	}

	// 如果是 GTID 格式
	if len(position) > 5 && position[:5] == "GTID:" {
		return countGTIDTransactions(position[5:])
	}

	// 如果是 BINLOG 格式: BINLOG:file:position
	if len(position) > 7 && position[:7] == "BINLOG:" {
		parts := splitString(position[7:], ':')
		if len(parts) >= 2 {
			var pos int64
			fmt.Sscanf(parts[1], "%d", &pos)
			return pos
		}
	}

	// 尝试作为纯 GTID 解析
	return countGTIDTransactions(position)
}

// compareGTID 比较两个 GTID 集合
// 返回: >0 如果 a > b, <0 如果 a < b, 0 如果相等
// 简化实现：比较事务数量（GTID 中的最大 interval）
func compareGTID(a, b string) int {
	if a == "" && b == "" {
		return 0
	}
	if a == "" {
		return -1
	}
	if b == "" {
		return 1
	}

	// 计算 GTID 中的事务总数
	countA := countGTIDTransactions(a)
	countB := countGTIDTransactions(b)

	if countA > countB {
		return 1
	} else if countA < countB {
		return -1
	}
	return 0
}

// countGTIDTransactions 计算 GTID 集合中的事务总数
// GTID 格式: uuid:1-100,uuid:1-50 表示 100+50=150 个事务
func countGTIDTransactions(gtid string) int64 {
	if gtid == "" {
		return 0
	}

	var total int64
	// 按逗号分割多个 UUID 的 GTID
	parts := splitString(gtid, ',')
	for _, part := range parts {
		part = trimSpace(part)
		if part == "" {
			continue
		}
		// 格式: uuid:intervals 或 uuid:n-m
		colonIdx := lastIndexOf(part, ':')
		if colonIdx < 0 {
			continue
		}
		intervals := part[colonIdx+1:]
		// intervals 可能是 "1-100" 或 "1-100:200-300"
		rangeParts := splitString(intervals, ':')
		for _, rp := range rangeParts {
			rp = trimSpace(rp)
			dashIdx := indexOf(rp, '-')
			if dashIdx > 0 {
				// 范围格式: start-end
				var start, end int64
				fmt.Sscanf(rp[:dashIdx], "%d", &start)
				fmt.Sscanf(rp[dashIdx+1:], "%d", &end)
				total += end - start + 1
			} else {
				// 单个数字
				var n int64
				fmt.Sscanf(rp, "%d", &n)
				if n > 0 {
					total++
				}
			}
		}
	}
	return total
}

// truncateGTID 截断 GTID 用于日志显示
func truncateGTID(gtid string) string {
	if len(gtid) <= 50 {
		return gtid
	}
	return gtid[:47] + "..."
}

// splitString 分割字符串（避免使用 strings 包）
func splitString(s string, sep byte) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	result = append(result, s[start:])
	return result
}

// trimSpace 去除首尾空格
func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

// indexOf 查找字符位置
func indexOf(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// lastIndexOf 查找字符最后出现的位置
func lastIndexOf(s string, c byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == c {
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
	// 获取当前复制位置（优先 GTID，其次 binlog position）
	position, _, _ := a.mysql.GetReplicationPosition()

	a.mu.RLock()
	member := &dcs.Member{
		NodeID:       a.state.NodeID,
		Hostname:     a.state.Hostname,
		APIAddr:      fmt.Sprintf("%s:%d", a.config.API.Listen, a.config.API.Port),
		Role:         string(a.state.Role),
		GTIDExecuted: position, // 存储位置信息（GTID 或 binlog position）
		IsHealthy:    a.state.IsHealthy,
		UpdatedAt:    time.Now().Unix(),
	}
	a.mu.RUnlock()

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
// 5. MySQL 服务故障时，自动尝试修复和重启
func (a *Agent) tick(ctx context.Context) {
	// 更新 MySQL 连接状态
	if err := a.mysql.Ping(); err != nil {
		a.mu.Lock()
		wasConnected := a.mysqlConnected
		isRecovering := a.mysqlRecovering
		a.mysqlConnected = false
		a.state.IsHealthy = false
		a.mu.Unlock()

		if wasConnected {
			a.logger.Warn(fmt.Sprintf("========== MySQL connection lost =========="))
			a.logger.Warn(fmt.Sprintf("MySQL connection lost: %v", err))
		}

		// 如果没有正在进行恢复，启动恢复
		if !isRecovering {
			a.logger.Info("starting automatic MySQL recovery in background...")
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
// 关键逻辑：防止 Agent 重启时触发不必要的选举
// 1. 如果自己之前是 leader（wasLeader=true），应该优先恢复
// 2. 如果自己不是原 leader，应该等待原 leader 恢复
// 3. 只有在原 leader 确实不可用时，才参与选举
func (a *Agent) handleAsNonLeader(ctx context.Context, leaderNodeID, leaderHost string, leaderPort int, leaderErr error, myHost string) {
	// 计算启动后经过的时间
	a.mu.RLock()
	timeSinceStart := time.Since(a.startTime)
	wasLeader := a.wasLeader
	a.mu.RUnlock()

	// 启动保护期：刚启动的 30 秒内，非原 leader 不应该尝试获取锁
	// 这给原 leader 足够的时间来恢复
	startupGracePeriod := 30 * time.Second

	// 如果 leader_info 存在且指向其他节点，说明有一个已知的 leader
	if leaderErr == nil && leaderHost != "" && leaderHost != myHost {
		a.logger.Debug(fmt.Sprintf("leader_info points to %s, checking if we should wait for leader recovery", leaderHost))

		// 如果我们在启动保护期内，且不是原 leader，等待原 leader 恢复
		if timeSinceStart < startupGracePeriod && !wasLeader {
			a.logger.Info(fmt.Sprintf("in startup grace period (%.0fs remaining), waiting for leader %s to recover",
				(startupGracePeriod - timeSinceStart).Seconds(), leaderHost))
			// 不尝试获取锁，直接配置为 replica
			a.ensureReplicaOf(ctx, leaderNodeID, leaderHost, leaderPort)
			return
		}

		// 检查原 leader 的 MySQL 是否可达
		// 如果可达，说明原 leader 可能正在恢复，继续等待
		if a.isLeaderMySQLReachable(leaderHost, leaderPort) {
			a.logger.Debug(fmt.Sprintf("leader %s MySQL is reachable, waiting for leader to recover", leaderHost))
			a.ensureReplicaOf(ctx, leaderNodeID, leaderHost, leaderPort)
			return
		}

		a.logger.Info(fmt.Sprintf("leader %s MySQL is not reachable, may attempt failover", leaderHost))
	}

	// 如果 leader 是我自己（但我没有锁），这通常发生在 agent 重启后
	// 在这种情况下，我们应该积极尝试恢复 leader 身份
	if leaderErr == nil && leaderHost == myHost {
		a.logger.Info("leader_info points to me but I don't hold the lock, attempting to recover leadership")

		// 检查 MySQL 是否健康且是 read-write
		if err := a.mysql.Ping(); err != nil {
			a.logger.Warn(fmt.Sprintf("MySQL not healthy, cannot recover as leader: %v", err))
			return
		}

		readOnly, _ := a.mysql.IsReadOnly()
		if readOnly {
			a.logger.Info("MySQL is read-only, not recovering as leader")
			return
		}

		// 尝试获取锁
		acquired, err := a.lockManager.TryAcquire(ctx)
		if err != nil {
			a.logger.Debug(fmt.Sprintf("failed to acquire lock: %v", err))
			return
		}

		if acquired {
			a.logger.Info("recovered leadership lock")
			a.performFailover(ctx)
			return
		}

		// 锁被其他节点持有，等待
		a.logger.Debug("lock held by another node, waiting for recovery")
		return
	}

	// 如果没有 leader 信息，或者原 leader 不可达，尝试获取锁
	// 但如果在启动保护期内且不是原 leader，仍然等待
	if timeSinceStart < startupGracePeriod && !wasLeader && leaderErr == nil {
		a.logger.Info(fmt.Sprintf("in startup grace period, not attempting lock acquisition"))
		// 至少确保是只读模式
		if err := a.mysql.SetReadOnly(true); err != nil {
			a.logger.Error(fmt.Sprintf("failed to set read-only: %v", err))
		}
		a.mu.Lock()
		a.state.Role = RoleReplica
		a.mu.Unlock()
		return
	}

	// 在尝试获取锁之前，检查自己是否有资格成为 leader（基于 GTID）
	// 只有 GTID 最新的节点才能参与选举，避免数据丢失
	eligible, reason := a.isEligibleForLeadership(ctx)
	if !eligible {
		a.logger.Info(fmt.Sprintf("not eligible for leadership: %s, waiting for eligible node", reason))
		// 至少确保是只读模式
		if err := a.mysql.SetReadOnly(true); err != nil {
			a.logger.Error(fmt.Sprintf("failed to set read-only: %v", err))
		}
		a.mu.Lock()
		a.state.Role = RoleReplica
		a.mu.Unlock()
		return
	}

	a.logger.Info(fmt.Sprintf("eligible for leadership: %s, attempting to acquire lock", reason))

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
	a.logger.Info("attempting to acquire leadership lock for switchover")

	// 尝试多次获取锁（等待当前 leader 释放）
	maxRetries := 30 // 增加重试次数，等待 demote 完成
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

// RequestDemote requests this node to release the leader lock
// This is called via the API when a manual switchover is requested on another node
func (a *Agent) RequestDemote(reason string) error {
	a.logger.Info(fmt.Sprintf("demote requested: %s", reason))

	// 检查是否是 leader
	if !a.lockManager.IsLeader() {
		a.logger.Info("not the leader, no demote needed")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 停止锁续约
	a.lockManager.StopRenewal()

	// 设置 MySQL 为只读
	if err := a.mysql.SetReadOnly(true); err != nil {
		a.logger.Warn(fmt.Sprintf("failed to set read-only: %v", err))
	}

	// 释放锁
	if err := a.lockManager.Release(ctx); err != nil {
		a.logger.Error(fmt.Sprintf("failed to release lock: %v", err))
		return fmt.Errorf("failed to release lock: %w", err)
	}

	// 更新状态
	a.mu.Lock()
	a.state.Role = RoleReplica
	a.mu.Unlock()

	a.logger.Info(fmt.Sprintf("demote completed, released leadership (reason: %s)", reason))
	return nil
}

// RestartMySQL restarts the MySQL service on this node
// This is called via the API when a manual restart is requested
func (a *Agent) RestartMySQL() error {
	a.logger.Info("MySQL restart requested via API")

	// 先尝试 mysqld 服务
	cmd := exec.Command("sudo", "systemctl", "restart", "mysqld")
	if output, err := cmd.CombinedOutput(); err != nil {
		a.logger.Warn(fmt.Sprintf("failed to restart mysqld: %v, output: %s", err, string(output)))

		// 如果 mysqld 服务不存在，尝试 mysql 服务
		cmd = exec.Command("sudo", "systemctl", "restart", "mysql")
		if output, err := cmd.CombinedOutput(); err != nil {
			a.logger.Error(fmt.Sprintf("failed to restart mysql: %v, output: %s", err, string(output)))
			return fmt.Errorf("failed to restart MySQL service: %w", err)
		}
	}

	a.logger.Info("MySQL restart command executed successfully")
	return nil
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
