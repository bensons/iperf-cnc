package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"

	pb "github.com/bensons/iperf-cnc/api/proto"
	"github.com/bensons/iperf-cnc/internal/common/config"
	"github.com/bensons/iperf-cnc/internal/daemon/server"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if err := newRootCommand().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	var configPath string

	rootCmd := &cobra.Command{
		Use:   "iperf-daemon",
		Short: "iperf-cnc daemon for managing iperf3 processes",
		Long: `iperf-daemon is a gRPC server that manages iperf3 processes
for distributed network performance testing.`,
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDaemon(configPath)
		},
	}

	rootCmd.Flags().StringVarP(&configPath, "config", "c", "./daemon.yaml",
		"path to configuration file")

	return rootCmd
}

func runDaemon(configPath string) error {
	fmt.Printf("iperf-daemon version %s\n", version)
	fmt.Printf("Loading configuration from: %s\n", configPath)

	// Load configuration
	cfg, err := config.LoadDaemonConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Set defaults
	cfg.SetDefaults()

	// Create daemon server
	serverConfig := &server.Config{
		ListenPort:     cfg.Daemon.ListenPort,
		PortRangeStart: cfg.Daemon.PortRange.Start,
		PortRangeEnd:   cfg.Daemon.PortRange.End,
		MaxProcesses:   cfg.Daemon.MaxProcesses,
		CPUAffinity:    cfg.Daemon.CPUAffinity,
		LogLevel:       cfg.Daemon.LogLevel,
		ResultDir:      cfg.Daemon.ResultDir,
		IperfPath:      "iperf3",
	}

	daemonServer, err := server.NewDaemonServer(serverConfig)
	if err != nil {
		return fmt.Errorf("failed to create daemon server: %w", err)
	}

	// Create gRPC server
	grpcServer := grpc.NewServer()
	pb.RegisterDaemonServiceServer(grpcServer, daemonServer)

	// Start listening
	listenAddr := fmt.Sprintf(":%d", cfg.Daemon.ListenPort)
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", listenAddr, err)
	}

	fmt.Printf("Daemon listening on %s\n", listenAddr)
	fmt.Printf("Port range: %d-%d\n", cfg.Daemon.PortRange.Start, cfg.Daemon.PortRange.End)
	fmt.Printf("Max processes: %d\n", cfg.Daemon.MaxProcesses)

	// Handle graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Println("Shutting down gracefully...")
		grpcServer.GracefulStop()
	}()

	// Start serving
	if err := grpcServer.Serve(listener); err != nil {
		return fmt.Errorf("failed to serve: %w", err)
	}

	return nil
}
