package installer

import (
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHClient wraps SSH connection
type SSHClient struct {
	client *ssh.Client
	host   *Host
}

// NewSSHClient creates a new SSH client
func NewSSHClient(host *Host) (*SSHClient, error) {
	var authMethods []ssh.AuthMethod

	// Password authentication
	if host.Password != "" {
		authMethods = append(authMethods, ssh.Password(host.Password))
	}

	// Key file authentication
	if host.KeyFile != "" {
		key, err := os.ReadFile(host.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read key file: %w", err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no authentication method provided")
	}

	config := &ssh.ClientConfig{
		User:            host.Username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	port := host.Port
	if port == 0 {
		port = 22
	}

	addr := fmt.Sprintf("%s:%d", host.IP, port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", addr, err)
	}

	return &SSHClient{
		client: client,
		host:   host,
	}, nil
}

// Close closes the SSH connection
func (c *SSHClient) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

// Run executes a command on the remote host
func (c *SSHClient) Run(cmd string) (string, error) {
	session, err := c.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(cmd)
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w, output: %s", err, string(output))
	}

	return string(output), nil
}

// RunWithSudo executes a command with sudo
func (c *SSHClient) RunWithSudo(cmd string) (string, error) {
	session, err := c.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	// 使用 stdin 传递密码和命令
	stdin, err := session.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	// 使用 sudo -S 从 stdin 读取密码，然后执行 bash 从 stdin 读取脚本
	sudoCmd := "sudo -S bash"

	// 异步写入密码和命令
	go func() {
		defer stdin.Close()
		// 先写密码
		fmt.Fprintf(stdin, "%s\n", c.host.Password)
		// 再写命令
		fmt.Fprintf(stdin, "%s\n", cmd)
	}()

	output, err := session.CombinedOutput(sudoCmd)
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w, output: %s", err, string(output))
	}

	return string(output), nil
}

// UploadFile uploads a file to the remote host using base64 encoding
func (c *SSHClient) UploadFile(localPath, remotePath string) error {
	// Read local file
	content, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("failed to read local file: %w", err)
	}

	// Get file permissions
	stat, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("failed to stat local file: %w", err)
	}
	perm := stat.Mode().Perm()

	// Encode to base64
	encoded := base64.StdEncoding.EncodeToString(content)

	// Create a temporary file on remote with base64 content
	tmpFile := remotePath + ".b64"

	// Write base64 content to temp file (using heredoc to avoid command line length limits)
	writeCmd := fmt.Sprintf("cat > %s << 'EOF_BASE64'\n%s\nEOF_BASE64", tmpFile, encoded)
	if _, err := c.RunWithSudo(writeCmd); err != nil {
		return fmt.Errorf("failed to write base64 content: %w", err)
	}

	// Decode base64 and write to final location
	decodeCmd := fmt.Sprintf("base64 -d %s > %s && rm %s && chmod %o %s",
		tmpFile, remotePath, tmpFile, perm, remotePath)
	if _, err := c.RunWithSudo(decodeCmd); err != nil {
		return fmt.Errorf("failed to decode and write file: %w", err)
	}

	return nil
}

// CheckConnection tests the SSH connection
func (c *SSHClient) CheckConnection() error {
	_, err := c.Run("echo 'connection test'")
	return err
}

// CheckSudo checks if the user has sudo privileges
func (c *SSHClient) CheckSudo() error {
	_, err := c.RunWithSudo("whoami")
	return err
}

// GetSystemInfo returns system information
func (c *SSHClient) GetSystemInfo() (map[string]string, error) {
	info := make(map[string]string)

	// Get OS info
	osInfo, err := c.Run("cat /etc/os-release | grep -E '^(ID|VERSION_ID)=' | head -2")
	if err == nil {
		info["os"] = osInfo
	}

	// Get architecture
	arch, err := c.Run("uname -m")
	if err == nil {
		info["arch"] = arch
	}

	// Get memory
	mem, err := c.Run("free -h | grep Mem | awk '{print $2}'")
	if err == nil {
		info["memory"] = mem
	}

	// Get disk space
	disk, err := c.Run("df -h / | tail -1 | awk '{print $4}'")
	if err == nil {
		info["disk_free"] = disk
	}

	return info, nil
}

// TestConnection tests if a host is reachable
func TestConnection(ip string, port int) error {
	if port == 0 {
		port = 22
	}
	addr := fmt.Sprintf("%s:%d", ip, port)
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}
