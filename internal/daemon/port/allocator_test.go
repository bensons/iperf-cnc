package port

import (
	"fmt"
	"testing"
)

func TestNewAllocator(t *testing.T) {
	tests := []struct {
		name      string
		startPort int
		endPort   int
		wantErr   bool
	}{
		{
			name:      "valid range",
			startPort: 5201,
			endPort:   5300,
			wantErr:   false,
		},
		{
			name:      "invalid start port (too low)",
			startPort: 0,
			endPort:   5300,
			wantErr:   true,
		},
		{
			name:      "invalid end port (too high)",
			startPort: 5201,
			endPort:   70000,
			wantErr:   true,
		},
		{
			name:      "start >= end",
			startPort: 5300,
			endPort:   5200,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allocator, err := NewAllocator(tt.startPort, tt.endPort)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewAllocator() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && allocator == nil {
				t.Error("NewAllocator() returned nil allocator")
			}
		})
	}
}

func TestAllocator_AllocatePort(t *testing.T) {
	allocator, err := NewAllocator(5201, 5205)
	if err != nil {
		t.Fatalf("Failed to create allocator: %v", err)
	}

	// Allocate first port
	port1, err := allocator.AllocatePort("test1")
	if err != nil {
		t.Errorf("AllocatePort() error = %v", err)
	}
	if port1 < 5201 || port1 > 5205 {
		t.Errorf("AllocatePort() port = %d, want range [5201-5205]", port1)
	}

	// Allocate same test ID should return same port
	port2, err := allocator.AllocatePort("test1")
	if err != nil {
		t.Errorf("AllocatePort() error = %v", err)
	}
	if port2 != port1 {
		t.Errorf("AllocatePort() for same test ID returned different port: %d != %d", port2, port1)
	}

	// Allocate different test ID
	port3, err := allocator.AllocatePort("test2")
	if err != nil {
		t.Errorf("AllocatePort() error = %v", err)
	}
	if port3 == port1 {
		t.Error("AllocatePort() returned same port for different test ID")
	}
}

func TestAllocator_AllocatePorts(t *testing.T) {
	allocator, err := NewAllocator(5201, 5205)
	if err != nil {
		t.Fatalf("Failed to create allocator: %v", err)
	}

	// Allocate 3 ports
	ports, err := allocator.AllocatePorts(3)
	if err != nil {
		t.Errorf("AllocatePorts() error = %v", err)
	}
	if len(ports) != 3 {
		t.Errorf("AllocatePorts() returned %d ports, want 3", len(ports))
	}

	// Check all ports are unique
	portMap := make(map[int]bool)
	for _, port := range ports {
		if portMap[port] {
			t.Errorf("AllocatePorts() returned duplicate port: %d", port)
		}
		portMap[port] = true
	}

	// Try to allocate more than available
	_, err = allocator.AllocatePorts(10)
	if err == nil {
		t.Error("AllocatePorts() should fail when requesting more ports than available")
	}
}

func TestAllocator_ReleasePort(t *testing.T) {
	allocator, err := NewAllocator(5201, 5205)
	if err != nil {
		t.Fatalf("Failed to create allocator: %v", err)
	}

	// Allocate a port
	port, err := allocator.AllocatePort("test1")
	if err != nil {
		t.Fatalf("AllocatePort() error = %v", err)
	}

	// Release it
	err = allocator.ReleasePort("test1")
	if err != nil {
		t.Errorf("ReleasePort() error = %v", err)
	}

	// Check it's released
	if allocator.IsPortAllocated(port) {
		t.Error("ReleasePort() did not release the port")
	}

	// Try to release non-existent test
	err = allocator.ReleasePort("nonexistent")
	if err == nil {
		t.Error("ReleasePort() should fail for non-existent test")
	}
}

func TestAllocator_GetCapacity(t *testing.T) {
	allocator, err := NewAllocator(5201, 5205)
	if err != nil {
		t.Fatalf("Failed to create allocator: %v", err)
	}

	capacity := allocator.GetCapacity()
	expectedCapacity := 5 // 5201-5205 inclusive
	if capacity != expectedCapacity {
		t.Errorf("GetCapacity() = %d, want %d", capacity, expectedCapacity)
	}
}

func TestAllocator_GetAvailableCount(t *testing.T) {
	allocator, err := NewAllocator(5201, 5205)
	if err != nil {
		t.Fatalf("Failed to create allocator: %v", err)
	}

	// Initially all available
	if allocator.GetAvailableCount() != 5 {
		t.Errorf("GetAvailableCount() = %d, want 5", allocator.GetAvailableCount())
	}

	// Allocate one
	_, err = allocator.AllocatePort("test1")
	if err != nil {
		t.Fatalf("AllocatePort() error = %v", err)
	}

	// Should have 4 available
	if allocator.GetAvailableCount() != 4 {
		t.Errorf("GetAvailableCount() = %d, want 4", allocator.GetAvailableCount())
	}

	// Release it
	allocator.ReleasePort("test1")

	// Should have 5 available again
	if allocator.GetAvailableCount() != 5 {
		t.Errorf("GetAvailableCount() = %d, want 5", allocator.GetAvailableCount())
	}
}

func TestAllocator_ReleaseAll(t *testing.T) {
	allocator, err := NewAllocator(5201, 5205)
	if err != nil {
		t.Fatalf("Failed to create allocator: %v", err)
	}

	// Allocate all ports
	for i := 0; i < 5; i++ {
		_, err := allocator.AllocatePort(fmt.Sprintf("test%d", i))
		if err != nil {
			t.Fatalf("AllocatePort() error = %v", err)
		}
	}

	// Verify all allocated
	if allocator.GetAllocatedCount() != 5 {
		t.Errorf("GetAllocatedCount() = %d, want 5", allocator.GetAllocatedCount())
	}

	// Release all
	allocator.ReleaseAll()

	// Verify all released
	if allocator.GetAllocatedCount() != 0 {
		t.Errorf("GetAllocatedCount() = %d, want 0 after ReleaseAll()", allocator.GetAllocatedCount())
	}
	if allocator.GetAvailableCount() != 5 {
		t.Errorf("GetAvailableCount() = %d, want 5 after ReleaseAll()", allocator.GetAvailableCount())
	}
}

func TestAllocator_ConcurrentAllocation(t *testing.T) {
	allocator, err := NewAllocator(5201, 5250)
	if err != nil {
		t.Fatalf("Failed to create allocator: %v", err)
	}

	// Allocate ports concurrently
	const numGoroutines = 10
	results := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			testID := fmt.Sprintf("test%d", id)
			_, err := allocator.AllocatePort(testID)
			results <- err
		}(i)
	}

	// Collect results
	for i := 0; i < numGoroutines; i++ {
		err := <-results
		if err != nil {
			t.Errorf("Concurrent allocation failed: %v", err)
		}
	}

	// Verify correct number allocated
	if allocator.GetAllocatedCount() != numGoroutines {
		t.Errorf("GetAllocatedCount() = %d, want %d", allocator.GetAllocatedCount(), numGoroutines)
	}
}
