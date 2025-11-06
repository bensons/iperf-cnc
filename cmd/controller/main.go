package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/bensons/iperf-cnc/internal/common/config"
	"github.com/bensons/iperf-cnc/internal/common/models"
	"github.com/bensons/iperf-cnc/internal/controller/aggregator"
	"github.com/bensons/iperf-cnc/internal/controller/client"
	"github.com/bensons/iperf-cnc/internal/controller/orchestrator"
	"github.com/bensons/iperf-cnc/internal/controller/output"
	"github.com/bensons/iperf-cnc/internal/controller/topology"
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
	rootCmd := &cobra.Command{
		Use:   "iperf-controller",
		Short: "iperf-cnc controller for orchestrating distributed tests",
		Long: `iperf-controller orchestrates distributed iperf3 network performance tests
across a cluster of nodes running iperf-daemon.`,
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date),
	}

	rootCmd.AddCommand(newRunCommand())
	rootCmd.AddCommand(newValidateCommand())
	rootCmd.AddCommand(newStatusCommand())

	return rootCmd
}

func newRunCommand() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run a test based on configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTest(configPath)
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", "./controller.yaml",
		"path to configuration file")
	if err := cmd.MarkFlagRequired("config"); err != nil {
		panic(err) // This should never happen during initialization
	}

	return cmd
}

func newValidateCommand() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration file",
		RunE: func(cmd *cobra.Command, args []string) error {
			return validateConfig(configPath)
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", "./controller.yaml",
		"path to configuration file")
	if err := cmd.MarkFlagRequired("config"); err != nil {
		panic(err) // This should never happen during initialization
	}

	return cmd
}

func newStatusCommand() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check status of all configured nodes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return checkStatus(configPath)
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", "./controller.yaml",
		"path to configuration file")
	if err := cmd.MarkFlagRequired("config"); err != nil {
		panic(err) // This should never happen during initialization
	}

	return cmd
}

func runTest(configPath string) error {
	fmt.Printf("iperf-controller version %s\n", version)
	fmt.Printf("Loading configuration from: %s\n\n", configPath)

	// Load configuration
	cfg, err := config.LoadControllerConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	cfg.SetDefaults()

	// Build node registry
	nodeRegistry := models.NewNodeRegistry()
	for _, nodeConfig := range cfg.Controller.Nodes {
		node := &models.Node{
			ID:       nodeConfig.ID,
			Hostname: nodeConfig.Hostname,
			IP:       nodeConfig.IP,
			Port:     nodeConfig.Port,
			Tags:     nodeConfig.Tags,
		}
		if addErr := nodeRegistry.AddNode(node); addErr != nil {
			return fmt.Errorf("failed to add node: %w", addErr)
		}
	}

	log.Printf("Loaded %d nodes from configuration", nodeRegistry.Count())

	// Build profile registry
	profileRegistry := models.NewProfileRegistry()
	for name, profileConfig := range cfg.Controller.TestProfiles {
		// Convert protocol string to Protocol type
		protocol := models.ProtocolTCP // Default to TCP
		if profileConfig.Protocol == "udp" {
			protocol = models.ProtocolUDP
		}

		profile := &models.TestProfile{
			Name:              name,
			Duration:          profileConfig.Duration,
			Protocol:          protocol,
			Bandwidth:         profileConfig.Bandwidth,
			WindowSize:        profileConfig.WindowSize,
			Parallel:          profileConfig.Parallel,
			Bidirectional:     profileConfig.Bidirectional,
			Reverse:           profileConfig.Reverse,
			BufferLength:      profileConfig.BufferLength,
			CongestionControl: profileConfig.CongestionControl,
			MSS:               profileConfig.MSS,
			NoDelay:           profileConfig.NoDelay,
			TOS:               profileConfig.TOS,
			ZeroCopy:          profileConfig.ZeroCopy,
			OmitSeconds:       profileConfig.OmitSeconds,
		}
		if addErr := profileRegistry.AddProfile(profile); addErr != nil {
			return fmt.Errorf("failed to add profile: %w", addErr)
		}
	}

	log.Printf("Loaded %d test profiles", len(cfg.Controller.TestProfiles))

	// Get default profile
	defaultProfile, err := profileRegistry.GetProfile(cfg.Controller.Topology.DefaultProfile)
	if err != nil {
		return fmt.Errorf("failed to get default profile: %w", err)
	}

	// Create client pool and connect
	ctx := context.Background()
	timeout := time.Duration(cfg.Controller.Concurrency.ConnectionTimeout) * time.Second
	pool := client.NewPool(timeout)

	log.Println("Connecting to daemons...")
	nodes := nodeRegistry.GetAllNodes()
	if connErr := pool.ConnectAll(ctx, nodes); connErr != nil {
		return fmt.Errorf("failed to connect to daemons: %w", connErr)
	}
	defer func() {
		if closeErr := pool.Close(); closeErr != nil {
			log.Printf("Warning: failed to close connection pool: %v", closeErr)
		}
	}()

	log.Printf("Connected to %d daemons\n", pool.Count())

	// Generate topology
	log.Println("Generating test topology...")
	topoGen := topology.NewGenerator(nodeRegistry, profileRegistry, defaultProfile)

	// Apply overrides from config
	for _, override := range cfg.Controller.Topology.Overrides {
		// For now, simple implementation: if "nodes" is specified, apply to all pairs
		if len(override.Nodes) >= 2 {
			for i, src := range override.Nodes {
				for j, dst := range override.Nodes {
					if i != j {
						if overrideErr := topoGen.AddOverride(src, dst, override.Profile); overrideErr != nil {
							return fmt.Errorf("failed to add topology override: %w", overrideErr)
						}
					}
				}
			}
		}
	}

	topo, err := topoGen.GenerateFullMesh()
	if err != nil {
		return fmt.Errorf("failed to generate topology: %w", err)
	}

	log.Printf("Generated topology: %d test pairs\n", topo.GetTestCount())

	// Execute test
	log.Println("\nStarting test execution...")
	orch := orchestrator.NewOrchestrator(pool)
	if err := orch.ExecuteTest(ctx, topo); err != nil {
		return fmt.Errorf("test execution failed: %w", err)
	}

	// Collect and aggregate results
	log.Println("\nAggregating results...")
	agg := aggregator.NewAggregator()
	if err := agg.CollectResults(ctx, pool); err != nil {
		return fmt.Errorf("failed to collect results: %w", err)
	}

	results := agg.GetResults()
	summary := agg.GetSummary()

	log.Printf("Collected %d results", len(results))
	log.Printf("Completed: %d, Failed: %d", summary.CompletedTests, summary.FailedTests)

	// Write outputs
	log.Println("\nWriting output files...")
	writer := output.NewWriter(cfg.Controller.Output.JSONFile, cfg.Controller.Output.CSVFile)
	if err := writer.WriteAll(summary, results); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	if cfg.Controller.Output.JSONFile != "" {
		log.Printf("JSON output: %s", cfg.Controller.Output.JSONFile)
	}
	if cfg.Controller.Output.CSVFile != "" {
		log.Printf("CSV output: %s", cfg.Controller.Output.CSVFile)
	}

	fmt.Println("\n✓ Test complete!")
	fmt.Printf("  Total tests: %d\n", summary.TotalTests)
	fmt.Printf("  Completed: %d\n", summary.CompletedTests)
	fmt.Printf("  Failed: %d\n", summary.FailedTests)
	if summary.AvgThroughput > 0 {
		fmt.Printf("  Avg throughput: %.2f Gbps\n", summary.AvgThroughput/1e9)
	}

	return nil
}

func validateConfig(configPath string) error {
	fmt.Printf("Validating configuration: %s\n", configPath)

	cfg, err := config.LoadControllerConfig(configPath)
	if err != nil {
		return fmt.Errorf("❌ Configuration invalid: %w", err)
	}

	cfg.SetDefaults()

	// Additional validation
	if len(cfg.Controller.Nodes) < 2 {
		return fmt.Errorf("❌ At least 2 nodes are required")
	}

	fmt.Println("✓ Configuration is valid")
	fmt.Printf("  Nodes: %d\n", len(cfg.Controller.Nodes))
	fmt.Printf("  Profiles: %d\n", len(cfg.Controller.TestProfiles))
	fmt.Printf("  Default profile: %s\n", cfg.Controller.Topology.DefaultProfile)
	fmt.Printf("  Topology type: %s\n", cfg.Controller.Topology.Type)

	return nil
}

func checkStatus(configPath string) error {
	fmt.Printf("Checking node status from: %s\n\n", configPath)

	// Load configuration
	cfg, err := config.LoadControllerConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	cfg.SetDefaults()

	// Build node registry
	nodeRegistry := models.NewNodeRegistry()
	for _, nodeConfig := range cfg.Controller.Nodes {
		node := &models.Node{
			ID:       nodeConfig.ID,
			Hostname: nodeConfig.Hostname,
			IP:       nodeConfig.IP,
			Port:     nodeConfig.Port,
		}
		if addErr := nodeRegistry.AddNode(node); addErr != nil {
			return fmt.Errorf("failed to add node: %w", addErr)
		}
	}

	// Create client pool and connect
	ctx := context.Background()
	timeout := 5 * time.Second
	pool := client.NewPool(timeout)

	nodes := nodeRegistry.GetAllNodes()
	if connErr := pool.ConnectAll(ctx, nodes); connErr != nil {
		log.Printf("Warning: %v", connErr)
	}
	defer func() {
		if closeErr := pool.Close(); closeErr != nil {
			log.Printf("Warning: failed to close connection pool: %v", closeErr)
		}
	}()

	// Check health
	statuses, err := pool.CheckHealth(ctx)
	if err != nil {
		log.Printf("Warning: %v", err)
	}

	// Display results
	fmt.Println("Node Status:")
	fmt.Println(strings.Repeat("-", 80))

	for _, node := range nodes {
		status, exists := statuses[node.ID]
		if !exists {
			fmt.Printf("%-20s  %s\n", node.ID, "❌ OFFLINE")
			continue
		}

		healthSymbol := "✓"
		if !status.Healthy {
			healthSymbol = "❌"
		}

		fmt.Printf("%-20s  %s ONLINE\n", node.ID, healthSymbol)
		fmt.Printf("  Running processes: %d\n", status.RunningProcesses)
		fmt.Printf("  Completed tests: %d\n", status.CompletedTests)
		fmt.Printf("  Failed tests: %d\n", status.FailedTests)
		fmt.Printf("  Available capacity: %d/%d\n",
			status.CurrentCapacity.AvailableProcesses,
			status.CurrentCapacity.MaxProcesses)
		fmt.Printf("  Uptime: %d seconds\n", status.UptimeSeconds)
		fmt.Println()
	}

	return nil
}
