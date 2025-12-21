package main

import (
	"fmt"
	"log"
	"mysql-ha/internal/installer"
)

func main() {
	nodes := []struct {
		name string
		ip   string
	}{
		{"node-1", "10.211.55.32"},
		{"node-2", "10.211.55.33"},
		{"node-3", "10.211.55.34"},
	}

	for _, node := range nodes {
		fmt.Printf("\n========== %s (%s) 故障转移相关日志 ==========\n", node.name, node.ip)

		host := &installer.Host{
			IP:       node.ip,
			Port:     22,
			Username: "mngdb",
			Password: "wuhua@2013",
		}

		client, err := installer.NewSSHClient(host)
		if err != nil {
			log.Printf("Failed to connect to %s: %v\n", node.name, err)
			continue
		}

		// 搜索日志文件中的故障转移相关日志
		output, _ := client.Run("grep -iE 'failover|repair|leader|acquired|promoting|environment|restart|MySQL service|connected to MySQL' /var/log/mypatroni/mypatroni.log 2>/dev/null | tail -30")
		if output == "" {
			output = "No failover-related logs found in log file"
		}
		fmt.Println(output)

		client.Close()
	}
}
