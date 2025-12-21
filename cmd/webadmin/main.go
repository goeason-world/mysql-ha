// Package main is the entry point for MySQL HA Web Admin
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"mysql-ha/internal/webapi"
)

func main() {
	addr := flag.String("addr", ":8888", "Web admin listen address")
	flag.Parse()

	fmt.Printf("Starting MySQL HA Web Admin on %s\n", *addr)

	server := webapi.NewServer(*addr)

	// Signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\nShutting down...")
		server.Stop(context.Background())
		os.Exit(0)
	}()

	if err := server.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
