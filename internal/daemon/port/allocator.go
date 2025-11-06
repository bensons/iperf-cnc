package port

import (
	"fmt"
	"sync"
)

// Allocator manages port allocation for iperf3 servers
type Allocator struct {
	startPort      int
	endPort        int
	allocatedPorts map[int]bool
	portToTestID   map[int]string
	testIDToPort   map[string]int
	mu             sync.RWMutex
}

// NewAllocator creates a new port allocator
func NewAllocator(startPort, endPort int) (*Allocator, error) {
	if startPort < 1 || startPort > 65535 {
		return nil, fmt.Errorf("invalid start port: %d", startPort)
	}
	if endPort < 1 || endPort > 65535 {
		return nil, fmt.Errorf("invalid end port: %d", endPort)
	}
	if startPort >= endPort {
		return nil, fmt.Errorf("start port must be less than end port")
	}

	return &Allocator{
		startPort:      startPort,
		endPort:        endPort,
		allocatedPorts: make(map[int]bool),
		portToTestID:   make(map[int]string),
		testIDToPort:   make(map[string]int),
	}, nil
}

// AllocatePort allocates a port for a test
func (a *Allocator) AllocatePort(testID string) (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Check if test already has a port
	if port, exists := a.testIDToPort[testID]; exists {
		return port, nil
	}

	// Find first available port
	for port := a.startPort; port <= a.endPort; port++ {
		if !a.allocatedPorts[port] {
			a.allocatedPorts[port] = true
			a.portToTestID[port] = testID
			a.testIDToPort[testID] = port
			return port, nil
		}
	}

	return 0, fmt.Errorf("no available ports in range %d-%d", a.startPort, a.endPort)
}

// AllocatePorts allocates multiple ports
func (a *Allocator) AllocatePorts(count int) ([]int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	ports := make([]int, 0, count)

	for port := a.startPort; port <= a.endPort && len(ports) < count; port++ {
		if !a.allocatedPorts[port] {
			ports = append(ports, port)
		}
	}

	if len(ports) < count {
		return nil, fmt.Errorf("insufficient ports: need %d, found %d", count, len(ports))
	}

	// Actually allocate the ports
	for _, port := range ports {
		a.allocatedPorts[port] = true
	}

	return ports, nil
}

// ReleasePort releases a port by test ID
func (a *Allocator) ReleasePort(testID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	port, exists := a.testIDToPort[testID]
	if !exists {
		return fmt.Errorf("test %s has no allocated port", testID)
	}

	delete(a.allocatedPorts, port)
	delete(a.portToTestID, port)
	delete(a.testIDToPort, testID)

	return nil
}

// ReleasePortByNumber releases a port by its number
func (a *Allocator) ReleasePortByNumber(port int) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.allocatedPorts[port] {
		return fmt.Errorf("port %d is not allocated", port)
	}

	testID := a.portToTestID[port]
	delete(a.allocatedPorts, port)
	delete(a.portToTestID, port)
	delete(a.testIDToPort, testID)

	return nil
}

// ReleasePorts releases multiple ports
func (a *Allocator) ReleasePorts(ports []int) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, port := range ports {
		if a.allocatedPorts[port] {
			testID := a.portToTestID[port]
			delete(a.allocatedPorts, port)
			delete(a.portToTestID, port)
			delete(a.testIDToPort, testID)
		}
	}
}

// GetPortForTest returns the port allocated to a test
func (a *Allocator) GetPortForTest(testID string) (int, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	port, exists := a.testIDToPort[testID]
	return port, exists
}

// GetTestForPort returns the test ID for a port
func (a *Allocator) GetTestForPort(port int) (string, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	testID, exists := a.portToTestID[port]
	return testID, exists
}

// IsPortAllocated checks if a port is allocated
func (a *Allocator) IsPortAllocated(port int) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.allocatedPorts[port]
}

// GetAllocatedCount returns the number of allocated ports
func (a *Allocator) GetAllocatedCount() int {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return len(a.allocatedPorts)
}

// GetAvailableCount returns the number of available ports
func (a *Allocator) GetAvailableCount() int {
	a.mu.RLock()
	defer a.mu.RUnlock()

	totalPorts := a.endPort - a.startPort + 1
	return totalPorts - len(a.allocatedPorts)
}

// GetCapacity returns the total port capacity
func (a *Allocator) GetCapacity() int {
	return a.endPort - a.startPort + 1
}

// ReleaseAll releases all allocated ports
func (a *Allocator) ReleaseAll() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.allocatedPorts = make(map[int]bool)
	a.portToTestID = make(map[int]string)
	a.testIDToPort = make(map[string]int)
}

// GetAllocatedPorts returns a list of all allocated ports
func (a *Allocator) GetAllocatedPorts() []int {
	a.mu.RLock()
	defer a.mu.RUnlock()

	ports := make([]int, 0, len(a.allocatedPorts))
	for port := range a.allocatedPorts {
		ports = append(ports, port)
	}

	return ports
}
