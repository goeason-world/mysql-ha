// Package main is the entry point for MySQL HA Web Admin
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mysql-ha/internal/webapi"
)

func main() {
	addr := flag.String("addr", ":8888", "Web admin listen address")
	logFile := flag.String("log", "./logs/webadmin.log", "Log file path")
	flag.Parse()

	// 设置日志输出到文件和控制台
	if err := setupLogging(*logFile); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to setup logging: %v\n", err)
	}

	log.Printf("[INFO] ========================================")
	log.Printf("[INFO] Starting MySQL HA Web Admin on %s", *addr)
	log.Printf("[INFO] Log file: %s", *logFile)
	log.Printf("[INFO] ========================================")

	server := webapi.NewServer(*addr)

	// Signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("[INFO] Shutting down...")
		server.Stop(context.Background())
		os.Exit(0)
	}()

	if err := server.Start(); err != nil {
		log.Printf("[ERROR] Server error: %v", err)
		os.Exit(1)
	}
}

// setupLogging 设置日志输出到文件和控制台
func setupLogging(logPath string) error {
	// 创建日志目录
	logDir := "./logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// 打开日志文件（追加模式）
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	// 同时输出到文件和控制台
	multiWriter := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(multiWriter)
	log.SetFlags(log.Ldate | log.Ltime)

	// 记录启动时间
	log.Printf("[INFO] Log initialized at %s", time.Now().Format("2006-01-02 15:04:05"))

	return nil
}
