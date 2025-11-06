# iperf-cnc

**Version:** v0.1.6

A distributed command and control system for orchestrating large-scale iperf3 network performance testing across clusters.

## Overview

iperf-cnc enables automated, full-mesh network performance testing across multiple nodes. It consists of two components:

- **iperf-daemon**: Lightweight agent running on each test node, managing local iperf3 processes
- **iperf-controller**: Centralized orchestrator that coordinates tests across all nodes and aggregates results

## Key Features

- **Full-mesh topology**: Every node tests against every other node
- **Resource maximization**: Spawns multiple iperf3 processes to utilize all available CPU cores
- **Dynamic port allocation**: Automatically manages port assignments to avoid conflicts
- **Distributed architecture**: gRPC-based communication between controller and daemons
- **Result aggregation**: Collects and consolidates test results in JSON/CSV formats
- **Flexible configuration**: YAML-based configuration for test profiles and node definitions

## Quick Start

### Build

```bash
make build
```

Binaries will be created in `build/`:
- `build/iperf-daemon`
- `build/iperf-controller`

### Run Daemon

On each test node:

```bash
./iperf-daemon -c daemon.yaml
```

### Run Controller

On the control node:

```bash
./iperf-controller run -c controller.yaml
```

## Configuration

See example configurations in `configs/`:
- `configs/daemon.yaml` - Daemon configuration
- `configs/controller.yaml` - Controller configuration with node definitions and test profiles

## Requirements

- Go 1.21+
- iperf3 3.16+ installed on all test nodes
- gRPC connectivity between controller and all daemons

## Documentation

- [Design Document](design-doc.md) - Architecture and design decisions
- [Release Process](RELEASING.md) - How to create releases

## License

See [LICENSE](LICENSE) file for details.

