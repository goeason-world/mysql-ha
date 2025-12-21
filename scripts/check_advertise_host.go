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

		// Check config file
		fmt.Println("--- Config file content (first 20 lines) ---")
		output, _ := client.Run("head -20 /etc/mypatroni/config.yaml")
		fmt.Println(output)

		// Check advertise_host specifically
		fmt.Println("--- advertise_host line ---")
		output, _ = client.Run("grep -n advertise_host /etc/mypatroni/config.yaml")
		if output == "" {
			fmt.Println("WARNING: advertise_host NOT FOUND in config!")
		} else {
			fmt.Println(output)
		}

		// Check mypatroni service status
		fmt.Println("--- mypatroni service status ---")
		output, _ = client.Run("systemctl is-active mypatroni")
		fmt.Printf("Service status: %s\n", output)

		// Check recent logs for advertise_host
		fmt.Println("--- Recent logs (looking for advertise_host) ---")
		output, _ = client.Run("journalctl -u mypatroni --no-pager -n 20 2>/dev/null | grep -i 'advertise\\|config\\|host' || tail -10 /var/log/mypatroni/mypatroni.log 2>/dev/null | grep -i 'advertise\\|config\\|host'")
		fmt.Println(output)

		client.Close()
	}
}
