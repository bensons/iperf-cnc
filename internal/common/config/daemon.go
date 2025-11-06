package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// DaemonConfig represents the daemon configuration
type DaemonConfig struct {
	Daemon DaemonSettings `yaml:"daemon"`
}

// DaemonSettings contains the daemon operational settings
type DaemonSettings struct {
	ListenPort    int           `yaml:"listen_port"`
	PortRange     PortRange     `yaml:"port_range"`
	MaxProcesses  int           `yaml:"max_processes"`
	CPUAffinity   bool          `yaml:"cpu_affinity"`
	LogLevel      string        `yaml:"log_level"`
	ResultDir     string        `yaml:"result_dir"`
	TimeoutConfig TimeoutConfig `yaml:"timeout"`
}

// PortRange defines the range of ports available for iperf3 servers
type PortRange struct {
	Start int `yaml:"start"`
	End   int `yaml:"end"`
}

// TimeoutConfig contains timeout settings for various operations
type TimeoutConfig struct {
	ProcessStart  int `yaml:"process_start_seconds"`
	ProcessStop   int `yaml:"process_stop_seconds"`
	TestExecution int `yaml:"test_execution_seconds"`
}

// LoadDaemonConfig loads daemon configuration from a YAML file
func LoadDaemonConfig(path string) (*DaemonConfig, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- Config file path is provided by user
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config DaemonConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// Validate checks if the daemon configuration is valid
func (c *DaemonConfig) Validate() error {
	if c.Daemon.ListenPort < 1 || c.Daemon.ListenPort > 65535 {
		return fmt.Errorf("listen_port must be between 1 and 65535")
	}

	if c.Daemon.PortRange.Start < 1 || c.Daemon.PortRange.Start > 65535 {
		return fmt.Errorf("port_range.start must be between 1 and 65535")
	}

	if c.Daemon.PortRange.End < 1 || c.Daemon.PortRange.End > 65535 {
		return fmt.Errorf("port_range.end must be between 1 and 65535")
	}

	if c.Daemon.PortRange.Start >= c.Daemon.PortRange.End {
		return fmt.Errorf("port_range.start must be less than port_range.end")
	}

	if c.Daemon.MaxProcesses < 1 {
		return fmt.Errorf("max_processes must be at least 1")
	}

	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLogLevels[c.Daemon.LogLevel] {
		return fmt.Errorf("log_level must be one of: debug, info, warn, error")
	}

	if c.Daemon.ResultDir == "" {
		return fmt.Errorf("result_dir cannot be empty")
	}

	return nil
}

// SetDefaults sets default values for unspecified configuration options
func (c *DaemonConfig) SetDefaults() {
	if c.Daemon.ListenPort == 0 {
		c.Daemon.ListenPort = 50051
	}

	if c.Daemon.PortRange.Start == 0 {
		c.Daemon.PortRange.Start = 5201
	}

	if c.Daemon.PortRange.End == 0 {
		c.Daemon.PortRange.End = 5400
	}

	if c.Daemon.MaxProcesses == 0 {
		c.Daemon.MaxProcesses = 200
	}

	if c.Daemon.LogLevel == "" {
		c.Daemon.LogLevel = "info"
	}

	if c.Daemon.ResultDir == "" {
		c.Daemon.ResultDir = "./results"
	}

	if c.Daemon.TimeoutConfig.ProcessStart == 0 {
		c.Daemon.TimeoutConfig.ProcessStart = 30
	}

	if c.Daemon.TimeoutConfig.ProcessStop == 0 {
		c.Daemon.TimeoutConfig.ProcessStop = 10
	}

	if c.Daemon.TimeoutConfig.TestExecution == 0 {
		c.Daemon.TimeoutConfig.TestExecution = 300
	}
}
