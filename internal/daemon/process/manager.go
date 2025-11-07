package process

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/bensons/iperf-cnc/internal/common/iperf"
	"github.com/bensons/iperf-cnc/internal/daemon/collector"
	"github.com/bensons/iperf-cnc/internal/daemon/port"
)

// ProcessInfo contains information about a running process
type ProcessInfo struct {
	TestID    string
	PID       int
	Port      int
	Mode      iperf.Mode
	StartTime time.Time
	Cmd       *exec.Cmd
	Cancel    context.CancelFunc
}

// Manager manages iperf3 processes
type Manager struct {
	portAllocator *port.Allocator
	capacity      *CapacityCalculator
	iperf         *iperf.Wrapper
	collector     *collector.Collector
	processes     map[string]*ProcessInfo // testID -> ProcessInfo
	servers       map[int]*ProcessInfo    // port -> ProcessInfo for servers
	mu            sync.RWMutex
	iperfPath     string
	saveResults   bool   // Whether to save iperf3 results to files
	resultDir     string // Directory for result files
}

// NewManager creates a new process manager
func NewManager(portAllocator *port.Allocator, capacity *CapacityCalculator, resultCollector *collector.Collector, iperfPath string) *Manager {
	return &Manager{
		portAllocator: portAllocator,
		capacity:      capacity,
		iperf:         iperf.NewWrapper(iperfPath),
		collector:     resultCollector,
		processes:     make(map[string]*ProcessInfo),
		servers:       make(map[int]*ProcessInfo),
		iperfPath:     iperfPath,
		saveResults:   false,
		resultDir:     "",
	}
}

// SetSaveResults configures whether to save iperf3 results to files
func (m *Manager) SetSaveResults(save bool, resultDir string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.saveResults = save
	m.resultDir = resultDir
}

// StartServer starts an iperf3 server on the specified port
func (m *Manager) StartServer(port int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if server already running on this port
	if _, exists := m.servers[port]; exists {
		return fmt.Errorf("server already running on port %d", port)
	}

	// Reserve capacity
	if err := m.capacity.ReserveSlots(1); err != nil {
		return fmt.Errorf("insufficient capacity: %w", err)
	}

	// Create context for the server
	ctx, cancel := context.WithCancel(context.Background())

	// Generate logfile path if saving is enabled
	var logFile string
	if m.saveResults {
		logFile = m.generateLogFilePath(fmt.Sprintf("server-%d", port))
	}

	// Start the server
	cmd, err := m.iperf.RunServer(ctx, port, logFile)
	if err != nil {
		m.capacity.ReleaseSlots(1)
		cancel()
		return fmt.Errorf("failed to start server: %w", err)
	}

	// Create process info
	processInfo := &ProcessInfo{
		TestID:    fmt.Sprintf("server-%d", port),
		PID:       cmd.Process.Pid,
		Port:      port,
		Mode:      iperf.ModeServer,
		StartTime: time.Now(),
		Cmd:       cmd,
		Cancel:    cancel,
	}

	m.servers[port] = processInfo
	m.processes[processInfo.TestID] = processInfo

	// Monitor server in background
	go m.monitorProcess(processInfo)

	return nil
}

// StartClient starts an iperf3 client test
func (m *Manager) StartClient(testID, host string, port int, config *iperf.Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if test already running
	if _, exists := m.processes[testID]; exists {
		return fmt.Errorf("test %s already running", testID)
	}

	// Reserve capacity
	if err := m.capacity.ReserveSlots(1); err != nil {
		return fmt.Errorf("insufficient capacity: %w", err)
	}

	// Set required fields
	config.Mode = iperf.ModeClient
	config.Host = host
	config.Port = port

	// Set logfile if saving is enabled
	if m.saveResults {
		config.LogFile = m.generateLogFilePath(testID)
	}

	// Create context with timeout
	timeout := time.Duration(config.Duration+30) * time.Second // Add buffer
	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	// Create process info
	processInfo := &ProcessInfo{
		TestID:    testID,
		Port:      port,
		Mode:      iperf.ModeClient,
		StartTime: time.Now(),
		Cancel:    cancel,
	}

	m.processes[testID] = processInfo

	// Run client in background
	go m.runClient(ctx, processInfo, config)

	return nil
}

// StopProcess stops a specific process
func (m *Manager) StopProcess(testID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	processInfo, exists := m.processes[testID]
	if !exists {
		return fmt.Errorf("process %s not found", testID)
	}

	// Cancel context to stop process
	if processInfo.Cancel != nil {
		processInfo.Cancel()
	}

	// If it's a server, also remove from servers map
	if processInfo.Mode == iperf.ModeServer {
		delete(m.servers, processInfo.Port)
	}

	delete(m.processes, testID)
	m.capacity.ReleaseSlots(1)

	return nil
}

// StopAllServers stops all running servers
func (m *Manager) StopAllServers() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for port, processInfo := range m.servers {
		if processInfo.Cancel != nil {
			processInfo.Cancel()
		}
		delete(m.servers, port)
		delete(m.processes, processInfo.TestID)
		m.capacity.ReleaseSlots(1)
		count++
	}

	return count
}

// StopAllClients stops all running clients
func (m *Manager) StopAllClients() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for testID, processInfo := range m.processes {
		if processInfo.Mode == iperf.ModeClient {
			if processInfo.Cancel != nil {
				processInfo.Cancel()
			}
			delete(m.processes, testID)
			m.capacity.ReleaseSlots(1)
			count++
		}
	}

	return count
}

// StopAll stops all running processes
func (m *Manager) StopAll() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := len(m.processes)

	for _, processInfo := range m.processes {
		if processInfo.Cancel != nil {
			processInfo.Cancel()
		}
	}

	m.processes = make(map[string]*ProcessInfo)
	m.servers = make(map[int]*ProcessInfo)
	m.capacity.ReleaseSlots(count)

	return count
}

// GetProcessInfo returns information about a process
func (m *Manager) GetProcessInfo(testID string) (*ProcessInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	processInfo, exists := m.processes[testID]
	if !exists {
		return nil, fmt.Errorf("process %s not found", testID)
	}

	return processInfo, nil
}

// GetRunningCount returns the number of running processes
func (m *Manager) GetRunningCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.processes)
}

// GetServerCount returns the number of running servers
func (m *Manager) GetServerCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.servers)
}

// IsServerRunning checks if a server is running on a port
func (m *Manager) IsServerRunning(port int) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.servers[port]
	return exists
}

// monitorProcess monitors a process and cleans up when it exits
func (m *Manager) monitorProcess(processInfo *ProcessInfo) {
	if processInfo.Cmd != nil {
		_ = processInfo.Cmd.Wait()
	}

	// Clean up
	m.mu.Lock()
	defer m.mu.Unlock()

	if processInfo.Mode == iperf.ModeServer {
		delete(m.servers, processInfo.Port)
	}
	delete(m.processes, processInfo.TestID)
	m.capacity.ReleaseSlots(1)
}

// runClient runs an iperf3 client test
func (m *Manager) runClient(ctx context.Context, processInfo *ProcessInfo, config *iperf.Config) {
	result, err := m.iperf.Run(ctx, config)

	// Store result in collector
	if m.collector != nil {
		if err != nil {
			// Store error result
			_ = m.collector.StoreIperfResult(processInfo.TestID, &iperf.Result{
				Success:    false,
				Error:      err.Error(),
				StartTime:  processInfo.StartTime,
				EndTime:    time.Now(),
				ExitCode:   -1,
				JSONOutput: "",
			})
		} else if result != nil {
			// Store successful result
			_ = m.collector.StoreIperfResult(processInfo.TestID, result)
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Clean up
	delete(m.processes, processInfo.TestID)
	m.capacity.ReleaseSlots(1)
}

// generateLogFilePath generates a unique log file path for a test
func (m *Manager) generateLogFilePath(testID string) string {
	// Use result directory if configured, otherwise current directory
	dir := m.resultDir
	if dir == "" {
		dir = "."
	}

	// Generate filename with timestamp and test ID
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("%s/iperf3_%s_%s.json", dir, testID, timestamp)

	return filename
}
