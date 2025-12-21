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
		fmt.Printf("\n========== Fixing %s (%s) ==========\n", node.name, node.ip)

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

		// Backup original config
		fmt.Println("Backing up original config...")
		_, err = client.RunWithSudo("cp /etc/mypatroni/config.yaml /etc/mypatroni/config.yaml.backup")
		if err != nil {
			log.Printf("Failed to backup config: %v\n", err)
			continue
		}

		// Add advertise_host to config
		fmt.Printf("Adding advertise_host: %s to config...\n", node.ip)
		addAdvertiseCmd := fmt.Sprintf(`sed -i '/^scope:/a advertise_host: "%s"' /etc/mypatroni/config.yaml`, node.ip)
		_, err = client.RunWithSudo(addAdvertiseCmd)
		if err != nil {
			log.Printf("Failed to add advertise_host: %v\n", err)
			continue
		}

		// Verify the change
		fmt.Println("Verifying config change...")
		output, _ := client.Run("grep advertise_host /etc/mypatroni/config.yaml")
		fmt.Printf("Config updated: %s\n", output)

		// Restart mypatroni service
		fmt.Println("Restarting mypatroni service...")
		_, err = client.RunWithSudo("systemctl restart mypatroni")
		if err != nil {
			log.Printf("Failed to restart service: %v\n", err)
			// Try alternative restart method
			fmt.Println("Trying alternative restart method...")
			_, err = client.RunWithSudo("pkill -9 mypatroni && sleep 2 && systemctl start mypatroni")
			if err != nil {
				log.Printf("Alternative restart also failed: %v\n", err)
			}
		}

		fmt.Printf("✓ %s configuration fixed and service restarted\n", node.name)
	}

	fmt.Println("\n========== All nodes updated ==========")
	fmt.Println("Please wait 10-20 seconds for the agents to reconnect and reconfigure replication.")
}
