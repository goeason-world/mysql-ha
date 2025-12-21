package mysql

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// Manager implements MySQL management operations
type Manager struct {
	db      *sql.DB
	config  Config
	version *Version
	replCmd *ReplicationCommand
}

// Config holds MySQL connection configuration
type Config struct {
	Host            string
	Port            int
	User            string
	Password        string
	ReplicationUser string
	ReplicationPass string
}

// ReplicationStatus contains MySQL replication status
type ReplicationStatus struct {
	SlaveIORunning      bool
	SlaveSQLRunning     bool
	MasterHost          string
	MasterPort          int
	SecondsBehindMaster *int64
	GTIDExecuted        string
	RetrievedGTIDSet    string
	ExecutedGTIDSet     string
	LastError           string
}

// GTIDPosition represents GTID position
type GTIDPosition struct {
	GTIDExecuted string
	GTIDPurged   string
}

// NewManager creates a new MySQL manager
func NewManager(cfg Config) *Manager {
	return &Manager{
		config: cfg,
	}
}

// Connect establishes connection to MySQL
func (m *Manager) Connect() error {
	// Close existing connection if any
	if m.db != nil {
		m.db.Close()
		m.db = nil
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/?timeout=10s",
		m.config.User, m.config.Password, m.config.Host, m.config.Port)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to open MySQL connection: %w", err)
	}

	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(time.Minute * 5)

	if err := db.Ping(); err != nil {
		db.Close()
		return fmt.Errorf("failed to ping MySQL: %w", err)
	}

	m.db = db

	// Get and validate version
	version, err := m.GetVersion()
	if err != nil {
		db.Close()
		m.db = nil
		return fmt.Errorf("failed to get MySQL version: %w", err)
	}

	v, err := ParseVersion(version)
	if err != nil {
		db.Close()
		m.db = nil
		return err
	}

	if !v.IsSupported() {
		db.Close()
		m.db = nil
		return fmt.Errorf("unsupported MySQL version: %s", version)
	}

	m.version = v
	m.replCmd = NewReplicationCommand(v)

	return nil
}

// Close closes the MySQL connection
func (m *Manager) Close() error {
	if m.db != nil {
		return m.db.Close()
	}
	return nil
}

// Ping checks MySQL connectivity
func (m *Manager) Ping() error {
	if m.db == nil {
		return fmt.Errorf("not connected")
	}
	return m.db.Ping()
}

// GetVersion returns the MySQL version string
func (m *Manager) GetVersion() (string, error) {
	var version string
	err := m.db.QueryRow("SELECT VERSION()").Scan(&version)
	if err != nil {
		return "", fmt.Errorf("failed to get version: %w", err)
	}
	return version, nil
}

// GetServerID returns the MySQL server ID
func (m *Manager) GetServerID() (uint32, error) {
	var serverID uint32
	err := m.db.QueryRow("SELECT @@server_id").Scan(&serverID)
	if err != nil {
		return 0, fmt.Errorf("failed to get server_id: %w", err)
	}
	return serverID, nil
}

// GetGTIDExecuted returns the GTID executed set
func (m *Manager) GetGTIDExecuted() (string, error) {
	var gtid sql.NullString
	err := m.db.QueryRow("SELECT @@gtid_executed").Scan(&gtid)
	if err != nil {
		return "", fmt.Errorf("failed to get gtid_executed: %w", err)
	}
	return gtid.String, nil
}

// IsReadOnly returns true if the server is in read-only mode
func (m *Manager) IsReadOnly() (bool, error) {
	var readOnly bool
	err := m.db.QueryRow("SELECT @@read_only").Scan(&readOnly)
	if err != nil {
		return false, fmt.Errorf("failed to get read_only: %w", err)
	}
	return readOnly, nil
}

// SetReadOnly sets the read-only mode
func (m *Manager) SetReadOnly(readonly bool) error {
	val := 0
	if readonly {
		val = 1
	}
	_, err := m.db.Exec(fmt.Sprintf("SET GLOBAL read_only = %d", val))
	if err != nil {
		return fmt.Errorf("failed to set read_only: %w", err)
	}

	// Also set super_read_only for MySQL 5.7.8+
	if m.version != nil && (m.version.Is80() || (m.version.Is57() && m.version.Patch >= 8)) {
		_, err = m.db.Exec(fmt.Sprintf("SET GLOBAL super_read_only = %d", val))
		if err != nil {
			// Ignore error for super_read_only as it might not be available
		}
	}

	return nil
}

// GetReplicationStatus returns the replication status
func (m *Manager) GetReplicationStatus() (*ReplicationStatus, error) {
	if m.replCmd == nil {
		return nil, fmt.Errorf("version not initialized")
	}

	rows, err := m.db.Query(m.replCmd.ShowSlaveStatusSQL())
	if err != nil {
		return nil, fmt.Errorf("failed to get replication status: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil // Not a replica
	}

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	values := make([]interface{}, len(cols))
	valuePtrs := make([]interface{}, len(cols))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	if err := rows.Scan(valuePtrs...); err != nil {
		return nil, err
	}

	status := &ReplicationStatus{}
	for i, col := range cols {
		val := values[i]
		switch col {
		case "Slave_IO_Running", "Replica_IO_Running":
			if v, ok := val.([]byte); ok {
				status.SlaveIORunning = string(v) == "Yes"
			}
		case "Slave_SQL_Running", "Replica_SQL_Running":
			if v, ok := val.([]byte); ok {
				status.SlaveSQLRunning = string(v) == "Yes"
			}
		case "Master_Host", "Source_Host":
			if v, ok := val.([]byte); ok {
				status.MasterHost = string(v)
			}
		case "Master_Port", "Source_Port":
			if v, ok := val.(int64); ok {
				status.MasterPort = int(v)
			}
		case "Seconds_Behind_Master", "Seconds_Behind_Source":
			if v, ok := val.(int64); ok {
				status.SecondsBehindMaster = &v
			}
		case "Executed_Gtid_Set":
			if v, ok := val.([]byte); ok {
				status.ExecutedGTIDSet = string(v)
			}
		case "Retrieved_Gtid_Set":
			if v, ok := val.([]byte); ok {
				status.RetrievedGTIDSet = string(v)
			}
		case "Last_Error", "Last_SQL_Error":
			if v, ok := val.([]byte); ok {
				status.LastError = string(v)
			}
		}
	}

	return status, nil
}

// GetReplicationLag returns the replication lag in seconds
func (m *Manager) GetReplicationLag() (int64, error) {
	status, err := m.GetReplicationStatus()
	if err != nil {
		return 0, err
	}
	if status == nil || status.SecondsBehindMaster == nil {
		return 0, nil
	}
	return *status.SecondsBehindMaster, nil
}

// StartReplication starts replication to the specified master
func (m *Manager) StartReplication(masterHost string, masterPort int, user, password string) error {
	if m.replCmd == nil {
		return fmt.Errorf("version not initialized")
	}

	// Stop any existing replication
	_, _ = m.db.Exec(m.replCmd.StopSlaveSQL())

	// Configure replication
	sql := m.replCmd.StartReplicationSQL(masterHost, masterPort, user, password, true)
	if _, err := m.db.Exec(sql); err != nil {
		return fmt.Errorf("failed to configure replication: %w", err)
	}

	// Start replication
	if _, err := m.db.Exec(m.replCmd.StartSlaveSQL()); err != nil {
		return fmt.Errorf("failed to start replication: %w", err)
	}

	return nil
}

// StopReplication stops replication
func (m *Manager) StopReplication() error {
	if m.replCmd == nil {
		return fmt.Errorf("version not initialized")
	}

	_, err := m.db.Exec(m.replCmd.StopSlaveSQL())
	if err != nil {
		return fmt.Errorf("failed to stop replication: %w", err)
	}
	return nil
}

// ResetReplication resets replication configuration
func (m *Manager) ResetReplication() error {
	if m.replCmd == nil {
		return fmt.Errorf("version not initialized")
	}

	_, err := m.db.Exec(m.replCmd.ResetSlaveSQL())
	if err != nil {
		return fmt.Errorf("failed to reset replication: %w", err)
	}
	return nil
}

// WaitForGTID waits for the specified GTID to be executed
func (m *Manager) WaitForGTID(gtid string, timeout time.Duration) error {
	sql := fmt.Sprintf("SELECT WAIT_FOR_EXECUTED_GTID_SET('%s', %d)", gtid, int(timeout.Seconds()))
	var result int
	err := m.db.QueryRow(sql).Scan(&result)
	if err != nil {
		return fmt.Errorf("failed to wait for GTID: %w", err)
	}
	if result != 0 {
		return fmt.Errorf("timeout waiting for GTID")
	}
	return nil
}

// GetGTIDPosition returns the current GTID position
func (m *Manager) GetGTIDPosition() (*GTIDPosition, error) {
	var executed, purged sql.NullString

	err := m.db.QueryRow("SELECT @@gtid_executed").Scan(&executed)
	if err != nil {
		return nil, fmt.Errorf("failed to get gtid_executed: %w", err)
	}

	err = m.db.QueryRow("SELECT @@gtid_purged").Scan(&purged)
	if err != nil {
		return nil, fmt.Errorf("failed to get gtid_purged: %w", err)
	}

	return &GTIDPosition{
		GTIDExecuted: executed.String,
		GTIDPurged:   purged.String,
	}, nil
}

// GetParsedVersion returns the parsed version
func (m *Manager) GetParsedVersion() *Version {
	return m.version
}
