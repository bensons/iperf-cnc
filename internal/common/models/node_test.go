package models

import (
	"testing"
)

func TestNewNodeRegistry(t *testing.T) {
	registry := NewNodeRegistry()
	if registry == nil {
		t.Fatal("NewNodeRegistry() returned nil")
	}
	if registry.Count() != 0 {
		t.Errorf("NewNodeRegistry() count = %d, want 0", registry.Count())
	}
}

func TestNodeRegistry_AddNode(t *testing.T) {
	registry := NewNodeRegistry()

	node := &Node{
		ID:       "node1",
		Hostname: "node1.example.com",
		IP:       "192.168.1.10",
		Port:     50051,
	}

	err := registry.AddNode(node)
	if err != nil {
		t.Errorf("AddNode() error = %v", err)
	}

	if registry.Count() != 1 {
		t.Errorf("Count() = %d, want 1", registry.Count())
	}

	// Try to add duplicate
	err = registry.AddNode(node)
	if err == nil {
		t.Error("AddNode() should fail for duplicate node ID")
	}

	// Try to add node with empty ID
	badNode := &Node{
		ID:       "",
		Hostname: "node2.example.com",
	}
	err = registry.AddNode(badNode)
	if err == nil {
		t.Error("AddNode() should fail for empty node ID")
	}
}

func TestNodeRegistry_GetNode(t *testing.T) {
	registry := NewNodeRegistry()

	node := &Node{
		ID:       "node1",
		Hostname: "node1.example.com",
		IP:       "192.168.1.10",
		Port:     50051,
	}

	registry.AddNode(node)

	// Get existing node
	retrieved, err := registry.GetNode("node1")
	if err != nil {
		t.Errorf("GetNode() error = %v", err)
	}
	if retrieved.ID != node.ID {
		t.Errorf("GetNode() ID = %s, want %s", retrieved.ID, node.ID)
	}

	// Get non-existent node
	_, err = registry.GetNode("nonexistent")
	if err == nil {
		t.Error("GetNode() should fail for non-existent node")
	}
}

func TestNodeRegistry_GetAllNodes(t *testing.T) {
	registry := NewNodeRegistry()

	node1 := &Node{ID: "node1", Hostname: "node1.example.com", IP: "192.168.1.10", Port: 50051}
	node2 := &Node{ID: "node2", Hostname: "node2.example.com", IP: "192.168.1.11", Port: 50051}
	node3 := &Node{ID: "node3", Hostname: "node3.example.com", IP: "192.168.1.12", Port: 50051}

	registry.AddNode(node1)
	registry.AddNode(node2)
	registry.AddNode(node3)

	nodes := registry.GetAllNodes()
	if len(nodes) != 3 {
		t.Errorf("GetAllNodes() returned %d nodes, want 3", len(nodes))
	}
}

func TestNodeRegistry_GetNodesByTag(t *testing.T) {
	registry := NewNodeRegistry()

	node1 := &Node{ID: "node1", Hostname: "node1.example.com", IP: "192.168.1.10", Port: 50051, Tags: []string{"us-west", "prod"}}
	node2 := &Node{ID: "node2", Hostname: "node2.example.com", IP: "192.168.1.11", Port: 50051, Tags: []string{"us-east", "prod"}}
	node3 := &Node{ID: "node3", Hostname: "node3.example.com", IP: "192.168.1.12", Port: 50051, Tags: []string{"us-west", "dev"}}

	registry.AddNode(node1)
	registry.AddNode(node2)
	registry.AddNode(node3)

	// Get nodes with "us-west" tag
	westNodes := registry.GetNodesByTag("us-west")
	if len(westNodes) != 2 {
		t.Errorf("GetNodesByTag('us-west') returned %d nodes, want 2", len(westNodes))
	}

	// Get nodes with "prod" tag
	prodNodes := registry.GetNodesByTag("prod")
	if len(prodNodes) != 2 {
		t.Errorf("GetNodesByTag('prod') returned %d nodes, want 2", len(prodNodes))
	}

	// Get nodes with non-existent tag
	noNodes := registry.GetNodesByTag("nonexistent")
	if len(noNodes) != 0 {
		t.Errorf("GetNodesByTag('nonexistent') returned %d nodes, want 0", len(noNodes))
	}
}

func TestNode_HasTag(t *testing.T) {
	node := &Node{
		ID:   "node1",
		Tags: []string{"prod", "us-west"},
	}

	if !node.HasTag("prod") {
		t.Error("HasTag('prod') should return true")
	}

	if node.HasTag("dev") {
		t.Error("HasTag('dev') should return false")
	}
}

func TestNode_Address(t *testing.T) {
	node := &Node{
		ID:   "node1",
		IP:   "192.168.1.10",
		Port: 50051,
	}

	expected := "192.168.1.10:50051"
	if node.Address() != expected {
		t.Errorf("Address() = %s, want %s", node.Address(), expected)
	}
}

func TestNode_String(t *testing.T) {
	node := &Node{
		ID:       "node1",
		Hostname: "node1.example.com",
		IP:       "192.168.1.10",
		Port:     50051,
	}

	str := node.String()
	if str == "" {
		t.Error("String() returned empty string")
	}

	// Check that string contains key information
	if !contains(str, "node1") || !contains(str, "192.168.1.10") {
		t.Errorf("String() = %s, should contain node ID and IP", str)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
