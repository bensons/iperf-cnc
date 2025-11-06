package server

import (
	"context"
	"fmt"
	"os"
	"time"

	pb "github.com/bensons/iperf-cnc/api/proto"
	"github.com/bensons/iperf-cnc/internal/common/iperf"
	"github.com/bensons/iperf-cnc/internal/daemon/collector"
	"github.com/bensons/iperf-cnc/internal/daemon/port"
	"github.com/bensons/iperf-cnc/internal/daemon/process"
)

// DaemonServer implements the DaemonService gRPC service
type DaemonServer struct {
	pb.UnimplementedDaemonServiceServer

	portAllocator  *port.Allocator
	processManager *process.Manager
	capacity       *process.CapacityCalculator
	collector      *collector.Collector

	// Daemon metadata
	hostname  string
	version   string
	startTime time.Time

	// Configuration
	config *Config
}

// Config contains daemon server configuration
type Config struct {
	ListenPort     int
	PortRangeStart int
	PortRangeEnd   int
	MaxProcesses   int
	CPUAffinity    bool
	LogLevel       string
	ResultDir      string
	IperfPath      string
}

// NewDaemonServer creates a new daemon gRPC server
func NewDaemonServer(config *Config) (*DaemonServer, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// Create port allocator
	portAllocator, err := port.NewAllocator(config.PortRangeStart, config.PortRangeEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to create port allocator: %w", err)
	}

	// Create capacity calculator
	capacityCalc := process.NewCapacityCalculator(config.MaxProcesses)

	// Create result collector
	resultCollector := collector.NewCollector(config.ResultDir)

	// Create process manager
	iperfPath := config.IperfPath
	if iperfPath == "" {
		iperfPath = "iperf3"
	}
	processManager := process.NewManager(portAllocator, capacityCalc, resultCollector, iperfPath)

	// Get hostname
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	return &DaemonServer{
		portAllocator:  portAllocator,
		processManager: processManager,
		capacity:       capacityCalc,
		collector:      resultCollector,
		hostname:       hostname,
		version:        "dev",
		startTime:      time.Now(),
		config:         config,
	}, nil
}

// Initialize initializes the daemon with configuration
func (s *DaemonServer) Initialize(ctx context.Context, req *pb.InitializeRequest) (*pb.InitializeResponse, error) {
	// Update configuration if provided
	if req.MaxProcesses > 0 {
		s.config.MaxProcesses = int(req.MaxProcesses)
	}

	// Detect capacity
	capacity, err := s.capacity.DetectCapacity()
	if err != nil {
		return &pb.InitializeResponse{
			Success: false,
			Message: fmt.Sprintf("failed to detect capacity: %v", err),
		}, nil
	}

	return &pb.InitializeResponse{
		Success: true,
		Message: "daemon initialized successfully",
		NodeInfo: &pb.NodeInfo{
			Id:       s.hostname,
			Hostname: s.hostname,
			Ip:       "",                         // Will be filled by controller
			Port:     int32(s.config.ListenPort), // #nosec G115 -- Port is validated to be in valid range
			Capacity: &pb.ProcessCapacity{
				MaxProcesses:         int32(capacity.MaxProcesses),       // #nosec G115 -- Process count is reasonable
				AvailableProcesses:   int32(capacity.AvailableProcesses), // #nosec G115 -- Process count is reasonable
				CpuCores:             int32(capacity.CPUCores),           // #nosec G115 -- CPU core count is reasonable
				AvailableMemoryBytes: int64(capacity.AvailableMemory),    // #nosec G115 -- Safe conversion to int64
				NetworkInterfaces:    capacity.NetworkInterfaces,
			},
		},
	}, nil
}

// PrepareTest validates if the daemon can handle the test topology
func (s *DaemonServer) PrepareTest(ctx context.Context, req *pb.PrepareTestRequest) (*pb.PrepareTestResponse, error) {
	if req.Topology == nil {
		return &pb.PrepareTestResponse{
			CanHandle: false,
			Message:   "topology is required",
		}, nil
	}

	// Calculate required capacity
	serverCount := len(req.Topology.ServerAssignments)
	clientCount := len(req.Topology.ClientAssignments)
	totalRequired := serverCount + clientCount

	// Check if we have enough slots
	availableSlots := s.capacity.GetAvailableSlots()
	canHandle := availableSlots >= totalRequired

	message := "sufficient capacity available"
	if !canHandle {
		message = fmt.Sprintf("insufficient capacity: need %d slots, have %d available",
			totalRequired, availableSlots)
	}

	// Detect current capacity
	capacity, err := s.capacity.DetectCapacity()
	if err != nil {
		return &pb.PrepareTestResponse{
			CanHandle: false,
			Message:   fmt.Sprintf("failed to detect capacity: %v", err),
		}, nil
	}

	return &pb.PrepareTestResponse{
		CanHandle: canHandle,
		Message:   message,
		RequiredCapacity: &pb.ProcessCapacity{
			MaxProcesses:       int32(totalRequired), // #nosec G115 -- Process count is reasonable
			AvailableProcesses: int32(totalRequired), // #nosec G115 -- Process count is reasonable
		},
		AvailableCapacity: &pb.ProcessCapacity{
			MaxProcesses:         int32(capacity.MaxProcesses),       // #nosec G115 -- Process count is reasonable
			AvailableProcesses:   int32(capacity.AvailableProcesses), // #nosec G115 -- Process count is reasonable
			CpuCores:             int32(capacity.CPUCores),           // #nosec G115 -- CPU core count is reasonable
			AvailableMemoryBytes: int64(capacity.AvailableMemory),    // #nosec G115 -- Safe conversion to int64
			NetworkInterfaces:    capacity.NetworkInterfaces,
		},
	}, nil
}

// StartServers starts iperf3 servers on allocated ports
func (s *DaemonServer) StartServers(ctx context.Context, req *pb.StartServersRequest) (*pb.StartServersResponse, error) {
	if len(req.Ports) == 0 {
		return &pb.StartServersResponse{
			Success: false,
			Message: "no ports specified",
		}, nil
	}

	startedPorts := make([]int32, 0)
	errors := make([]string, 0)

	for _, port := range req.Ports {
		if err := s.processManager.StartServer(int(port)); err != nil {
			errors = append(errors, fmt.Sprintf("port %d: %v", port, err))
		} else {
			startedPorts = append(startedPorts, port)
		}
	}

	success := len(startedPorts) > 0
	message := fmt.Sprintf("started %d/%d servers", len(startedPorts), len(req.Ports))

	return &pb.StartServersResponse{
		Success:      success,
		Message:      message,
		StartedPorts: startedPorts,
		Errors:       errors,
	}, nil
}

// StartClients starts iperf3 clients to connect to targets
func (s *DaemonServer) StartClients(ctx context.Context, req *pb.StartClientsRequest) (*pb.StartClientsResponse, error) {
	if len(req.Targets) == 0 {
		return &pb.StartClientsResponse{
			Success: false,
			Message: "no targets specified",
		}, nil
	}

	startedTestIDs := make([]string, 0)
	errors := make([]string, 0)

	for _, target := range req.Targets {
		config := convertProfileToIperfConfig(target.Profile)

		err := s.processManager.StartClient(
			target.TestId,
			target.DestinationIp,
			int(target.DestinationPort),
			config,
		)

		if err != nil {
			errors = append(errors, fmt.Sprintf("test %s: %v", target.TestId, err))
		} else {
			startedTestIDs = append(startedTestIDs, target.TestId)
		}
	}

	success := len(startedTestIDs) > 0
	message := fmt.Sprintf("started %d/%d clients", len(startedTestIDs), len(req.Targets))

	return &pb.StartClientsResponse{
		Success:        success,
		Message:        message,
		StartedTestIds: startedTestIDs,
		Errors:         errors,
	}, nil
}

// StopAll stops all running iperf3 processes
func (s *DaemonServer) StopAll(ctx context.Context, req *pb.StopAllRequest) (*pb.StopAllResponse, error) {
	stoppedCount := s.processManager.StopAll()

	return &pb.StopAllResponse{
		Success:          true,
		Message:          fmt.Sprintf("stopped %d processes", stoppedCount),
		StoppedProcesses: int32(stoppedCount), // #nosec G115 -- Process count is reasonable
	}, nil
}

// GetResults retrieves test results from completed runs
func (s *DaemonServer) GetResults(ctx context.Context, req *pb.GetResultsRequest) (*pb.GetResultsResponse, error) {
	var results []*collector.TestResult

	if len(req.TestIds) == 0 {
		// Get all results
		results = s.collector.GetAllResults()
	} else {
		// Get specific results
		results = s.collector.GetResults(req.TestIds)
	}

	// Convert to protobuf
	pbResults := make([]*pb.TestResult, 0, len(results))
	for _, result := range results {
		status := pb.TestStatus_TEST_STATUS_COMPLETED
		if result.Status == "failed" {
			status = pb.TestStatus_TEST_STATUS_FAILED
		}

		pbResults = append(pbResults, &pb.TestResult{
			TestId:        result.TestID,
			SourceId:      result.SourceID,
			DestinationId: result.DestinationID,
			Status:        status,
			IperfJson:     result.IperfJSON,
			ErrorMessage:  result.ErrorMessage,
			StartTimeUnix: result.StartTime.Unix(),
			EndTimeUnix:   result.EndTime.Unix(),
			ExitCode:      int32(result.ExitCode), // #nosec G115 -- Exit code is in valid range
		})
	}

	// Clear results if requested
	if req.ClearAfterRetrieval {
		s.collector.ClearAll()
	}

	return &pb.GetResultsResponse{
		Results:    pbResults,
		TotalCount: int32(len(pbResults)), // #nosec G115 -- Result count is reasonable
	}, nil
}

// GetStatus returns current daemon health and resource usage
func (s *DaemonServer) GetStatus(ctx context.Context, req *pb.GetStatusRequest) (*pb.GetStatusResponse, error) {
	capacity, err := s.capacity.DetectCapacity()
	if err != nil {
		return nil, fmt.Errorf("failed to detect capacity: %w", err)
	}

	uptime := time.Since(s.startTime).Seconds()

	return &pb.GetStatusResponse{
		Status: &pb.DaemonStatus{
			Healthy:          true,
			RunningProcesses: int32(s.processManager.GetRunningCount()), // #nosec G115 -- Process count is reasonable
			CompletedTests:   int32(s.collector.GetCompletedCount()),    // #nosec G115 -- Test count is reasonable
			FailedTests:      int32(s.collector.GetFailedCount()),       // #nosec G115 -- Test count is reasonable
			CurrentCapacity: &pb.ProcessCapacity{
				MaxProcesses:         int32(capacity.MaxProcesses),       // #nosec G115 -- Process count is reasonable
				AvailableProcesses:   int32(capacity.AvailableProcesses), // #nosec G115 -- Process count is reasonable
				CpuCores:             int32(capacity.CPUCores),           // #nosec G115 -- CPU core count is reasonable
				AvailableMemoryBytes: int64(capacity.AvailableMemory),    // #nosec G115 -- Safe conversion to int64
				NetworkInterfaces:    capacity.NetworkInterfaces,
			},
			UptimeSeconds: int64(uptime),
			Version:       s.version,
		},
	}, nil
}

// convertProfileToIperfConfig converts protobuf TestProfile to iperf.Config
func convertProfileToIperfConfig(profile *pb.TestProfile) *iperf.Config {
	if profile == nil {
		return &iperf.Config{}
	}

	// Convert protocol
	protocol := iperf.ProtocolTCP // Default to TCP
	if profile.Protocol == pb.Protocol_PROTOCOL_UDP {
		protocol = iperf.ProtocolUDP
	}

	return &iperf.Config{
		Protocol:          protocol,
		Duration:          int(profile.DurationSeconds),
		Bandwidth:         profile.Bandwidth,
		WindowSize:        profile.WindowSize,
		Parallel:          int(profile.ParallelStreams),
		Bidirectional:     profile.Bidirectional,
		Reverse:           profile.Reverse,
		BufferLength:      int(profile.BufferLength),
		CongestionControl: profile.CongestionControl,
		MSS:               int(profile.Mss),
		NoDelay:           profile.NoDelay,
		TOS:               int(profile.Tos),
		ZeroCopy:          profile.Zerocopy,
		OmitSeconds:       int(profile.OmitSeconds),
	}
}
