# Design Document Prompt for Distributed Mesh iperf3 Testing Tool

## Overview
Design a distributed network performance testing system that executes and manages a large number of iperf3 instances across a cluster of servers. The system should create a fully-meshed topology where every server acts as both a client and server, running multiple parallel iperf3 processes to maximize resource utilization.

## System Architecture

### High-Level Requirements
- **Fully-meshed topology**: Every node in the cluster should run iperf3 clients connecting to every other node's iperf3 servers
- **Distributed daemon architecture**: A lightweight daemon process running on each cluster node to manage local iperf3 instances
- **Centralized control**: A single control application that orchestrates the entire testing workflow across all nodes
- **Resource maximization**: Each daemon should spawn multiple separate iperf3 processes to maximize CPU parallelization and utilize all available system resources
- **Data aggregation**: The control application should collect results from all test instances and persist them in a structured format

## Daemon Component Specifications

### Responsibilities
1. **Process Management**: Launch and monitor multiple iperf3 server and client processes on the local node
2. **Resource Awareness**: Automatically determine optimal number of processes based on available CPU cores, memory, and network interfaces
3. **Port Management**: Dynamically allocate ports for iperf3 instances to avoid conflicts
4. **Process Lifecycle**: Handle process startup, monitoring, graceful shutdown, and error recovery
5. **Local Result Collection**: Gather JSON output from each iperf3 process with test metadata (source node, destination node, process ID, port)

### Interface
- **RPC/gRPC Communication**: Accept commands from the control application to:
  - Initialize and configure the local test environment
  - Start server instances on specified ports with specified connection attributes
  - Launch client instances connecting to remote servers
  - Stop all local iperf3 processes
  - Retrieve collected results
- **Health Monitoring**: Report daemon status, process health, and resource utilization to the control application

### Process Spawning Strategy
- Spawn multiple separate iperf3 processes (version 3.16+) rather than relying on libiperf threading to avoid thread-safety issues
- Use process isolation to ensure stability and resource independence
- Implement CPU affinity/pinning to bind processes to specific CPU cores for optimal performance
- Each iperf3 process should run with appropriate flags for multi-stream support (`-P` option) within a single connection to the remote daemon

## Control Application Specifications

### Responsibilities
1. **Cluster Discovery**: Identify all nodes in the cluster and establish connections with their daemons
2. **Topology Configuration**: Define the fully-meshed test topology (all-to-all connections)
3. **Test Orchestration**: 
   - Issue coordinated commands to all daemons to start server instances
   - Issue commands to all daemons to launch client instances in a synchronized manner
   - Ensure proper timing and sequencing to avoid race conditions
4. **Result Collection**: 
   - Query all daemons for their local iperf3 results
   - Aggregate and correlate results across all nodes
   - Create comprehensive mapping of source-to-destination test results
5. **Data Persistence**: Save all collected results in a well-structured format (JSON or CSV) with complete metadata

### Interface
- **CLI/Configuration**: Accept cluster topology definition, test parameters (duration, bandwidth profiles, etc.)
- **Progress Reporting**: Display real-time progress of test execution
- **Result Export**: Provide flexible output formats and filtering capabilities

## Data Model and Metadata

### Test Metadata to Capture
- Source node hostname/IP
- Destination node hostname/IP
- Local daemon process ID and iperf3 instance number
- Test start and end timestamps
- Port numbers used for client and server
- iperf3 version
- Test duration and parameters (bitrate, window size, etc.)
- Measured throughput, jitter, packet loss, and other relevant metrics

### Result File Structure
- Aggregate JSON file containing complete test results with hierarchical organization:
  - Top-level: cluster metadata and test parameters
  - Second level: per-node results grouped by source
  - Third level: per-destination results within each source node
  - Fourth level: individual test instance results with JSON output from iperf3

## Implementation Considerations

### Technology Stack Recommendations
- **Language**: Consider Python for rapid development, Go for performance and concurrency, or C/C++ for tight resource control
- **RPC Framework**: gRPC for efficient binary serialization and cross-platform compatibility
- **Process Management**: Use standard OS process APIs (subprocess in Python, etc.)
- **Concurrency Model**: Implement proper asynchronous/concurrent handling of multiple daemon connections in the control application

### Resilience and Error Handling
- Implement retry logic for daemon communication failures
- Handle incomplete or partial test results gracefully
- Provide detailed error logging and reporting for troubleshooting
- Support incremental result collection in case of node failures

### Performance Optimization
- Use process-level parallelization exclusively (no threading within libiperf)
- Implement proper CPU affinity binding to avoid context switching overhead
- Consider network resource constraints when dimensioning number of parallel processes
- Provide configurable resource limits per daemon to prevent system overload

### Monitoring and Diagnostics
- Log daemon and control application activity for troubleshooting
- Provide metrics on:
  - Number of active iperf3 processes per node
  - CPU and memory utilization per daemon
  - Success/failure rates for test connections
  - Latency of daemon communications

## Deliverables
1. Detailed architecture diagram showing daemon, control, and iperf3 process interactions
2. Data model and schema documentation
3. API/Protocol specification for daemon-to-control communication
4. Sequence diagrams for key workflows (initialization, test execution, result collection)
5. Configuration file format specification
6. Result file format specification with example output
7. Deployment and operational procedures
