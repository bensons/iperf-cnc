package models

import (
	"fmt"
)

// NodePair represents a source-destination pair
type NodePair struct {
	Source      *Node
	Destination *Node
}

// TestMatrix contains the full test topology and profile assignments
type TestMatrix struct {
	DefaultProfile *TestProfile
	NodePairs      map[NodePair]*TestProfile
	Nodes          *NodeRegistry
}

// TestAssignment represents a test to be executed
type TestAssignment struct {
	ID          string
	Source      *Node
	Destination *Node
	Profile     *TestProfile
}

// NewTestMatrix creates a new test matrix
func NewTestMatrix(defaultProfile *TestProfile, nodes *NodeRegistry) *TestMatrix {
	return &TestMatrix{
		DefaultProfile: defaultProfile,
		NodePairs:      make(map[NodePair]*TestProfile),
		Nodes:          nodes,
	}
}

// SetPairProfile sets a specific profile for a node pair
func (m *TestMatrix) SetPairProfile(source, destination *Node, profile *TestProfile) {
	pair := NodePair{Source: source, Destination: destination}
	m.NodePairs[pair] = profile
}

// GetPairProfile gets the profile for a node pair, returns default if not set
func (m *TestMatrix) GetPairProfile(source, destination *Node) *TestProfile {
	pair := NodePair{Source: source, Destination: destination}
	if profile, exists := m.NodePairs[pair]; exists {
		return profile
	}
	return m.DefaultProfile
}

// GenerateFullMesh creates test assignments for a full mesh topology
func (m *TestMatrix) GenerateFullMesh() []*TestAssignment {
	assignments := make([]*TestAssignment, 0)
	nodes := m.Nodes.GetAllNodes()

	testID := 0
	for _, source := range nodes {
		for _, dest := range nodes {
			// Skip self-tests
			if source.ID == dest.ID {
				continue
			}

			profile := m.GetPairProfile(source, dest)
			testID++

			assignment := &TestAssignment{
				ID:          fmt.Sprintf("test-%d", testID),
				Source:      source,
				Destination: dest,
				Profile:     profile,
			}

			assignments = append(assignments, assignment)

			// If bidirectional, create reverse test
			if profile.Bidirectional {
				testID++
				reverseAssignment := &TestAssignment{
					ID:          fmt.Sprintf("test-%d", testID),
					Source:      dest,
					Destination: source,
					Profile:     profile,
				}
				assignments = append(assignments, reverseAssignment)
			}
		}
	}

	return assignments
}

// CountTests returns the total number of tests in the matrix
func (m *TestMatrix) CountTests() int {
	assignments := m.GenerateFullMesh()
	return len(assignments)
}

// GroupAssignmentsBySource groups test assignments by source node
func (m *TestMatrix) GroupAssignmentsBySource() map[string][]*TestAssignment {
	assignments := m.GenerateFullMesh()
	grouped := make(map[string][]*TestAssignment)

	for _, assignment := range assignments {
		sourceID := assignment.Source.ID
		grouped[sourceID] = append(grouped[sourceID], assignment)
	}

	return grouped
}

// GroupAssignmentsByDestination groups test assignments by destination node
func (m *TestMatrix) GroupAssignmentsByDestination() map[string][]*TestAssignment {
	assignments := m.GenerateFullMesh()
	grouped := make(map[string][]*TestAssignment)

	for _, assignment := range assignments {
		destID := assignment.Destination.ID
		grouped[destID] = append(grouped[destID], assignment)
	}

	return grouped
}

// String returns a string representation of the node pair
func (p NodePair) String() string {
	return fmt.Sprintf("%s -> %s", p.Source.ID, p.Destination.ID)
}

// String returns a string representation of the test assignment
func (a *TestAssignment) String() string {
	return fmt.Sprintf("TestAssignment{ID: %s, %s -> %s, Profile: %s}",
		a.ID, a.Source.ID, a.Destination.ID, a.Profile.Name)
}

// Equals checks if two node pairs are equal
func (p NodePair) Equals(other NodePair) bool {
	return p.Source.ID == other.Source.ID && p.Destination.ID == other.Destination.ID
}

// Key returns a unique string key for the node pair (for use in maps)
func (p NodePair) Key() string {
	return fmt.Sprintf("%s:%s", p.Source.ID, p.Destination.ID)
}
