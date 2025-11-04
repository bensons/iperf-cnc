package topology

import (
	"fmt"

	pb "github.com/bensons/iperf-cnc/api/proto"
	"github.com/bensons/iperf-cnc/internal/common/models"
)

// TestPair represents a source-destination test pair
type TestPair struct {
	TestID      string
	Source      *models.Node
	Destination *models.Node
	Profile     *models.TestProfile
}

// Topology represents the complete test topology
type Topology struct {
	Pairs          []*TestPair
	ServerPorts    map[string][]int32 // nodeID -> ports
	ClientTests    map[string][]*TestPair // nodeID -> test pairs
}

// Generator generates test topologies
type Generator struct {
	nodes          *models.NodeRegistry
	profiles       *models.ProfileRegistry
	defaultProfile *models.TestProfile
	overrides      map[string]string // nodePairKey -> profileName
}

// NewGenerator creates a new topology generator
func NewGenerator(nodes *models.NodeRegistry, profiles *models.ProfileRegistry, defaultProfile *models.TestProfile) *Generator {
	return &Generator{
		nodes:          nodes,
		profiles:       profiles,
		defaultProfile: defaultProfile,
		overrides:      make(map[string]string),
	}
}

// AddOverride adds a profile override for specific node pairs
func (g *Generator) AddOverride(sourceID, destID, profileName string) error {
	key := fmt.Sprintf("%s:%s", sourceID, destID)
	g.overrides[key] = profileName
	return nil
}

// GenerateFullMesh generates a full mesh topology
func (g *Generator) GenerateFullMesh() (*Topology, error) {
	nodes := g.nodes.GetAllNodes()
	if len(nodes) < 2 {
		return nil, fmt.Errorf("at least 2 nodes required for mesh topology")
	}

	topology := &Topology{
		Pairs:       make([]*TestPair, 0),
		ServerPorts: make(map[string][]int32),
		ClientTests: make(map[string][]*TestPair),
	}

	testCounter := 0

	// Generate all source-destination pairs
	for _, source := range nodes {
		for _, dest := range nodes {
			// Skip self-tests
			if source.ID == dest.ID {
				continue
			}

			// Get profile for this pair
			profile := g.getProfileForPair(source.ID, dest.ID)

			testCounter++
			testID := fmt.Sprintf("test-%d-%s-to-%s", testCounter, source.ID, dest.ID)

			pair := &TestPair{
				TestID:      testID,
				Source:      source,
				Destination: dest,
				Profile:     profile,
			}

			topology.Pairs = append(topology.Pairs, pair)

			// Track client tests by source
			topology.ClientTests[source.ID] = append(topology.ClientTests[source.ID], pair)
		}
	}

	// Allocate server ports (one per node for all incoming tests)
	portCounter := int32(5201) // Starting port
	for _, node := range nodes {
		// Each node needs to run a server for incoming tests
		topology.ServerPorts[node.ID] = []int32{portCounter}
		portCounter++
	}

	return topology, nil
}

// GenerateNodeTopologies creates per-node topology assignments
func (g *Generator) GenerateNodeTopologies(topology *Topology) (map[string]*pb.TestTopology, error) {
	result := make(map[string]*pb.TestTopology)

	// Initialize for all nodes
	for _, node := range g.nodes.GetAllNodes() {
		result[node.ID] = &pb.TestTopology{
			ServerAssignments: make([]*pb.TestPair, 0),
			ClientAssignments: make([]*pb.TestPair, 0),
		}
	}

	// Build server assignments
	for nodeID, ports := range topology.ServerPorts {
		if len(ports) == 0 {
			continue
		}

		// For now, each node runs one server on its allocated port
		port := ports[0]

		// Find all tests where this node is the destination
		for _, pair := range topology.Pairs {
			if pair.Destination.ID == nodeID {
				result[nodeID].ServerAssignments = append(result[nodeID].ServerAssignments, &pb.TestPair{
					SourceId:      pair.Source.ID,
					DestinationId: pair.Destination.ID,
					DestinationIp: pair.Destination.IP,
					DestinationPort: port,
					Profile:       ConvertProfileToProto(pair.Profile),
				})
			}
		}
	}

	// Build client assignments
	for nodeID, pairs := range topology.ClientTests {
		for _, pair := range pairs {
			// Get the server port for the destination
			destPorts := topology.ServerPorts[pair.Destination.ID]
			if len(destPorts) == 0 {
				return nil, fmt.Errorf("no server port allocated for node %s", pair.Destination.ID)
			}

			result[nodeID].ClientAssignments = append(result[nodeID].ClientAssignments, &pb.TestPair{
				SourceId:        pair.Source.ID,
				DestinationId:   pair.Destination.ID,
				DestinationIp:   pair.Destination.IP,
				DestinationPort: destPorts[0],
				Profile:         ConvertProfileToProto(pair.Profile),
			})
		}
	}

	return result, nil
}

// getProfileForPair gets the profile for a node pair
func (g *Generator) getProfileForPair(sourceID, destID string) *models.TestProfile {
	key := fmt.Sprintf("%s:%s", sourceID, destID)

	if profileName, exists := g.overrides[key]; exists {
		if profile, err := g.profiles.GetProfile(profileName); err == nil {
			return profile
		}
	}

	return g.defaultProfile
}

// ConvertProfileToProto converts a model TestProfile to protobuf
func ConvertProfileToProto(profile *models.TestProfile) *pb.TestProfile {
	if profile == nil {
		return &pb.TestProfile{}
	}

	return &pb.TestProfile{
		Name:              profile.Name,
		DurationSeconds:   int32(profile.Duration),
		Bandwidth:         profile.Bandwidth,
		WindowSize:        profile.WindowSize,
		ParallelStreams:   int32(profile.Parallel),
		Bidirectional:     profile.Bidirectional,
		Reverse:           profile.Reverse,
		BufferLength:      int32(profile.BufferLength),
		CongestionControl: profile.CongestionControl,
		Mss:               int32(profile.MSS),
		NoDelay:           profile.NoDelay,
		Tos:               int32(profile.TOS),
		Zerocopy:          profile.ZeroCopy,
		OmitSeconds:       int32(profile.OmitSeconds),
	}
}

// GetTestCount returns the total number of tests in the topology
func (t *Topology) GetTestCount() int {
	return len(t.Pairs)
}

// GetServerCount returns the number of servers to start
func (t *Topology) GetServerCount() int {
	return len(t.ServerPorts)
}

// GetClientCount returns the number of client tests to run
func (t *Topology) GetClientCount() int {
	count := 0
	for _, tests := range t.ClientTests {
		count += len(tests)
	}
	return count
}
