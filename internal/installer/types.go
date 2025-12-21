// Package installer provides remote installation capabilities
package installer

// SoftwareVersion defines available software versions
type SoftwareVersion struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	DownloadURL string `json:"download_url"`
	Checksum    string `json:"checksum,omitempty"`
}

// Software download URLs
var (
	// etcd versions
	EtcdVersions = []SoftwareVersion{
		{
			Name:        "etcd",
			Version:     "3.5.13",
			DownloadURL: "https://github.com/etcd-io/etcd/releases/download/v3.5.13/etcd-v3.5.13-linux-amd64.tar.gz",
		},
		{
			Name:        "etcd",
			Version:     "3.5.12",
			DownloadURL: "https://github.com/etcd-io/etcd/releases/download/v3.5.12/etcd-v3.5.12-linux-amd64.tar.gz",
		},
	}

	// MySQL 5.7 versions
	MySQL57Versions = []SoftwareVersion{
		{
			Name:        "mysql",
			Version:     "5.7.44",
			DownloadURL: "https://dev.mysql.com/get/Downloads/MySQL-5.7/mysql-5.7.44-linux-glibc2.12-x86_64.tar.gz",
		},
	}

	// MySQL 8.0 versions
	MySQL80Versions = []SoftwareVersion{
		{
			Name:        "mysql",
			Version:     "8.0.36",
			DownloadURL: "https://dev.mysql.com/get/Downloads/MySQL-8.0/mysql-8.0.36-linux-glibc2.17-x86_64.tar.xz",
		},
		{
			Name:        "mysql",
			Version:     "8.0.35",
			DownloadURL: "https://dev.mysql.com/get/Downloads/MySQL-8.0/mysql-8.0.35-linux-glibc2.17-x86_64.tar.xz",
		},
	}
)

// Host represents a remote host configuration
type Host struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	IP       string   `json:"ip"`
	Port     int      `json:"port"`
	Username string   `json:"username"`
	Password string   `json:"password,omitempty"`
	KeyFile  string   `json:"key_file,omitempty"`
	Roles    []string `json:"roles"` // 支持多角色: master, slave, etcd
	Status   string   `json:"status"`
}

// HasRole checks if host has a specific role
func (h *Host) HasRole(role string) bool {
	for _, r := range h.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// IsMySQLNode checks if host is a MySQL node (master or slave)
func (h *Host) IsMySQLNode() bool {
	return h.HasRole(RoleMaster) || h.HasRole(RoleSlave)
}

// IsEtcdNode checks if host is an etcd node
func (h *Host) IsEtcdNode() bool {
	return h.HasRole(RoleEtcd)
}

// ClusterConfig represents the cluster configuration
type ClusterConfig struct {
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	Hosts        []Host           `json:"hosts"`
	EtcdVersion  string           `json:"etcd_version"`
	MySQLVersion string           `json:"mysql_version"`
	InstallPath  string           `json:"install_path"`
	DataPath     string           `json:"data_path"`
	Settings     *ClusterSettings `json:"settings"`
	Phase        string           `json:"phase"` // 安装阶段: phase1_etcd, phase2_mysql, phase3_replication, phase4_agent, phase5_verify, completed, failed_*
}

// ClusterSettings holds cluster-specific settings
type ClusterSettings struct {
	MySQLPort       int    `json:"mysql_port"`
	EtcdClientPort  int    `json:"etcd_client_port"`
	EtcdPeerPort    int    `json:"etcd_peer_port"`
	ReplicationUser string `json:"replication_user"`
	ReplicationPass string `json:"replication_pass"`
	RootPassword    string `json:"root_password"`
	HAAgentPort     int    `json:"ha_agent_port"`
}

// DownloadSource 软件下载源配置
type DownloadSource struct {
	Type    string `json:"type"`     // "remote" 或 "local"
	BaseURL string `json:"base_url"` // 本地文件服务器地址或留空使用官方源
}

// DefaultDownloadSource 默认下载源
var DefaultDownloadSource = DownloadSource{
	Type:    "remote",
	BaseURL: "", // 空表示使用官方源
}

// InstallTask represents an installation task
type InstallTask struct {
	ID        string `json:"id"`
	ClusterID string `json:"cluster_id"`
	HostID    string `json:"host_id"`
	Type      string `json:"type"` // etcd, mysql, agent
	Status    string `json:"status"`
	Progress  int    `json:"progress"`
	Message   string `json:"message"`
	StartTime int64  `json:"start_time"`
	EndTime   int64  `json:"end_time"`
}

// TaskStatus constants
const (
	TaskPending   = "pending"
	TaskRunning   = "running"
	TaskCompleted = "completed"
	TaskFailed    = "failed"
)

// HostRole constants
const (
	RoleMaster = "master"
	RoleSlave  = "slave"
	RoleEtcd   = "etcd"
)
