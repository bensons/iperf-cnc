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
	Pairs       []*TestPair
	ServerPorts map[string][]int32     // nodeID -> ports
	ClientTests map[string][]*TestPair // nodeID -> test pairs
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

	// Allocate server ports - each node needs one port per incoming connection
	// For a full mesh with N nodes, each node receives N-1 incoming connections
	portCounter := int32(5201) // Starting port
	for _, node := range nodes {
		// Allocate N-1 ports for this node (one for each potential source)
		numPorts := len(nodes) - 1
		ports := make([]int32, numPorts)
		for i := 0; i < numPorts; i++ {
			ports[i] = portCounter
			portCounter++
		}
		topology.ServerPorts[node.ID] = ports
	}

	return topology, nil
}

// GenerateNodeTopologies creates per-node topology assignments
func GenerateNodeTopologies(topology *Topology) (map[string]*pb.TestTopology, error) {
	result := make(map[string]*pb.TestTopology)

	// Extract unique nodes from topology
	nodeIDs := make(map[string]bool)
	for _, pair := range topology.Pairs {
		nodeIDs[pair.Source.ID] = true
		nodeIDs[pair.Destination.ID] = true
	}

	// Initialize for all nodes
	for nodeID := range nodeIDs {
		result[nodeID] = &pb.TestTopology{
			ServerAssignments: make([]*pb.TestPair, 0),
			ClientAssignments: make([]*pb.TestPair, 0),
		}
	}

	// Build server assignments
	// Each destination node needs to map each source node to a unique port
	for nodeID, ports := range topology.ServerPorts {
		if len(ports) == 0 {
			continue
		}

		// Create a mapping from source node ID to port index
		// We need to assign ports consistently across server and client assignments
		sourceNodes := make([]string, 0)
		for _, pair := range topology.Pairs {
			if pair.Destination.ID == nodeID {
				sourceNodes = append(sourceNodes, pair.Source.ID)
			}
		}

		// Create source -> port mapping
		sourceToPort := make(map[string]int32)
		for i, sourceID := range sourceNodes {
			if i < len(ports) {
				sourceToPort[sourceID] = ports[i]
			}
		}

		// Find all tests where this node is the destination
		for _, pair := range topology.Pairs {
			if pair.Destination.ID == nodeID {
				port, exists := sourceToPort[pair.Source.ID]
				if !exists {
					return nil, fmt.Errorf("no port allocated for source %s -> dest %s", pair.Source.ID, nodeID)
				}

				result[nodeID].ServerAssignments = append(result[nodeID].ServerAssignments, &pb.TestPair{
					SourceId:        pair.Source.ID,
					DestinationId:   pair.Destination.ID,
					DestinationIp:   pair.Destination.IP,
					DestinationPort: port,
					Profile:         ConvertProfileToProto(pair.Profile),
				})
			}
		}
	}

	// Build client assignments
	// Each client needs to connect to the port assigned for its source->dest pair
	for nodeID, pairs := range topology.ClientTests {
		for _, pair := range pairs {
			// Get the server ports for the destination
			destPorts := topology.ServerPorts[pair.Destination.ID]
			if len(destPorts) == 0 {
				return nil, fmt.Errorf("no server port allocated for node %s", pair.Destination.ID)
			}

			// Find which port this source should use on the destination
			// We need to find the index of this source in the destination's source list
			sourceNodes := make([]string, 0)
			for _, p := range topology.Pairs {
				if p.Destination.ID == pair.Destination.ID {
					sourceNodes = append(sourceNodes, p.Source.ID)
				}
			}

			// Find the index of this source
			portIndex := -1
			for i, sourceID := range sourceNodes {
				if sourceID == pair.Source.ID {
					portIndex = i
					break
				}
			}

			if portIndex < 0 || portIndex >= len(destPorts) {
				return nil, fmt.Errorf("no port index found for source %s -> dest %s", pair.Source.ID, pair.Destination.ID)
			}

			result[nodeID].ClientAssignments = append(result[nodeID].ClientAssignments, &pb.TestPair{
				SourceId:        pair.Source.ID,
				DestinationId:   pair.Destination.ID,
				DestinationIp:   pair.Destination.IP,
				DestinationPort: destPorts[portIndex],
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

	// Convert protocol
	protocol := pb.Protocol_PROTOCOL_TCP // Default to TCP
	if profile.Protocol == models.ProtocolUDP {
		protocol = pb.Protocol_PROTOCOL_UDP
	}

	return &pb.TestProfile{
		Name:              profile.Name,
		DurationSeconds:   int32(profile.Duration), // #nosec G115 -- Duration is validated to be reasonable
		Protocol:          protocol,
		Bandwidth:         profile.Bandwidth,
		WindowSize:        profile.WindowSize,
		ParallelStreams:   int32(profile.Parallel), // #nosec G115 -- Parallel streams is validated to be reasonable
		Bidirectional:     profile.Bidirectional,
		Reverse:           profile.Reverse,
		BufferLength:      int32(profile.BufferLength), // #nosec G115 -- Buffer length is validated to be reasonable
		CongestionControl: profile.CongestionControl,
		Mss:               int32(profile.MSS), // #nosec G115 -- MSS is validated to be reasonable
		NoDelay:           profile.NoDelay,
		Tos:               int32(profile.TOS), // #nosec G115 -- TOS is validated to be reasonable
		Zerocopy:          profile.ZeroCopy,
		OmitSeconds:       int32(profile.OmitSeconds), // #nosec G115 -- Omit seconds is validated to be reasonable
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
