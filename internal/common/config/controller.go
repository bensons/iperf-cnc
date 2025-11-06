package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ControllerConfig represents the controller configuration
type ControllerConfig struct {
	Controller ControllerSettings `yaml:"controller"`
}

// ControllerSettings contains the controller operational settings
type ControllerSettings struct {
	Nodes        []NodeConfig           `yaml:"nodes"`
	TestProfiles map[string]TestProfile `yaml:"test_profiles"`
	Topology     TopologyConfig         `yaml:"topology"`
	Output       OutputConfig           `yaml:"output"`
	Concurrency  ConcurrencyConfig      `yaml:"concurrency"`
}

// NodeConfig represents a node in the cluster
type NodeConfig struct {
	Hostname string   `yaml:"hostname"`
	IP       string   `yaml:"ip"`
	Port     int      `yaml:"port"`
	ID       string   `yaml:"id,omitempty"` // Optional, defaults to hostname
	Tags     []string `yaml:"tags,omitempty"`
}

// TestProfile contains iperf3 test parameters
type TestProfile struct {
	Duration          int               `yaml:"duration"`
	Protocol          string            `yaml:"protocol,omitempty"` // "tcp" or "udp" (default: tcp)
	Bandwidth         string            `yaml:"bandwidth,omitempty"`
	WindowSize        string            `yaml:"window_size,omitempty"`
	Parallel          int               `yaml:"parallel"`
	Bidirectional     bool              `yaml:"bidirectional"`
	Reverse           bool              `yaml:"reverse"`
	BufferLength      int               `yaml:"buffer_length,omitempty"`
	CongestionControl string            `yaml:"congestion_control,omitempty"` // TCP only
	MSS               int               `yaml:"mss,omitempty"`                // TCP only
	NoDelay           bool              `yaml:"no_delay"`                     // TCP only
	TOS               int               `yaml:"tos,omitempty"`
	ZeroCopy          bool              `yaml:"zerocopy"`
	OmitSeconds       int               `yaml:"omit_seconds,omitempty"`
	ExtraFlags        map[string]string `yaml:"extra_flags,omitempty"`
}

// TopologyConfig defines the test topology
type TopologyConfig struct {
	Type           string             `yaml:"type"` // "full_mesh", "custom"
	DefaultProfile string             `yaml:"default_profile"`
	Overrides      []TopologyOverride `yaml:"overrides,omitempty"`
}

// TopologyOverride allows specific node pairs to use different profiles
type TopologyOverride struct {
	SourceNodes      []string `yaml:"source_nodes,omitempty"`
	DestinationNodes []string `yaml:"destination_nodes,omitempty"`
	Nodes            []string `yaml:"nodes,omitempty"` // For symmetric overrides
	Profile          string   `yaml:"profile"`
}

// OutputConfig defines output settings
type OutputConfig struct {
	JSONFile   string `yaml:"json_file"`
	CSVFile    string `yaml:"csv_file,omitempty"`
	SchemaFile string `yaml:"schema_file,omitempty"`
	Compress   bool   `yaml:"compress"`
}

// ConcurrencyConfig controls parallelism and batching
type ConcurrencyConfig struct {
	MaxConcurrentNodes   int `yaml:"max_concurrent_nodes"`
	MaxConcurrentTests   int `yaml:"max_concurrent_tests"`
	ClientStartBatchSize int `yaml:"client_start_batch_size"`
	ConnectionTimeout    int `yaml:"connection_timeout_seconds"`
	RPCTimeout           int `yaml:"rpc_timeout_seconds"`
}

// LoadControllerConfig loads controller configuration from a YAML file
func LoadControllerConfig(path string) (*ControllerConfig, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- Config file path is provided by user
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config ControllerConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// Validate checks if the controller configuration is valid
func (c *ControllerConfig) Validate() error {
	if len(c.Controller.Nodes) < 2 {
		return fmt.Errorf("at least 2 nodes are required")
	}

	// Validate nodes
	nodeIDs := make(map[string]bool)
	for i, node := range c.Controller.Nodes {
		if node.Hostname == "" {
			return fmt.Errorf("node[%d]: hostname cannot be empty", i)
		}
		if node.IP == "" {
			return fmt.Errorf("node[%d]: ip cannot be empty", i)
		}
		if node.Port < 1 || node.Port > 65535 {
			return fmt.Errorf("node[%d]: port must be between 1 and 65535", i)
		}

		// Check for duplicate IDs
		id := node.ID
		if id == "" {
			id = node.Hostname
		}
		if nodeIDs[id] {
			return fmt.Errorf("duplicate node ID: %s", id)
		}
		nodeIDs[id] = true
	}

	// Validate test profiles
	if len(c.Controller.TestProfiles) == 0 {
		return fmt.Errorf("at least one test profile is required")
	}

	for name, profile := range c.Controller.TestProfiles {
		if err := validateTestProfile(name, profile); err != nil {
			return err
		}
	}

	// Validate topology
	if c.Controller.Topology.Type == "" {
		return fmt.Errorf("topology type cannot be empty")
	}

	validTopologyTypes := map[string]bool{
		"full_mesh": true,
		"custom":    true,
	}
	if !validTopologyTypes[c.Controller.Topology.Type] {
		return fmt.Errorf("topology type must be one of: full_mesh, custom")
	}

	if c.Controller.Topology.DefaultProfile == "" {
		return fmt.Errorf("topology default_profile cannot be empty")
	}

	if _, exists := c.Controller.TestProfiles[c.Controller.Topology.DefaultProfile]; !exists {
		return fmt.Errorf("default_profile '%s' not found in test_profiles", c.Controller.Topology.DefaultProfile)
	}

	// Validate output
	if c.Controller.Output.JSONFile == "" {
		return fmt.Errorf("output json_file cannot be empty")
	}

	return nil
}

// validateTestProfile checks if a test profile is valid
func validateTestProfile(name string, profile TestProfile) error {
	if profile.Duration < 1 {
		return fmt.Errorf("profile '%s': duration must be at least 1 second", name)
	}

	if profile.Parallel < 1 {
		return fmt.Errorf("profile '%s': parallel must be at least 1", name)
	}

	return nil
}

// SetDefaults sets default values for unspecified configuration options
func (c *ControllerConfig) SetDefaults() {
	// Set node IDs to hostname if not specified
	for i := range c.Controller.Nodes {
		if c.Controller.Nodes[i].ID == "" {
			c.Controller.Nodes[i].ID = c.Controller.Nodes[i].Hostname
		}
		if c.Controller.Nodes[i].Port == 0 {
			c.Controller.Nodes[i].Port = 50051
		}
	}

	// Set concurrency defaults
	if c.Controller.Concurrency.MaxConcurrentNodes == 0 {
		c.Controller.Concurrency.MaxConcurrentNodes = 100
	}
	if c.Controller.Concurrency.MaxConcurrentTests == 0 {
		c.Controller.Concurrency.MaxConcurrentTests = 1000
	}
	if c.Controller.Concurrency.ClientStartBatchSize == 0 {
		c.Controller.Concurrency.ClientStartBatchSize = 50
	}
	if c.Controller.Concurrency.ConnectionTimeout == 0 {
		c.Controller.Concurrency.ConnectionTimeout = 10
	}
	if c.Controller.Concurrency.RPCTimeout == 0 {
		c.Controller.Concurrency.RPCTimeout = 60
	}
}
