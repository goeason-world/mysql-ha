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
		{"node-2", "10.211.55.33"},
		{"node-3", "10.211.55.34"},
	}

	for _, node := range nodes {
		fmt.Printf("\n========== Checking %s (%s) ==========\n", node.name, node.ip)

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
		defer client.Close()

		// Check if mypatroni is running
		fmt.Printf("\n--- Process Status ---\n")
		output, _ := client.Run("ps aux | grep mypatroni | grep -v grep")
		fmt.Println(output)

		// Check config file
		fmt.Printf("\n--- Config File ---\n")
		output, _ = client.Run("cat /etc/mypatroni/config.yaml 2>/dev/null || echo 'Config not found'")
		fmt.Println(output)

		// Find log file location
		fmt.Printf("\n--- Finding Log Files ---\n")
		output, _ = client.Run("find /var/log /opt/mysql-ha /tmp -name '*mypatroni*.log' -o -name '*patroni*.log' 2>/dev/null | head -5")
		fmt.Println(output)

		// Check mypatroni logs (try multiple locations)
		fmt.Printf("\n--- MyPatroni Logs (last 100 lines) ---\n")
		output, _ = client.Run("tail -100 /var/log/mypatroni/mypatroni.log 2>/dev/null || tail -100 /opt/mysql-ha/logs/mypatroni.log 2>/dev/null || tail -100 /tmp/mypatroni.log 2>/dev/null || journalctl -u mypatroni -n 100 --no-pager 2>/dev/null || echo 'No logs found'")
		fmt.Println(output)

		// Check MySQL replication status
		fmt.Printf("\n--- MySQL Replication Status ---\n")
		output, _ = client.Run("mysql -uroot -pRoot@123456 -e 'SHOW SLAVE STATUS\\G' 2>/dev/null | grep -E '(Master_Host|Master_Port|Slave_IO_Running|Slave_SQL_Running|Last_IO_Error)' || echo 'Not a slave or MySQL not accessible'")
		fmt.Println(output)

		// Check MySQL read_only status
		fmt.Printf("\n--- MySQL Read-Only Status ---\n")
		output, _ = client.Run("mysql -uroot -pRoot@123456 -e 'SELECT @@read_only, @@super_read_only' 2>/dev/null || echo 'MySQL not accessible'")
		fmt.Println(output)

		// Check etcd leader_info
		if node.name == "node-2" {
			fmt.Printf("\n--- Etcd Leader Info ---\n")
			output, _ = client.Run("ETCDCTL_API=3 /opt/mysql-ha/etcd/etcdctl --endpoints=http://localhost:2379 get /mysql-ha/mysql-cluster/leader_info 2>/dev/null || echo 'Etcd not accessible'")
			fmt.Println(output)

			fmt.Printf("\n--- Etcd All Keys ---\n")
			output, _ = client.Run("ETCDCTL_API=3 /opt/mysql-ha/etcd/etcdctl --endpoints=http://localhost:2379 get --prefix /mysql-ha 2>/dev/null | head -50")
			fmt.Println(output)
		}
	}
}
