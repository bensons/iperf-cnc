package iperf

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

// Mode represents iperf3 operation mode
type Mode string

const (
	// ModeServer runs iperf3 in server mode
	ModeServer Mode = "server"
	// ModeClient runs iperf3 in client mode
	ModeClient Mode = "client"
)

// Config contains iperf3 execution configuration
type Config struct {
	Mode              Mode
	Port              int
	Host              string // For client mode
	Duration          int
	Bandwidth         string
	WindowSize        string
	Parallel          int
	Bidirectional     bool
	Reverse           bool
	BufferLength      int
	CongestionControl string
	MSS               int
	NoDelay           bool
	TOS               int
	ZeroCopy          bool
	OmitSeconds       int
	ExtraArgs         []string
}

// Result contains iperf3 execution result
type Result struct {
	Success    bool
	JSONOutput string
	ExitCode   int
	Error      string
	StartTime  time.Time
	EndTime    time.Time
	Duration   time.Duration
}

// Wrapper wraps iperf3 command execution
type Wrapper struct {
	iperfPath string
}

// NewWrapper creates a new iperf3 wrapper
func NewWrapper(iperfPath string) *Wrapper {
	if iperfPath == "" {
		iperfPath = "iperf3"
	}
	return &Wrapper{
		iperfPath: iperfPath,
	}
}

// BuildCommand builds the iperf3 command arguments
func (w *Wrapper) BuildCommand(config *Config) ([]string, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	args := make([]string, 0)

	// Mode
	switch config.Mode {
	case ModeServer:
		args = append(args, "-s")
		args = append(args, "-p", fmt.Sprintf("%d", config.Port))
	case ModeClient:
		if config.Host == "" {
			return nil, fmt.Errorf("host is required for client mode")
		}
		args = append(args, "-c", config.Host)
		args = append(args, "-p", fmt.Sprintf("%d", config.Port))

		// Duration (only for client)
		if config.Duration > 0 {
			args = append(args, "-t", fmt.Sprintf("%d", config.Duration))
		}

		// Bandwidth
		if config.Bandwidth != "" {
			args = append(args, "-b", config.Bandwidth)
		}

		// Parallel streams
		if config.Parallel > 1 {
			args = append(args, "-P", fmt.Sprintf("%d", config.Parallel))
		}

		// Bidirectional
		if config.Bidirectional {
			args = append(args, "--bidir")
		}

		// Reverse
		if config.Reverse {
			args = append(args, "-R")
		}

		// Buffer length
		if config.BufferLength > 0 {
			args = append(args, "-l", fmt.Sprintf("%d", config.BufferLength))
		}

		// Congestion control
		if config.CongestionControl != "" {
			args = append(args, "-C", config.CongestionControl)
		}

		// MSS
		if config.MSS > 0 {
			args = append(args, "-M", fmt.Sprintf("%d", config.MSS))
		}

		// No delay
		if config.NoDelay {
			args = append(args, "-N")
		}

		// TOS
		if config.TOS > 0 {
			args = append(args, "-S", fmt.Sprintf("%d", config.TOS))
		}

		// Zero copy
		if config.ZeroCopy {
			args = append(args, "-Z")
		}

		// Omit seconds
		if config.OmitSeconds > 0 {
			args = append(args, "-O", fmt.Sprintf("%d", config.OmitSeconds))
		}

	default:
		return nil, fmt.Errorf("invalid mode: %s", config.Mode)
	}

	// Window size (both modes)
	if config.WindowSize != "" {
		args = append(args, "-w", config.WindowSize)
	}

	// JSON output
	args = append(args, "-J")

	// One-off mode for servers
	if config.Mode == ModeServer {
		args = append(args, "-1")
	}

	// Extra arguments
	if len(config.ExtraArgs) > 0 {
		args = append(args, config.ExtraArgs...)
	}

	return args, nil
}

// Run executes iperf3 with the given configuration
func (w *Wrapper) Run(ctx context.Context, config *Config) (*Result, error) {
	args, err := w.BuildCommand(config)
	if err != nil {
		return nil, fmt.Errorf("failed to build command: %w", err)
	}

	result := &Result{
		StartTime: time.Now(),
	}

	cmd := exec.CommandContext(ctx, w.iperfPath, args...) // #nosec G204 -- iperf3 path is controlled, args are validated

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
		}
		result.Success = false
		result.Error = fmt.Sprintf("iperf3 failed: %v, stderr: %s", err, stderr.String())
		return result, nil
	}

	result.Success = true
	result.ExitCode = 0
	result.JSONOutput = stdout.String()

	// Validate JSON output
	if config.Mode == ModeClient {
		if err := validateJSON(result.JSONOutput); err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("invalid JSON output: %v", err)
		}
	}

	return result, nil
}

// RunServer starts an iperf3 server that runs until context is cancelled
func (w *Wrapper) RunServer(ctx context.Context, port int) (*exec.Cmd, error) {
	args := []string{
		"-s",
		"-p", fmt.Sprintf("%d", port),
		"-J",
	}

	cmd := exec.CommandContext(ctx, w.iperfPath, args...) // #nosec G204 -- iperf3 path is controlled, args are validated

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start iperf3 server: %w", err)
	}

	return cmd, nil
}

// validateJSON checks if the output is valid JSON
func validateJSON(output string) error {
	var js map[string]interface{}
	if err := json.Unmarshal([]byte(output), &js); err != nil {
		return err
	}
	return nil
}

// ParseResult parses iperf3 JSON output into a structured format
func ParseResult(jsonOutput string) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(jsonOutput), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
	return result, nil
}

// ExtractThroughput extracts throughput information from parsed result
func ExtractThroughput(result map[string]interface{}) (float64, string, error) {
	end, ok := result["end"].(map[string]interface{})
	if !ok {
		return 0, "", fmt.Errorf("missing 'end' section in result")
	}

	sumSent, ok := end["sum_sent"].(map[string]interface{})
	if !ok {
		return 0, "", fmt.Errorf("missing 'sum_sent' section in result")
	}

	bitsPerSecond, ok := sumSent["bits_per_second"].(float64)
	if !ok {
		return 0, "", fmt.Errorf("missing 'bits_per_second' in result")
	}

	// Convert to human-readable format
	var throughput float64
	var unit string

	if bitsPerSecond >= 1e9 {
		throughput = bitsPerSecond / 1e9
		unit = "Gbps"
	} else if bitsPerSecond >= 1e6 {
		throughput = bitsPerSecond / 1e6
		unit = "Mbps"
	} else if bitsPerSecond >= 1e3 {
		throughput = bitsPerSecond / 1e3
		unit = "Kbps"
	} else {
		throughput = bitsPerSecond
		unit = "bps"
	}

	return throughput, unit, nil
}
