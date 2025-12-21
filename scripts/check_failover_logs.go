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
		fmt.Printf("\n========== %s (%s) 日志 ==========\n", node.name, node.ip)

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

		// 检查 mypatroni 日志 - 查找 failover、repair、MySQL 相关的日志
		fmt.Println("--- mypatroni 最近日志 (查找 failover/repair/MySQL 相关) ---")
		output, _ := client.Run("journalctl -u mypatroni --no-pager -n 50 2>/dev/null | grep -iE 'failover|repair|mysql|leader|acquired|promoting|environment|restart|start' || tail -30 /var/log/mypatroni/mypatroni.log 2>/dev/null | grep -iE 'failover|repair|mysql|leader|acquired|promoting|environment|restart|start'")
		if output == "" {
			output, _ = client.Run("journalctl -u mypatroni --no-pager -n 30 2>/dev/null || tail -30 /var/log/mypatroni/mypatroni.log 2>/dev/null")
		}
		fmt.Println(output)

		// 检查 MySQL 服务状态
		fmt.Println("--- MySQL 服务状态 ---")
		output, _ = client.Run("systemctl is-active mysqld 2>/dev/null || systemctl is-active mysql 2>/dev/null")
		fmt.Printf("MySQL status: %s\n", output)

		client.Close()
	}

	// 检查当前 etcd leader_info
	fmt.Println("\n========== 当前 etcd leader_info ==========")
	host := &installer.Host{
		IP:       "10.211.55.32",
		Port:     22,
		Username: "mngdb",
		Password: "wuhua@2013",
	}
	client, _ := installer.NewSSHClient(host)
	if client != nil {
		output, _ := client.Run("etcdctl get /mypatroni/mysql-cluster/leader_info --print-value-only")
		fmt.Printf("leader_info: %s\n", output)
		client.Close()
	}
}
