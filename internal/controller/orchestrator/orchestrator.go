package orchestrator

import (
	"context"
	"fmt"
	"log"
	"time"

	pb "github.com/bensons/iperf-cnc/api/proto"
	"github.com/bensons/iperf-cnc/internal/controller/client"
	"github.com/bensons/iperf-cnc/internal/controller/topology"
)

// TestState represents the current state of test execution
type TestState string

const (
	StateInit            TestState = "init"
	StateConnecting      TestState = "connecting"
	StatePreparing       TestState = "preparing"
	StateStartingServers TestState = "starting_servers"
	StateStartingClients TestState = "starting_clients"
	StateRunning         TestState = "running"
	StateCollecting      TestState = "collecting"
	StateComplete        TestState = "complete"
	StateFailed          TestState = "failed"
)

// Orchestrator manages the execution of distributed tests
type Orchestrator struct {
	clientPool        *client.Pool
	topology          *topology.Topology
	state             TestState
	errors            []error
	saveDaemonResults bool
}

// NewOrchestrator creates a new test orchestrator
func NewOrchestrator(clientPool *client.Pool, saveDaemonResults bool) *Orchestrator {
	return &Orchestrator{
		clientPool:        clientPool,
		state:             StateInit,
		errors:            make([]error, 0),
		saveDaemonResults: saveDaemonResults,
	}
}

// ExecuteTest executes a complete test workflow
func (o *Orchestrator) ExecuteTest(ctx context.Context, topo *topology.Topology) error {
	o.topology = topo

	log.Printf("Starting test execution with %d test pairs", topo.GetTestCount())

	// Phase 1: Initialize all daemons
	if err := o.initializePhase(ctx); err != nil {
		return fmt.Errorf("initialization phase failed: %w", err)
	}

	// Phase 2: Prepare test on all nodes
	if err := o.preparePhase(ctx); err != nil {
		return fmt.Errorf("prepare phase failed: %w", err)
	}

	// Phase 3: Start servers on all nodes
	if err := o.startServersPhase(ctx); err != nil {
		return fmt.Errorf("start servers phase failed: %w", err)
	}

	// Phase 4: Start clients on all nodes
	if err := o.startClientsPhase(ctx); err != nil {
		return fmt.Errorf("start clients phase failed: %w", err)
	}

	// Phase 5: Wait for tests to complete
	if err := o.waitPhase(ctx); err != nil {
		return fmt.Errorf("wait phase failed: %w", err)
	}

	// Phase 6: Collect results
	if err := o.collectPhase(ctx); err != nil {
		return fmt.Errorf("collect phase failed: %w", err)
	}

	// Phase 7: Cleanup
	if err := o.cleanupPhase(ctx); err != nil {
		log.Printf("Warning: cleanup phase had errors: %v", err)
	}

	o.state = StateComplete
	log.Println("Test execution complete")

	return nil
}

// initializePhase initializes all daemons
func (o *Orchestrator) initializePhase(ctx context.Context) error {
	o.state = StateConnecting
	log.Println("Phase 1: Initializing daemons...")

	req := &pb.InitializeRequest{
		MaxProcesses: 200,
		LogLevel:     "info",
		SaveResults:  o.saveDaemonResults,
	}

	if err := o.clientPool.Initialize(ctx, req); err != nil {
		o.state = StateFailed
		return err
	}

	log.Printf("Successfully initialized %d daemons", o.clientPool.Count())
	if o.saveDaemonResults {
		log.Println("Daemons will save local copies of results")
	}
	return nil
}

// preparePhase validates capacity on all nodes
func (o *Orchestrator) preparePhase(ctx context.Context) error {
	o.state = StatePreparing
	log.Println("Phase 2: Preparing test topology...")

	// Generate per-node topologies
	nodeTopologies, err := topology.GenerateNodeTopologies(o.topology)
	if err != nil {
		o.state = StateFailed
		return fmt.Errorf("failed to generate node topologies: %w", err)
	}

	// Send prepare request to each node
	clients := o.clientPool.GetAllClients()
	errors := make([]error, 0)

	for _, c := range clients {
		nodeTopology, exists := nodeTopologies[c.Node.ID]
		if !exists {
			continue
		}

		req := &pb.PrepareTestRequest{
			Topology: nodeTopology,
		}

		resp, err := c.Client.PrepareTest(ctx, req)
		if err != nil {
			errors = append(errors, fmt.Errorf("node %s: %w", c.Node.ID, err))
			continue
		}

		if !resp.CanHandle {
			errors = append(errors, fmt.Errorf("node %s: %s", c.Node.ID, resp.Message))
		} else {
			log.Printf("Node %s: ready (%d servers, %d clients)",
				c.Node.ID,
				len(nodeTopology.ServerAssignments),
				len(nodeTopology.ClientAssignments))
		}
	}

	if len(errors) > 0 {
		o.state = StateFailed
		return fmt.Errorf("preparation failed on %d nodes: %v", len(errors), errors)
	}

	log.Println("All nodes prepared successfully")
	return nil
}

// startServersPhase starts iperf3 servers on all nodes
func (o *Orchestrator) startServersPhase(ctx context.Context) error {
	o.state = StateStartingServers
	log.Println("Phase 3: Starting iperf3 servers...")

	clients := o.clientPool.GetAllClients()
	errors := make([]error, 0)
	totalServers := 0

	for _, c := range clients {
		ports, exists := o.topology.ServerPorts[c.Node.ID]
		if !exists || len(ports) == 0 {
			continue
		}

		req := &pb.StartServersRequest{
			Ports:          ports,
			TimeoutSeconds: 30,
		}

		resp, err := c.Client.StartServers(ctx, req)
		if err != nil {
			errors = append(errors, fmt.Errorf("node %s: %w", c.Node.ID, err))
			continue
		}

		if !resp.Success {
			errors = append(errors, fmt.Errorf("node %s: %s", c.Node.ID, resp.Message))
		} else {
			totalServers += len(resp.StartedPorts)
			log.Printf("Node %s: started %d servers on ports %v",
				c.Node.ID, len(resp.StartedPorts), resp.StartedPorts)
		}
	}

	if len(errors) > 0 {
		o.state = StateFailed
		return fmt.Errorf("server start failed on %d nodes: %v", len(errors), errors)
	}

	log.Printf("Started %d servers across all nodes", totalServers)

	// Give servers time to start
	time.Sleep(2 * time.Second)

	return nil
}

// startClientsPhase starts iperf3 clients on all nodes
func (o *Orchestrator) startClientsPhase(ctx context.Context) error {
	o.state = StateStartingClients
	log.Println("Phase 4: Starting iperf3 clients...")

	clients := o.clientPool.GetAllClients()
	errors := make([]error, 0)
	totalClients := 0

	for _, c := range clients {
		testPairs, exists := o.topology.ClientTests[c.Node.ID]
		if !exists || len(testPairs) == 0 {
			continue
		}

		// Build client targets
		targets := make([]*pb.ClientTarget, 0, len(testPairs))
		for _, pair := range testPairs {
			destPorts := o.topology.ServerPorts[pair.Destination.ID]
			if len(destPorts) == 0 {
				continue
			}

			targets = append(targets, &pb.ClientTarget{
				TestId:          pair.TestID,
				DestinationIp:   pair.Destination.IP,
				DestinationPort: destPorts[0],
				Profile:         topology.ConvertProfileToProto(pair.Profile),
			})
		}

		if len(targets) == 0 {
			continue
		}

		req := &pb.StartClientsRequest{
			Targets: targets,
		}

		resp, err := c.Client.StartClients(ctx, req)
		if err != nil {
			errors = append(errors, fmt.Errorf("node %s: %w", c.Node.ID, err))
			continue
		}

		if !resp.Success {
			errors = append(errors, fmt.Errorf("node %s: %s", c.Node.ID, resp.Message))
		} else {
			totalClients += len(resp.StartedTestIds)
			log.Printf("Node %s: started %d client tests", c.Node.ID, len(resp.StartedTestIds))
		}
	}

	if len(errors) > 0 {
		o.state = StateFailed
		return fmt.Errorf("client start failed on %d nodes: %v", len(errors), errors)
	}

	log.Printf("Started %d client tests across all nodes", totalClients)
	return nil
}

// waitPhase waits for all tests to complete
func (o *Orchestrator) waitPhase(ctx context.Context) error {
	o.state = StateRunning
	log.Println("Phase 5: Waiting for tests to complete...")

	// Calculate wait time based on longest test duration
	maxDuration := 10 // Default 10 seconds
	for _, pair := range o.topology.Pairs {
		if pair.Profile.Duration > maxDuration {
			maxDuration = pair.Profile.Duration
		}
	}

	// Add buffer for test setup and teardown
	waitTime := time.Duration(maxDuration+10) * time.Second

	log.Printf("Waiting %v for tests to complete...", waitTime)
	time.Sleep(waitTime)

	log.Println("Test execution window complete")
	return nil
}

// collectPhase verifies results are ready on all nodes
func (o *Orchestrator) collectPhase(ctx context.Context) error {
	o.state = StateCollecting
	log.Println("Phase 6: Collecting results...")

	clients := o.clientPool.GetAllClients()
	totalResults := 0

	for _, c := range clients {
		req := &pb.GetResultsRequest{
			ClearAfterRetrieval: false, // Don't clear - aggregator will collect later
		}

		resp, err := c.Client.GetResults(ctx, req)
		if err != nil {
			log.Printf("Warning: failed to get results from node %s: %v", c.Node.ID, err)
			continue
		}

		totalResults += int(resp.TotalCount)
		log.Printf("Node %s: collected %d results", c.Node.ID, resp.TotalCount)
	}

	log.Printf("Collected %d total results", totalResults)
	return nil
}

// cleanupPhase stops all processes and cleans up
func (o *Orchestrator) cleanupPhase(ctx context.Context) error {
	log.Println("Phase 7: Cleanup...")

	if err := o.clientPool.StopAll(ctx); err != nil {
		return err
	}

	log.Println("Cleanup complete")
	return nil
}

// GetState returns the current orchestrator state
func (o *Orchestrator) GetState() TestState {
	return o.state
}

// GetErrors returns any errors encountered during execution
func (o *Orchestrator) GetErrors() []error {
	return o.errors
}
