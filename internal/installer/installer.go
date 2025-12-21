package installer

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Installer handles software installation on remote hosts
type Installer struct {
	taskChan chan *InstallTask
	tasks    map[string]*InstallTask
}

// NewInstaller creates a new Installer
func NewInstaller() *Installer {
	return &Installer{
		taskChan: make(chan *InstallTask, 100),
		tasks:    make(map[string]*InstallTask),
	}
}

// CheckEtcdInstalled checks if etcd is already installed
func (i *Installer) CheckEtcdInstalled(host *Host, installPath string) (bool, error) {
	client, err := NewSSHClient(host)
	if err != nil {
		return false, err
	}
	defer client.Close()

	// 只检查我们安装的路径，不检查系统路径
	etcdPath := fmt.Sprintf("%s/etcd/etcd", installPath)
	checkCmd := fmt.Sprintf("test -f %s && %s --version 2>/dev/null", etcdPath, etcdPath)
	output, err := client.Run(checkCmd)
	if err == nil && strings.Contains(output, "etcd Version") {
		return true, nil
	}

	return false, nil
}

// CheckMySQLInstalled checks if MySQL is already installed and running
func (i *Installer) CheckMySQLInstalled(host *Host, installPath string) (bool, error) {
	client, err := NewSSHClient(host)
	if err != nil {
		return false, err
	}
	defer client.Close()

	// 检查 MySQL 二进制文件是否存在
	mysqldPath := fmt.Sprintf("%s/mysql/bin/mysqld", installPath)
	checkCmd := fmt.Sprintf("test -f %s && %s --version 2>/dev/null", mysqldPath, mysqldPath)
	output, err := client.Run(checkCmd)
	if err == nil && strings.Contains(output, "mysqld") {
		return true, nil
	}
	return false, nil
}

// InstallEtcd installs etcd on a remote host
func (i *Installer) InstallEtcd(host *Host, version, installPath string, etcdNodes []Host, progress func(int, string)) error {
	client, err := NewSSHClient(host)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Close()

	// 检查是否已安装
	progress(5, "Checking if etcd is already installed...")
	installed, _ := i.CheckEtcdInstalled(host, installPath)
	if installed {
		progress(100, "etcd already installed, skipping...")
		return nil
	}

	// Find version
	var downloadURL string
	for _, v := range EtcdVersions {
		if v.Version == version {
			downloadURL = v.DownloadURL
			break
		}
	}
	if downloadURL == "" {
		return fmt.Errorf("etcd version %s not found", version)
	}

	progress(10, "Creating directories...")
	if _, err := client.RunWithSudo(fmt.Sprintf("mkdir -p %s/etcd", installPath)); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	progress(20, "Downloading etcd...")
	// 清理旧文件并下载
	cleanCmd := "rm -f /tmp/etcd.tar.gz /tmp/etcd-v*-linux-amd64 2>/dev/null || true"
	client.Run(cleanCmd)

	downloadCmd := fmt.Sprintf("wget -q --timeout=300 '%s' -O /tmp/etcd.tar.gz", downloadURL)
	if _, err := client.Run(downloadCmd); err != nil {
		return fmt.Errorf("failed to download etcd: %w", err)
	}

	// 验证下载文件
	checkCmd := "test -f /tmp/etcd.tar.gz && ls -la /tmp/etcd.tar.gz"
	if _, err := client.Run(checkCmd); err != nil {
		return fmt.Errorf("etcd download file not found: %w", err)
	}

	progress(50, "Extracting etcd...")
	extractCmd := fmt.Sprintf("tar -xzf /tmp/etcd.tar.gz -C /tmp && sudo mv /tmp/etcd-v%s-linux-amd64/* %s/etcd/", version, installPath)
	if _, err := client.RunWithSudo(extractCmd); err != nil {
		return fmt.Errorf("failed to extract etcd: %w", err)
	}

	progress(70, "Creating symlinks...")
	linkCmd := fmt.Sprintf("sudo ln -sf %s/etcd/etcd /usr/local/bin/etcd && sudo ln -sf %s/etcd/etcdctl /usr/local/bin/etcdctl", installPath, installPath)
	if _, err := client.RunWithSudo(linkCmd); err != nil {
		return fmt.Errorf("failed to create symlinks: %w", err)
	}

	progress(80, "Creating systemd service...")
	serviceContent := generateEtcdService(host.Name, host.IP, installPath, etcdNodes)
	// 使用 cat 和 heredoc 写入文件，避免特殊字符问题
	serviceCmd := fmt.Sprintf("cat > /etc/systemd/system/etcd.service << 'ETCD_SERVICE_EOF'\n%s\nETCD_SERVICE_EOF", serviceContent)
	if _, err := client.RunWithSudo(serviceCmd); err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}

	progress(90, "Starting etcd service...")
	if _, err := client.RunWithSudo("systemctl daemon-reload && systemctl enable etcd && systemctl start etcd"); err != nil {
		return fmt.Errorf("failed to start etcd: %w", err)
	}

	progress(100, "etcd installation completed")
	return nil
}

// InstallMySQL installs MySQL on a remote host
func (i *Installer) InstallMySQL(host *Host, version, installPath, dataPath string, settings *ClusterSettings, progress func(int, string)) error {
	client, err := NewSSHClient(host)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Close()

	// 检查是否已安装
	progress(3, "Checking if MySQL is already installed...")
	installed, _ := i.CheckMySQLInstalled(host, installPath)
	if installed {
		progress(100, "MySQL already installed, skipping...")
		return nil
	}

	// Find version
	var downloadURL string
	allVersions := append(MySQL57Versions, MySQL80Versions...)
	for _, v := range allVersions {
		if v.Version == version {
			downloadURL = v.DownloadURL
			break
		}
	}
	if downloadURL == "" {
		return fmt.Errorf("MySQL version %s not found", version)
	}

	progress(5, "Installing dependencies...")
	if _, err := client.RunWithSudo("apt-get update && apt-get install -y libaio1 libnuma1 libncurses5 || yum install -y libaio numactl ncurses-compat-libs"); err != nil {
		// Ignore error, might be different package manager
	}

	progress(10, "Creating MySQL user and directories...")
	cmds := []string{
		"groupadd -r mysql || true",
		"useradd -r -g mysql -s /bin/false mysql || true",
		fmt.Sprintf("mkdir -p %s/mysql %s /var/run/mysqld /var/log/mysql", installPath, dataPath),
		fmt.Sprintf("chown -R mysql:mysql %s /var/run/mysqld /var/log/mysql", dataPath),
		"chmod 755 /var/run/mysqld",
	}
	for _, cmd := range cmds {
		client.RunWithSudo(cmd)
	}

	progress(20, "Downloading MySQL...")
	ext := "tar.gz"
	if strings.Contains(downloadURL, ".tar.xz") {
		ext = "tar.xz"
	}
	// 清理旧文件
	cleanCmd := fmt.Sprintf("rm -f /tmp/mysql.%s /tmp/mysql-*-linux-* 2>/dev/null || true", ext)
	client.Run(cleanCmd)

	downloadCmd := fmt.Sprintf("wget -q --timeout=600 '%s' -O /tmp/mysql.%s", downloadURL, ext)
	if _, err := client.Run(downloadCmd); err != nil {
		return fmt.Errorf("failed to download MySQL: %w", err)
	}

	// 验证下载文件
	checkCmd := fmt.Sprintf("test -f /tmp/mysql.%s && ls -la /tmp/mysql.%s", ext, ext)
	if _, err := client.Run(checkCmd); err != nil {
		return fmt.Errorf("MySQL download file not found: %w", err)
	}

	progress(40, "Extracting MySQL...")
	var extractCmd string
	if ext == "tar.xz" {
		extractCmd = fmt.Sprintf("tar -xJf /tmp/mysql.%s -C /tmp", ext)
	} else {
		extractCmd = fmt.Sprintf("tar -xzf /tmp/mysql.%s -C /tmp", ext)
	}
	if _, err := client.Run(extractCmd); err != nil {
		return fmt.Errorf("failed to extract MySQL: %w", err)
	}

	progress(50, "Moving MySQL files...")
	// 先找到解压后的目录名
	findCmd := "ls -d /tmp/mysql-*-linux-* 2>/dev/null | head -1"
	mysqlDir, err := client.Run(findCmd)
	if err != nil || strings.TrimSpace(mysqlDir) == "" {
		return fmt.Errorf("failed to find extracted MySQL directory")
	}
	mysqlDir = strings.TrimSpace(mysqlDir)

	// 移动文件到目标目录（注意：解压后可能有嵌套目录）
	// 先清理目标目录
	client.RunWithSudo(fmt.Sprintf("rm -rf %s/mysql/*", installPath))
	// 移动所有内容
	moveCmd := fmt.Sprintf("cp -r %s/* %s/mysql/", mysqlDir, installPath)
	if _, err := client.RunWithSudo(moveCmd); err != nil {
		return fmt.Errorf("failed to move MySQL: %w", err)
	}

	// 验证 mysqld 是否存在
	verifyCmd := fmt.Sprintf("test -f %s/mysql/bin/mysqld", installPath)
	if _, err := client.Run(verifyCmd); err != nil {
		return fmt.Errorf("mysqld binary not found after installation, directory structure may be incorrect")
	}

	progress(60, "Creating MySQL configuration...")
	serverID := generateServerID(host.IP)
	mycnf := generateMySQLConfig(host.IP, serverID, installPath, dataPath, settings, version)
	// 使用 cat 和 heredoc 写入文件，避免特殊字符问题
	mycnfCmd := fmt.Sprintf("cat > /etc/my.cnf << 'MYSQL_CNF_EOF'\n%s\nMYSQL_CNF_EOF", mycnf)
	if _, err := client.RunWithSudo(mycnfCmd); err != nil {
		return fmt.Errorf("failed to create my.cnf: %w", err)
	}

	progress(70, "Initializing MySQL...")
	initCmd := fmt.Sprintf("sudo %s/mysql/bin/mysqld --initialize-insecure --user=mysql --datadir=%s", installPath, dataPath)
	if _, err := client.RunWithSudo(initCmd); err != nil {
		return fmt.Errorf("failed to initialize MySQL: %w", err)
	}

	progress(80, "Creating systemd service...")
	serviceContent := generateMySQLService(installPath, dataPath)
	// 使用 cat 和 heredoc 写入文件，避免特殊字符问题
	serviceCmd := fmt.Sprintf("cat > /etc/systemd/system/mysqld.service << 'MYSQL_SERVICE_EOF'\n%s\nMYSQL_SERVICE_EOF", serviceContent)
	if _, err := client.RunWithSudo(serviceCmd); err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}

	progress(85, "Starting MySQL service...")
	// 先停止可能存在的 MySQL 服务
	client.RunWithSudo("systemctl stop mysqld 2>/dev/null || true")
	client.RunWithSudo("systemctl stop mysql 2>/dev/null || true")

	// 确保数据目录权限正确
	client.RunWithSudo(fmt.Sprintf("chown -R mysql:mysql %s", dataPath))
	client.RunWithSudo(fmt.Sprintf("chmod 750 %s", dataPath))

	// 重新加载并启动服务
	if _, err := client.RunWithSudo("systemctl daemon-reload"); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}

	if _, err := client.RunWithSudo("systemctl enable mysqld"); err != nil {
		return fmt.Errorf("failed to enable mysqld: %w", err)
	}

	// 启动 MySQL 服务，增加超时时间
	if _, err := client.RunWithSudo("timeout 120 systemctl start mysqld"); err != nil {
		// 如果启动失败，获取详细错误信息
		status, _ := client.RunWithSudo("systemctl status mysqld --no-pager -l")
		logs, _ := client.RunWithSudo("journalctl -u mysqld --no-pager -l -n 20")
		return fmt.Errorf("failed to start MySQL: %w\nStatus: %s\nLogs: %s", err, status, logs)
	}

	// Wait for MySQL to start and check if it's running (with retry)
	progress(87, "Waiting for MySQL to be ready...")
	var mysqlReady bool
	for retry := 0; retry < 12; retry++ {
		time.Sleep(5 * time.Second)
		checkCmd = fmt.Sprintf("%s/mysql/bin/mysqladmin -u root ping 2>/dev/null", installPath)
		if _, err = client.Run(checkCmd); err == nil {
			mysqlReady = true
			break
		}
	}

	if !mysqlReady {
		// 获取详细错误信息
		status, _ := client.RunWithSudo("systemctl status mysqld --no-pager -l")
		logs, _ := client.RunWithSudo("tail -50 /var/log/mysql/error.log 2>/dev/null || journalctl -u mysqld --no-pager -l -n 30")
		return fmt.Errorf("MySQL not ready after 60 seconds\nStatus: %s\nLogs: %s", status, logs)
	}

	progress(90, "Initializing MySQL accounts...")
	if err := i.initMySQLAccounts(client, installPath, settings); err != nil {
		return fmt.Errorf("failed to initialize MySQL accounts: %w", err)
	}

	progress(100, "MySQL installation completed")
	return nil
}

// initMySQLAccounts initializes MySQL user accounts
func (i *Installer) initMySQLAccounts(client *SSHClient, installPath string, settings *ClusterSettings) error {
	// 检查是否需要设置密码（初始化后 root 无密码）
	checkPassCmd := fmt.Sprintf("%s/mysql/bin/mysql -u root --socket=/tmp/mysql.sock -e \"SELECT 1\" 2>/dev/null", installPath)
	_, noPassErr := client.Run(checkPassCmd)

	var mysqlCmd string
	if noPassErr == nil {
		// 无密码可以连接，说明是新安装
		mysqlCmd = fmt.Sprintf("%s/mysql/bin/mysql -u root --socket=/tmp/mysql.sock", installPath)
	} else {
		// 需要密码
		mysqlCmd = fmt.Sprintf("%s/mysql/bin/mysql -u root -p'%s' --socket=/tmp/mysql.sock", installPath, settings.RootPassword)
	}

	// 设置 root 密码
	if noPassErr == nil {
		setPassCmd := fmt.Sprintf("%s -e \"ALTER USER 'root'@'localhost' IDENTIFIED BY '%s'; FLUSH PRIVILEGES;\"", mysqlCmd, settings.RootPassword)
		if output, err := client.RunWithSudo(setPassCmd); err != nil {
			return fmt.Errorf("failed to set root password: %w, output: %s", err, output)
		}
		// 更新连接命令使用新密码
		mysqlCmd = fmt.Sprintf("%s/mysql/bin/mysql -u root -p'%s' --socket=/tmp/mysql.sock", installPath, settings.RootPassword)
	}

	// 创建 root@127.0.0.1 用户（HA Agent 使用）
	agentUserCmd := fmt.Sprintf("%s -e \"CREATE USER IF NOT EXISTS 'root'@'127.0.0.1' IDENTIFIED BY '%s'; GRANT ALL PRIVILEGES ON *.* TO 'root'@'127.0.0.1' WITH GRANT OPTION; FLUSH PRIVILEGES;\"",
		mysqlCmd, settings.RootPassword)
	if output, err := client.RunWithSudo(agentUserCmd); err != nil {
		return fmt.Errorf("failed to create root@127.0.0.1: %w, output: %s", err, output)
	}

	// 创建 root@% 用户（远程管理使用）
	remoteRootCmd := fmt.Sprintf("%s -e \"CREATE USER IF NOT EXISTS 'root'@'%%' IDENTIFIED BY '%s'; GRANT ALL PRIVILEGES ON *.* TO 'root'@'%%' WITH GRANT OPTION; FLUSH PRIVILEGES;\"",
		mysqlCmd, settings.RootPassword)
	if output, err := client.RunWithSudo(remoteRootCmd); err != nil {
		// 远程 root 用户创建失败不是致命错误
		fmt.Printf("Warning: failed to create root@%%: %v, output: %s\n", err, output)
	}

	// 创建复制用户
	replCmd := fmt.Sprintf("%s -e \"CREATE USER IF NOT EXISTS '%s'@'%%' IDENTIFIED BY '%s'; GRANT REPLICATION SLAVE ON *.* TO '%s'@'%%'; FLUSH PRIVILEGES;\"",
		mysqlCmd, settings.ReplicationUser, settings.ReplicationPass, settings.ReplicationUser)
	if output, err := client.RunWithSudo(replCmd); err != nil {
		return fmt.Errorf("failed to create replication user: %w, output: %s", err, output)
	}

	return nil
}

// InitMySQLAccountsOnHost initializes MySQL accounts on a specific host (can be called separately)
func (i *Installer) InitMySQLAccountsOnHost(host *Host, installPath string, settings *ClusterSettings) error {
	client, err := NewSSHClient(host)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Close()

	return i.initMySQLAccounts(client, installPath, settings)
}

// InstallHAAgent installs the HA agent on a remote host
func (i *Installer) InstallHAAgent(host *Host, installPath string, config *ClusterConfig, progress func(int, string)) error {
	client, err := NewSSHClient(host)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Close()

	progress(20, "Creating agent directory...")
	if _, err := client.RunWithSudo(fmt.Sprintf("mkdir -p %s/mypatroni /etc/mypatroni /var/log/mypatroni", installPath)); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	progress(40, "Generating agent configuration...")
	agentConfig := generateAgentConfig(host, config)
	// 使用 cat 和 heredoc 写入文件，避免特殊字符问题
	configCmd := fmt.Sprintf("cat > /etc/mypatroni/config.yaml << 'AGENT_CONFIG_EOF'\n%s\nAGENT_CONFIG_EOF", agentConfig)
	if _, err := client.RunWithSudo(configCmd); err != nil {
		return fmt.Errorf("failed to create config: %w", err)
	}

	progress(60, "Creating agent service...")
	serviceContent := generateAgentService(installPath)
	// 使用 cat 和 heredoc 写入文件，避免特殊字符问题
	serviceCmd := fmt.Sprintf("cat > /etc/systemd/system/mypatroni.service << 'AGENT_SERVICE_EOF'\n%s\nAGENT_SERVICE_EOF", serviceContent)
	if _, err := client.RunWithSudo(serviceCmd); err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}

	progress(100, "HA Agent configuration completed")
	return nil
}

// CreateTask creates a new installation task
func (i *Installer) CreateTask(clusterID, hostID, taskType string) *InstallTask {
	task := &InstallTask{
		ID:        uuid.New().String(),
		ClusterID: clusterID,
		HostID:    hostID,
		Type:      taskType,
		Status:    TaskPending,
		StartTime: time.Now().Unix(),
	}
	i.tasks[task.ID] = task
	return task
}

// GetTask returns a task by ID
func (i *Installer) GetTask(id string) *InstallTask {
	return i.tasks[id]
}

// Helper functions

func generateServerID(ip string) int {
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return 1
	}
	var id int
	for _, p := range parts[2:] {
		var n int
		fmt.Sscanf(p, "%d", &n)
		id = id*256 + n
	}
	return id
}

func generateEtcdService(name, ip, installPath string, etcdNodes []Host) string {
	// 构建 initial-cluster 参数
	var initialCluster []string
	for _, node := range etcdNodes {
		initialCluster = append(initialCluster, fmt.Sprintf("%s=http://%s:2380", node.Name, node.IP))
	}
	initialClusterStr := strings.Join(initialCluster, ",")

	// 判断集群状态：如果是新集群用 "new"，如果是加入现有集群用 "existing"
	clusterState := "new"

	return fmt.Sprintf(`[Unit]
Description=etcd
After=network.target

[Service]
Type=notify
ExecStart=%s/etcd/etcd \
  --name=%s \
  --data-dir=/var/lib/etcd \
  --listen-client-urls=http://%s:2379,http://127.0.0.1:2379 \
  --advertise-client-urls=http://%s:2379 \
  --listen-peer-urls=http://%s:2380 \
  --initial-advertise-peer-urls=http://%s:2380 \
  --initial-cluster=%s \
  --initial-cluster-token=mysql-ha-etcd-cluster \
  --initial-cluster-state=%s
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
`, installPath, name, ip, ip, ip, ip, initialClusterStr, clusterState)
}

func generateMySQLConfig(ip string, serverID int, installPath, dataPath string, settings *ClusterSettings, version string) string {
	gtidMode := "ON"
	return fmt.Sprintf(`[mysqld]
basedir=%s/mysql
datadir=%s
socket=/tmp/mysql.sock
port=%d
server-id=%d
pid-file=/var/run/mysqld/mysqld.pid
log-error=/var/log/mysql/error.log
log-bin=mysql-bin
gtid-mode=%s
enforce-gtid-consistency=ON
log-slave-updates=ON
binlog-format=ROW
bind-address=0.0.0.0
skip-name-resolve

[client]
socket=/tmp/mysql.sock
`, installPath, dataPath, settings.MySQLPort, serverID, gtidMode)
}

func generateMySQLService(installPath, dataPath string) string {
	return fmt.Sprintf(`[Unit]
Description=MySQL Server
After=network.target

[Service]
Type=simple
User=mysql
Group=mysql
# 确保必要的目录存在且权限正确
ExecStartPre=/bin/bash -c 'mkdir -p /var/run/mysqld /var/log/mysql %s && chown mysql:mysql /var/run/mysqld /var/log/mysql %s && chmod 755 /var/run/mysqld /var/log/mysql && chmod 750 %s'
ExecStart=%s/mysql/bin/mysqld --defaults-file=/etc/my.cnf --user=mysql
ExecStop=/bin/kill -SIGTERM $MAINPID
TimeoutStartSec=300
TimeoutStopSec=60
Restart=on-failure
RestartSec=5
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
`, dataPath, dataPath, dataPath, installPath)
}

func generateAgentConfig(host *Host, config *ClusterConfig) string {
	var etcdEndpoints []string
	for _, h := range config.Hosts {
		if h.IsEtcdNode() {
			etcdEndpoints = append(etcdEndpoints, fmt.Sprintf("http://%s:2379", h.IP))
		}
	}

	return fmt.Sprintf(`name: %s
namespace: mypatroni
scope: %s
advertise_host: "%s"

dcs:
  endpoints:
%s

mysql:
  host: "127.0.0.1"
  port: %d
  user: "root"
  password: "%s"
  replication_user: "%s"
  replication_password: "%s"

api:
  listen: "0.0.0.0"
  port: %d

log:
  level: "info"
  file: "/var/log/mypatroni/mypatroni.log"

ha:
  ttl: 30
  loop_wait: 10
  retry_timeout: 10
  max_replication_lag: 10
  failover_cooldown: 60
`,
		host.Name,
		config.Name,
		host.IP, // Add advertise_host with the node's external IP
		formatEndpoints(etcdEndpoints),
		config.Settings.MySQLPort,
		config.Settings.RootPassword,
		config.Settings.ReplicationUser,
		config.Settings.ReplicationPass,
		config.Settings.HAAgentPort,
	)
}

func formatEndpoints(endpoints []string) string {
	var lines []string
	for _, ep := range endpoints {
		lines = append(lines, fmt.Sprintf("    - \"%s\"", ep))
	}
	return strings.Join(lines, "\n")
}

func generateAgentService(installPath string) string {
	return fmt.Sprintf(`[Unit]
Description=MyPatroni HA Agent
After=network.target mysqld.service

[Service]
Type=simple
ExecStart=%s/mypatroni/mypatroni --config /etc/mypatroni/config.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
`, installPath)
}

// ConfigureReplication configures MySQL replication for the cluster
func (i *Installer) ConfigureReplication(config *ClusterConfig) error {
	var mysqlHosts []*Host
	for idx := range config.Hosts {
		if config.Hosts[idx].IsMySQLNode() {
			mysqlHosts = append(mysqlHosts, &config.Hosts[idx])
		}
	}

	if len(mysqlHosts) < 2 {
		return nil
	}

	master := mysqlHosts[0]
	masterClient, err := NewSSHClient(master)
	if err != nil {
		return fmt.Errorf("failed to connect to master: %w", err)
	}
	defer masterClient.Close()

	gtidCmd := fmt.Sprintf("%s/mysql/bin/mysql -u root -p'%s' -e \"SHOW MASTER STATUS\\G\"",
		config.InstallPath, config.Settings.RootPassword)
	if _, err = masterClient.Run(gtidCmd); err != nil {
		return fmt.Errorf("failed to get master status: %w", err)
	}

	for _, slave := range mysqlHosts[1:] {
		slaveClient, err := NewSSHClient(slave)
		if err != nil {
			return fmt.Errorf("failed to connect to slave %s: %w", slave.Name, err)
		}

		replCmd := fmt.Sprintf(`%s/mysql/bin/mysql -u root -p'%s' -e "STOP SLAVE; CHANGE MASTER TO MASTER_HOST='%s', MASTER_PORT=%d, MASTER_USER='%s', MASTER_PASSWORD='%s', MASTER_AUTO_POSITION=1; START SLAVE;"`,
			config.InstallPath, config.Settings.RootPassword,
			master.IP, config.Settings.MySQLPort,
			config.Settings.ReplicationUser, config.Settings.ReplicationPass)

		if _, err := slaveClient.Run(replCmd); err != nil {
			slaveClient.Close()
			return fmt.Errorf("failed to configure replication on %s: %w", slave.Name, err)
		}

		checkCmd := fmt.Sprintf("%s/mysql/bin/mysql -u root -p'%s' -e \"SHOW SLAVE STATUS\\G\"",
			config.InstallPath, config.Settings.RootPassword)
		output, _ := slaveClient.Run(checkCmd)
		if !strings.Contains(output, "Slave_IO_Running: Yes") || !strings.Contains(output, "Slave_SQL_Running: Yes") {
			slaveClient.Close()
			return fmt.Errorf("replication not running properly on %s", slave.Name)
		}
		slaveClient.Close()
	}
	return nil
}

// UploadAndStartHAAgent uploads the HA agent binary and starts the service
func (i *Installer) UploadAndStartHAAgent(host *Host, installPath, agentBinaryPath string, progress func(int, string)) error {
	client, err := NewSSHClient(host)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Close()

	// 先停止已运行的服务，避免 "Text file busy" 错误
	progress(65, "Stopping existing HA Agent service if running...")
	client.RunWithSudo("systemctl stop mypatroni 2>/dev/null || true")
	// 等待进程完全退出
	time.Sleep(2 * time.Second)
	// 确保进程已停止
	client.RunWithSudo("pkill -9 mypatroni 2>/dev/null || true")
	time.Sleep(1 * time.Second)

	progress(70, "Uploading HA Agent binary...")
	// 先删除旧文件
	client.RunWithSudo(fmt.Sprintf("rm -f %s/mypatroni/mypatroni 2>/dev/null || true", installPath))

	if err := client.UploadFile(agentBinaryPath, fmt.Sprintf("%s/mypatroni/mypatroni", installPath)); err != nil {
		return fmt.Errorf("failed to upload agent binary: %w", err)
	}

	if _, err := client.RunWithSudo(fmt.Sprintf("chmod +x %s/mypatroni/mypatroni", installPath)); err != nil {
		return fmt.Errorf("failed to set execute permission: %w", err)
	}

	progress(85, "Starting HA Agent service...")
	if _, err := client.RunWithSudo("systemctl daemon-reload"); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}

	if _, err := client.RunWithSudo("systemctl enable mypatroni"); err != nil {
		return fmt.Errorf("failed to enable mypatroni: %w", err)
	}

	if _, err := client.RunWithSudo("systemctl start mypatroni"); err != nil {
		return fmt.Errorf("failed to start mypatroni: %w", err)
	}

	time.Sleep(3 * time.Second)

	if _, err := client.Run("systemctl is-active mypatroni"); err != nil {
		status, _ := client.RunWithSudo("systemctl status mypatroni --no-pager -l")
		return fmt.Errorf("mypatroni service not running: %s", status)
	}

	progress(100, "HA Agent started successfully")
	return nil
}

// VerifyEtcdCluster verifies that the etcd cluster is healthy
func (i *Installer) VerifyEtcdCluster(etcdNodes []Host) error {
	if len(etcdNodes) == 0 {
		return fmt.Errorf("no etcd nodes provided")
	}

	// 连接到第一个节点验证集群状态
	host := &etcdNodes[0]
	client, err := NewSSHClient(host)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", host.Name, err)
	}
	defer client.Close()

	// 检查 etcd 集群健康状态
	checkCmd := "etcdctl endpoint health --cluster 2>/dev/null || /usr/local/bin/etcdctl endpoint health --cluster 2>/dev/null"
	output, err := client.Run(checkCmd)
	if err != nil {
		// 尝试使用 API v2
		checkCmd = fmt.Sprintf("etcdctl --endpoints=http://%s:2379 cluster-health 2>/dev/null", host.IP)
		output, err = client.Run(checkCmd)
		if err != nil {
			return fmt.Errorf("etcd cluster health check failed: %w, output: %s", err, output)
		}
	}

	// 检查所有节点是否健康
	for _, node := range etcdNodes {
		if !strings.Contains(output, node.IP) && !strings.Contains(output, "healthy") {
			// 单独检查每个节点
			nodeCheckCmd := fmt.Sprintf("etcdctl --endpoints=http://%s:2379 endpoint health", node.IP)
			nodeOutput, nodeErr := client.Run(nodeCheckCmd)
			if nodeErr != nil || !strings.Contains(nodeOutput, "healthy") {
				return fmt.Errorf("etcd node %s is not healthy", node.Name)
			}
		}
	}

	return nil
}

// VerifyMySQLConnection verifies that MySQL is accessible on a host
func (i *Installer) VerifyMySQLConnection(host *Host, installPath string, settings *ClusterSettings) error {
	client, err := NewSSHClient(host)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", host.Name, err)
	}
	defer client.Close()

	// 使用 mysqladmin ping 检查连接
	pingCmd := fmt.Sprintf("%s/mysql/bin/mysqladmin -u root -p'%s' --socket=/tmp/mysql.sock ping 2>/dev/null",
		installPath, settings.RootPassword)
	output, err := client.Run(pingCmd)
	if err != nil || !strings.Contains(output, "alive") {
		return fmt.Errorf("MySQL is not responding: %v", err)
	}

	return nil
}

// VerifyReplication verifies that MySQL replication is working
func (i *Installer) VerifyReplication(config *ClusterConfig) error {
	var mysqlHosts []*Host
	for idx := range config.Hosts {
		if config.Hosts[idx].IsMySQLNode() {
			mysqlHosts = append(mysqlHosts, &config.Hosts[idx])
		}
	}

	if len(mysqlHosts) < 2 {
		return nil // 单节点无需验证
	}

	// 检查每个 slave 的复制状态
	for _, slave := range mysqlHosts[1:] {
		client, err := NewSSHClient(slave)
		if err != nil {
			return fmt.Errorf("failed to connect to slave %s: %w", slave.Name, err)
		}

		checkCmd := fmt.Sprintf("%s/mysql/bin/mysql -u root -p'%s' --socket=/tmp/mysql.sock -e \"SHOW SLAVE STATUS\\G\"",
			config.InstallPath, config.Settings.RootPassword)
		output, err := client.Run(checkCmd)
		client.Close()

		if err != nil {
			return fmt.Errorf("failed to check replication status on %s: %w", slave.Name, err)
		}

		if !strings.Contains(output, "Slave_IO_Running: Yes") {
			return fmt.Errorf("Slave_IO_Running is not Yes on %s", slave.Name)
		}
		if !strings.Contains(output, "Slave_SQL_Running: Yes") {
			return fmt.Errorf("Slave_SQL_Running is not Yes on %s", slave.Name)
		}
	}

	return nil
}

// PerformSwitchover performs MySQL master-slave switchover
// newMaster: the slave node to be promoted to master
// oldMaster: the current master node to be demoted to slave
func (i *Installer) PerformSwitchover(config *ClusterConfig, newMasterID string) error {
	// 找到新主节点和当前主节点
	var newMaster, oldMaster *Host
	for idx := range config.Hosts {
		host := &config.Hosts[idx]
		if host.ID == newMasterID {
			newMaster = host
		}
		if host.HasRole(RoleMaster) {
			oldMaster = host
		}
	}

	if newMaster == nil {
		return fmt.Errorf("new master node not found")
	}
	if oldMaster == nil {
		return fmt.Errorf("current master node not found")
	}
	if newMaster.ID == oldMaster.ID {
		return fmt.Errorf("new master is already the current master")
	}

	mysqlCmd := func(host *Host) string {
		return fmt.Sprintf("%s/mysql/bin/mysql -u root -p'%s' --socket=/tmp/mysql.sock",
			config.InstallPath, config.Settings.RootPassword)
	}

	// Step 1: 在旧主节点上设置只读，阻止新写入
	oldMasterClient, err := NewSSHClient(oldMaster)
	if err != nil {
		return fmt.Errorf("failed to connect to old master: %w", err)
	}
	defer oldMasterClient.Close()

	// 设置只读
	if _, err := oldMasterClient.Run(fmt.Sprintf("%s -e \"SET GLOBAL read_only = ON;\"", mysqlCmd(oldMaster))); err != nil {
		return fmt.Errorf("failed to set old master read_only: %w", err)
	}

	// Step 2: 等待新主节点同步完成
	newMasterClient, err := NewSSHClient(newMaster)
	if err != nil {
		// 回滚：恢复旧主节点可写
		oldMasterClient.Run(fmt.Sprintf("%s -e \"SET GLOBAL read_only = OFF;\"", mysqlCmd(oldMaster)))
		return fmt.Errorf("failed to connect to new master: %w", err)
	}
	defer newMasterClient.Close()

	// 等待复制追上（最多等待30秒）
	for retry := 0; retry < 30; retry++ {
		output, _ := newMasterClient.Run(fmt.Sprintf("%s -e \"SHOW SLAVE STATUS\\G\"", mysqlCmd(newMaster)))
		if strings.Contains(output, "Seconds_Behind_Master: 0") {
			break
		}
		if retry == 29 {
			// 回滚
			oldMasterClient.Run(fmt.Sprintf("%s -e \"SET GLOBAL read_only = OFF;\"", mysqlCmd(oldMaster)))
			return fmt.Errorf("replication lag timeout, switchover aborted")
		}
		time.Sleep(1 * time.Second)
	}

	// Step 3: 在新主节点上停止复制并重置
	if _, err := newMasterClient.Run(fmt.Sprintf("%s -e \"STOP SLAVE; RESET SLAVE ALL;\"", mysqlCmd(newMaster))); err != nil {
		oldMasterClient.Run(fmt.Sprintf("%s -e \"SET GLOBAL read_only = OFF;\"", mysqlCmd(oldMaster)))
		return fmt.Errorf("failed to stop slave on new master: %w", err)
	}

	// Step 4: 设置新主节点为可写
	if _, err := newMasterClient.Run(fmt.Sprintf("%s -e \"SET GLOBAL read_only = OFF;\"", mysqlCmd(newMaster))); err != nil {
		return fmt.Errorf("failed to set new master writable: %w", err)
	}

	// Step 5: 将旧主节点配置为新主节点的从节点
	replCmd := fmt.Sprintf(`%s -e "CHANGE MASTER TO MASTER_HOST='%s', MASTER_PORT=%d, MASTER_USER='%s', MASTER_PASSWORD='%s', MASTER_AUTO_POSITION=1; START SLAVE;"`,
		mysqlCmd(oldMaster), newMaster.IP, config.Settings.MySQLPort,
		config.Settings.ReplicationUser, config.Settings.ReplicationPass)
	if _, err := oldMasterClient.Run(replCmd); err != nil {
		return fmt.Errorf("failed to configure old master as slave: %w", err)
	}

	// Step 6: 重新配置其他从节点指向新主节点
	for idx := range config.Hosts {
		host := &config.Hosts[idx]
		if !host.IsMySQLNode() || host.ID == newMaster.ID || host.ID == oldMaster.ID {
			continue
		}

		slaveClient, err := NewSSHClient(host)
		if err != nil {
			continue // 跳过无法连接的节点
		}

		replCmd := fmt.Sprintf(`%s -e "STOP SLAVE; CHANGE MASTER TO MASTER_HOST='%s', MASTER_PORT=%d, MASTER_USER='%s', MASTER_PASSWORD='%s', MASTER_AUTO_POSITION=1; START SLAVE;"`,
			mysqlCmd(host), newMaster.IP, config.Settings.MySQLPort,
			config.Settings.ReplicationUser, config.Settings.ReplicationPass)
		slaveClient.Run(replCmd)
		slaveClient.Close()
	}

	return nil
}
