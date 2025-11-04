package models

import (
	"fmt"
)

// Node represents a node in the cluster
type Node struct {
	ID       string
	Hostname string
	IP       string
	Port     int
	Capacity ProcessCapacity
	Tags     []string
}

// ProcessCapacity represents a node's ability to run processes
type ProcessCapacity struct {
	MaxProcesses       int
	AvailableProcesses int
	CPUCores           int
	AvailableMemory    int64
	NetworkInterfaces  []string
}

// NodeRegistry manages a collection of nodes
type NodeRegistry struct {
	nodes    map[string]*Node
	nodeList []*Node
}

// NewNodeRegistry creates a new node registry
func NewNodeRegistry() *NodeRegistry {
	return &NodeRegistry{
		nodes:    make(map[string]*Node),
		nodeList: make([]*Node, 0),
	}
}

// AddNode adds a node to the registry
func (r *NodeRegistry) AddNode(node *Node) error {
	if node.ID == "" {
		return fmt.Errorf("node ID cannot be empty")
	}

	if _, exists := r.nodes[node.ID]; exists {
		return fmt.Errorf("node with ID %s already exists", node.ID)
	}

	r.nodes[node.ID] = node
	r.nodeList = append(r.nodeList, node)
	return nil
}

// GetNode retrieves a node by ID
func (r *NodeRegistry) GetNode(id string) (*Node, error) {
	node, exists := r.nodes[id]
	if !exists {
		return nil, fmt.Errorf("node with ID %s not found", id)
	}
	return node, nil
}

// GetAllNodes returns all registered nodes
func (r *NodeRegistry) GetAllNodes() []*Node {
	return r.nodeList
}

// Count returns the number of registered nodes
func (r *NodeRegistry) Count() int {
	return len(r.nodeList)
}

// GetNodesByTag returns all nodes with a specific tag
func (r *NodeRegistry) GetNodesByTag(tag string) []*Node {
	result := make([]*Node, 0)
	for _, node := range r.nodeList {
		for _, nodeTag := range node.Tags {
			if nodeTag == tag {
				result = append(result, node)
				break
			}
		}
	}
	return result
}

// String returns a string representation of the node
func (n *Node) String() string {
	return fmt.Sprintf("Node{ID: %s, Hostname: %s, IP: %s, Port: %d}",
		n.ID, n.Hostname, n.IP, n.Port)
}

// HasTag checks if a node has a specific tag
func (n *Node) HasTag(tag string) bool {
	for _, t := range n.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

// Address returns the network address of the node
func (n *Node) Address() string {
	return fmt.Sprintf("%s:%d", n.IP, n.Port)
}
