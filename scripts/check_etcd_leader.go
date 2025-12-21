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

	fmt.Println("========== etcd leader_info ==========")
	output, _ := client.Run("etcdctl get /mypatroni/mysql-cluster/leader_info --print-value-only")
	fmt.Printf("leader_info: %s\n", output)

	fmt.Println("\n========== etcd lock holder ==========")
	output, _ = client.Run("etcdctl get /mypatroni/mysql-cluster --prefix --keys-only")
	fmt.Printf("Keys: %s\n", output)

	fmt.Println("\n========== All agent states ==========")
	for _, ip := range []string{"10.211.55.32", "10.211.55.33", "10.211.55.34"} {
		output, _ := client.Run(fmt.Sprintf("curl -s http://%s:8080/state 2>/dev/null || echo 'unreachable'", ip))
		fmt.Printf("%s: %s\n", ip, output)
	}
}
