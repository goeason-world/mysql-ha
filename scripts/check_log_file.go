package main

import (
	"fmt"
	"log"
	"mysql-ha/internal/installer"
)

func main() {
	host := &installer.Host{
		IP:       "10.211.55.32",
		Port:     22,
		Username: "mngdb",
		Password: "wuhua@2013",
	}

	client, err := installer.NewSSHClient(host)
	if err != nil {
		log.Fatalf("Failed to connect: %v\n", err)
	}
	defer client.Close()

	fmt.Println("=== 检查日志文件是否存在 ===")
	output, _ := client.Run("ls -la /var/log/mypatroni/ 2>/dev/null || echo 'Directory not found'")
	fmt.Println(output)

	fmt.Println("\n=== 检查日志文件内容 ===")
	output, _ = client.Run("tail -10 /var/log/mypatroni/mypatroni.log 2>/dev/null || echo 'Log file not found or empty'")
	fmt.Println(output)

	fmt.Println("\n=== 检查 mypatroni 配置中的日志设置 ===")
	output, _ = client.Run("grep -A2 'log:' /etc/mypatroni/config.yaml")
	fmt.Println(output)
}
