package main

import (
	"fmt"
	"log"
	"mysql-ha/internal/installer"
	"strings"
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

		// Check advertise_host in config
		fmt.Printf("\n--- Config advertise_host ---\n")
		output, _ := client.Run("grep advertise_host /etc/mypatroni/config.yaml")
		fmt.Println(output)

		// Check recent logs
		fmt.Printf("\n--- Recent Logs (last 30 lines) ---\n")
		output, _ = client.Run("tail -30 /var/log/mypatroni/mypatroni.log | grep -E '(leader|replica|replication|failover|promoting|ensureReplicaOf|configureAsReplica)'")
		fmt.Println(output)

		// Check MySQL replication status
		fmt.Printf("\n--- MySQL Replication Status ---\n")
		output, _ = client.Run("mysql -uroot -pRoot@123456 -e 'SHOW SLAVE STATUS\\G' 2>/dev/null | grep -E '(Master_Host|Master_Port|Slave_IO_Running|Slave_SQL_Running|Last_IO_Error|Seconds_Behind_Master)'")
		if strings.TrimSpace(output) == "" {
			fmt.Println("Not a slave (this is the master)")
		} else {
			fmt.Println(output)
		}

		// Check MySQL read_only status
		fmt.Printf("\n--- MySQL Read-Only Status ---\n")
		output, _ = client.Run("mysql -uroot -pRoot@123456 -e 'SELECT @@read_only as read_only, @@super_read_only as super_read_only' 2>/dev/null")
		fmt.Println(output)

		// Check etcd leader_info (only on node-2)
		if node.name == "node-2" {
			fmt.Printf("\n--- Etcd Leader Info ---\n")
			output, _ = client.Run("ETCDCTL_API=3 /opt/mysql-ha/etcd/etcdctl --endpoints=http://localhost:2379 get /mysql-ha/mysql-cluster/leader_info 2>/dev/null")
			fmt.Println(output)
		}
	}

	fmt.Println("\n========== Summary ==========")
	fmt.Println("Check if:")
	fmt.Println("1. node-2 is read_only=0 (master)")
	fmt.Println("2. node-3 is read_only=1 (slave)")
	fmt.Println("3. node-3 Master_Host should be 10.211.55.33")
	fmt.Println("4. node-3 Slave_IO_Running and Slave_SQL_Running should be Yes")
}
