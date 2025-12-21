// Package main is the entry point for MySQL HA Agent
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"mysql-ha/internal/agent"
	"mysql-ha/internal/api"
	"mysql-ha/internal/config"
	"mysql-ha/internal/dcs"
	"mysql-ha/internal/log"
	"mysql-ha/internal/mysql"
)

var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	configPath := flag.String("config", "/etc/mypatroni/config.yaml", "Path to configuration file")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *showVersion {
		fmt.Printf("MyPatroni %s (built %s)\n", version, buildTime)
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logger, err := log.New(log.Config{
		Level:  cfg.Log.Level,
		NodeID: cfg.Name,
		File:   cfg.Log.File,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	// 打印关键配置信息用于调试
	logger.Info(fmt.Sprintf("Config loaded: name=%s, scope=%s, advertise_host=%s, mysql_host=%s",
		cfg.Name, cfg.Scope, cfg.AdvertiseHost, cfg.MySQL.Host))

	// Initialize DCS
	etcdDCS := dcs.NewEtcdDCS(dcs.EtcdConfig{
		Endpoints: cfg.DCS.Endpoints,
		Username:  cfg.DCS.Username,
		Password:  cfg.DCS.Password,
		Prefix:    "/" + cfg.Namespace + "/" + cfg.Scope,
	})

	// Initialize MySQL manager
	mysqlMgr := mysql.NewManager(mysql.Config{
		Host:            cfg.MySQL.Host,
		Port:            cfg.MySQL.Port,
		User:            cfg.MySQL.User,
		Password:        cfg.MySQL.Password,
		ReplicationUser: cfg.MySQL.ReplicationUser,
		ReplicationPass: cfg.MySQL.ReplicationPass,
	})

	// Create agent
	haAgent := agent.NewAgent(cfg, etcdDCS, mysqlMgr, logger)

	// Create API server
	apiServer := api.NewServer(api.Config{
		Listen: cfg.API.Listen,
		Port:   cfg.API.Port,
		APIKey: cfg.API.APIKey,
	}, haAgent, logger)

	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	go func() {
		for sig := range sigCh {
			switch sig {
			case syscall.SIGINT, syscall.SIGTERM:
				logger.Info("Received shutdown signal")
				cancel()
			case syscall.SIGHUP:
				logger.Info("Received SIGHUP, reloading configuration")
				// TODO: Implement config reload
			}
		}
	}()

	// Start agent
	if err := haAgent.Start(ctx); err != nil {
		logger.Error(fmt.Sprintf("Failed to start agent: %v", err))
		os.Exit(1)
	}

	// Start API server in background
	go func() {
		if err := apiServer.Start(); err != nil {
			logger.Error("API server error")
		}
	}()

	// Wait for shutdown
	<-ctx.Done()

	// Graceful shutdown
	logger.Info("Shutting down...")
	apiServer.Stop(context.Background())
	haAgent.Stop()

	logger.Info("Goodbye!")
}
