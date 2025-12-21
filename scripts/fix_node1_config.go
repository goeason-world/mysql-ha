package main

import (
	"fmt"
	"log"
	"mysql-ha/internal/installer"
)

func main() {
	fmt.Println("========== Fixing node-1 (10.211.55.32) ==========")

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

	// Backup original config
	fmt.Println("Backing up original config...")
	_, err = client.RunWithSudo("cp /etc/mypatroni/config.yaml /etc/mypatroni/config.yaml.backup")
	if err != nil {
		log.Printf("Failed to backup config: %v\n", err)
	}

	// Add advertise_host to config after scope line
	fmt.Println("Adding advertise_host: 10.211.55.32 to config...")
	addAdvertiseCmd := `sed -i '/^scope:/a advertise_host: "10.211.55.32"' /etc/mypatroni/config.yaml`
	_, err = client.RunWithSudo(addAdvertiseCmd)
	if err != nil {
		log.Fatalf("Failed to add advertise_host: %v\n", err)
	}

	// Verify the change
	fmt.Println("Verifying config change...")
	output, _ := client.Run("head -10 /etc/mypatroni/config.yaml")
	fmt.Println(output)

	// Restart mypatroni service
	fmt.Println("Restarting mypatroni service...")
	_, err = client.RunWithSudo("systemctl restart mypatroni")
	if err != nil {
		log.Printf("Failed to restart service: %v\n", err)
	}

	// Wait and check status
	fmt.Println("Checking service status...")
	output, _ = client.Run("systemctl is-active mypatroni")
	fmt.Printf("Service status: %s\n", output)

	fmt.Println("✓ node-1 configuration fixed!")
}
