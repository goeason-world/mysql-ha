package main

import (
	"fmt"
	"log"
	"mysql-ha/internal/installer"
)

func main() {
	node := struct {
		name string
		ip   string
	}{"node-3", "10.211.55.34"}

	fmt.Printf("========== Checking MySQL on %s (%s) ==========\n", node.name, node.ip)

	host := &installer.Host{
		IP:       node.ip,
		Port:     22,
		Username: "mngdb",
		Password: "wuhua@2013",
	}

	client, err := installer.NewSSHClient(host)
	if err != nil {
		log.Fatalf("Failed to connect to %s: %v\n", node.name, err)
	}
	defer client.Close()

	// Check MySQL process
	fmt.Println("\n--- MySQL Process ---")
	output, _ := client.Run("ps aux | grep mysqld | grep -v grep")
	fmt.Printf("%s\n", output)

	// Check MySQL socket
	fmt.Println("\n--- MySQL Socket ---")
	output, _ = client.Run("ls -la /tmp/mysql.sock 2>&1")
	fmt.Printf("%s\n", output)

	// Use full path to mysql client
	mysqlCmd := "/opt/mysql-ha/mysql/bin/mysql"

	// Check SHOW SLAVE STATUS
	fmt.Println("\n--- SHOW SLAVE STATUS ---")
	output, err = client.Run(fmt.Sprintf("%s -uroot -pRoot@123456 --socket=/tmp/mysql.sock -e 'SHOW SLAVE STATUS\\G' 2>&1", mysqlCmd))
	fmt.Printf("%s\n", output)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	// Check read_only status
	fmt.Println("\n--- Read-Only Status ---")
	output, err = client.Run(fmt.Sprintf("%s -uroot -pRoot@123456 --socket=/tmp/mysql.sock -e 'SELECT @@read_only, @@super_read_only' 2>&1", mysqlCmd))
	fmt.Printf("%s\n", output)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
}
