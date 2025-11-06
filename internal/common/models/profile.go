package models

import (
	"fmt"
	"strings"
)

// Protocol represents the transport protocol for iperf3 tests
type Protocol string

const (
	// ProtocolTCP represents TCP protocol
	ProtocolTCP Protocol = "tcp"
	// ProtocolUDP represents UDP protocol
	ProtocolUDP Protocol = "udp"
)

// TestProfile contains all iperf3 parameters for a test
type TestProfile struct {
	Name              string
	Duration          int
	Protocol          Protocol // TCP or UDP (default: TCP)
	Bandwidth         string
	WindowSize        string
	Parallel          int
	Bidirectional     bool
	Reverse           bool
	BufferLength      int
	CongestionControl string // TCP only
	MSS               int    // TCP only
	NoDelay           bool   // TCP only
	TOS               int
	ZeroCopy          bool
	OmitSeconds       int
	ExtraFlags        map[string]string
}

// ProfileRegistry manages test profiles
type ProfileRegistry struct {
	profiles map[string]*TestProfile
}

// NewProfileRegistry creates a new profile registry
func NewProfileRegistry() *ProfileRegistry {
	return &ProfileRegistry{
		profiles: make(map[string]*TestProfile),
	}
}

// AddProfile adds a profile to the registry
func (r *ProfileRegistry) AddProfile(profile *TestProfile) error {
	if profile.Name == "" {
		return fmt.Errorf("profile name cannot be empty")
	}

	if _, exists := r.profiles[profile.Name]; exists {
		return fmt.Errorf("profile %s already exists", profile.Name)
	}

	r.profiles[profile.Name] = profile
	return nil
}

// GetProfile retrieves a profile by name
func (r *ProfileRegistry) GetProfile(name string) (*TestProfile, error) {
	profile, exists := r.profiles[name]
	if !exists {
		return nil, fmt.Errorf("profile %s not found", name)
	}
	return profile, nil
}

// GetAllProfiles returns all registered profiles
func (r *ProfileRegistry) GetAllProfiles() map[string]*TestProfile {
	return r.profiles
}

// Clone creates a deep copy of the test profile
func (p *TestProfile) Clone() *TestProfile {
	clone := &TestProfile{
		Name:              p.Name,
		Duration:          p.Duration,
		Protocol:          p.Protocol,
		Bandwidth:         p.Bandwidth,
		WindowSize:        p.WindowSize,
		Parallel:          p.Parallel,
		Bidirectional:     p.Bidirectional,
		Reverse:           p.Reverse,
		BufferLength:      p.BufferLength,
		CongestionControl: p.CongestionControl,
		MSS:               p.MSS,
		NoDelay:           p.NoDelay,
		TOS:               p.TOS,
		ZeroCopy:          p.ZeroCopy,
		OmitSeconds:       p.OmitSeconds,
	}

	if p.ExtraFlags != nil {
		clone.ExtraFlags = make(map[string]string)
		for k, v := range p.ExtraFlags {
			clone.ExtraFlags[k] = v
		}
	}

	return clone
}

// ToCommandArgs converts the profile to iperf3 command line arguments
func (p *TestProfile) ToCommandArgs() []string {
	args := make([]string, 0)

	// Protocol (UDP requires -u flag, TCP is default)
	if p.Protocol == ProtocolUDP {
		args = append(args, "-u")
	}

	// Duration
	args = append(args, "-t", fmt.Sprintf("%d", p.Duration))

	// Bandwidth
	if p.Bandwidth != "" && p.Bandwidth != "0" {
		args = append(args, "-b", p.Bandwidth)
	}

	// Window size
	if p.WindowSize != "" {
		args = append(args, "-w", p.WindowSize)
	}

	// Parallel streams
	if p.Parallel > 1 {
		args = append(args, "-P", fmt.Sprintf("%d", p.Parallel))
	}

	// Bidirectional
	if p.Bidirectional {
		args = append(args, "--bidir")
	}

	// Reverse
	if p.Reverse {
		args = append(args, "-R")
	}

	// Buffer length
	if p.BufferLength > 0 {
		args = append(args, "-l", fmt.Sprintf("%d", p.BufferLength))
	}

	// Congestion control (TCP only)
	if p.Protocol != ProtocolUDP && p.CongestionControl != "" {
		args = append(args, "-C", p.CongestionControl)
	}

	// MSS (TCP only)
	if p.Protocol != ProtocolUDP && p.MSS > 0 {
		args = append(args, "-M", fmt.Sprintf("%d", p.MSS))
	}

	// No delay (TCP only)
	if p.Protocol != ProtocolUDP && p.NoDelay {
		args = append(args, "-N")
	}

	// TOS
	if p.TOS > 0 {
		args = append(args, "-S", fmt.Sprintf("%d", p.TOS))
	}

	// Zero copy
	if p.ZeroCopy {
		args = append(args, "-Z")
	}

	// Omit seconds
	if p.OmitSeconds > 0 {
		args = append(args, "-O", fmt.Sprintf("%d", p.OmitSeconds))
	}

	// JSON output
	args = append(args, "-J")

	// Extra flags
	if p.ExtraFlags != nil {
		for flag, value := range p.ExtraFlags {
			if value == "" {
				args = append(args, flag)
			} else {
				args = append(args, flag, value)
			}
		}
	}

	return args
}

// String returns a string representation of the profile
func (p *TestProfile) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Profile{Name: %s, Duration: %ds", p.Name, p.Duration))

	if p.Bandwidth != "" {
		sb.WriteString(fmt.Sprintf(", Bandwidth: %s", p.Bandwidth))
	}

	if p.Parallel > 1 {
		sb.WriteString(fmt.Sprintf(", Parallel: %d", p.Parallel))
	}

	if p.Bidirectional {
		sb.WriteString(", Bidirectional")
	}

	sb.WriteString("}")
	return sb.String()
}

// Validate checks if the profile is valid
func (p *TestProfile) Validate() error {
	if p.Duration < 1 {
		return fmt.Errorf("duration must be at least 1 second")
	}

	if p.Parallel < 1 {
		return fmt.Errorf("parallel must be at least 1")
	}

	return nil
}
