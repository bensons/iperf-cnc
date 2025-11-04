package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/bensons/iperf-cnc/api/proto"
	"github.com/bensons/iperf-cnc/internal/common/models"
)

// NodeClient wraps a gRPC connection to a daemon
type NodeClient struct {
	Node   *models.Node
	Conn   *grpc.ClientConn
	Client pb.DaemonServiceClient
}

// Pool manages gRPC connections to multiple daemons
type Pool struct {
	clients map[string]*NodeClient
	mu      sync.RWMutex
	timeout time.Duration
}

// NewPool creates a new client pool
func NewPool(timeout time.Duration) *Pool {
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	return &Pool{
		clients: make(map[string]*NodeClient),
		timeout: timeout,
	}
}

// Connect establishes a connection to a node
func (p *Pool) Connect(ctx context.Context, node *models.Node) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check if already connected
	if _, exists := p.clients[node.ID]; exists {
		return nil
	}

	// Create connection
	addr := node.Address()
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", addr, err)
	}

	// Create client
	client := pb.NewDaemonServiceClient(conn)

	// Store client
	p.clients[node.ID] = &NodeClient{
		Node:   node,
		Conn:   conn,
		Client: client,
	}

	return nil
}

// ConnectAll establishes connections to all nodes
func (p *Pool) ConnectAll(ctx context.Context, nodes []*models.Node) error {
	errors := make([]error, 0)

	for _, node := range nodes {
		if err := p.Connect(ctx, node); err != nil {
			errors = append(errors, fmt.Errorf("node %s: %w", node.ID, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to connect to %d nodes: %v", len(errors), errors)
	}

	return nil
}

// GetClient returns the client for a node
func (p *Pool) GetClient(nodeID string) (*NodeClient, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	client, exists := p.clients[nodeID]
	if !exists {
		return nil, fmt.Errorf("no connection to node %s", nodeID)
	}

	return client, nil
}

// GetAllClients returns all connected clients
func (p *Pool) GetAllClients() []*NodeClient {
	p.mu.RLock()
	defer p.mu.RUnlock()

	clients := make([]*NodeClient, 0, len(p.clients))
	for _, client := range p.clients {
		clients = append(clients, client)
	}

	return clients
}

// Initialize initializes all connected daemons
func (p *Pool) Initialize(ctx context.Context, config *pb.InitializeRequest) error {
	clients := p.GetAllClients()
	errors := make([]error, 0)

	for _, client := range clients {
		resp, err := client.Client.Initialize(ctx, config)
		if err != nil {
			errors = append(errors, fmt.Errorf("node %s: %w", client.Node.ID, err))
			continue
		}

		if !resp.Success {
			errors = append(errors, fmt.Errorf("node %s: %s", client.Node.ID, resp.Message))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("initialization failed on %d nodes: %v", len(errors), errors)
	}

	return nil
}

// CheckHealth checks the health of all connected nodes
func (p *Pool) CheckHealth(ctx context.Context) (map[string]*pb.DaemonStatus, error) {
	clients := p.GetAllClients()
	statuses := make(map[string]*pb.DaemonStatus)
	errors := make([]error, 0)

	for _, client := range clients {
		resp, err := client.Client.GetStatus(ctx, &pb.GetStatusRequest{})
		if err != nil {
			errors = append(errors, fmt.Errorf("node %s: %w", client.Node.ID, err))
			continue
		}

		statuses[client.Node.ID] = resp.Status
	}

	if len(errors) > 0 {
		return statuses, fmt.Errorf("health check failed on %d nodes: %v", len(errors), errors)
	}

	return statuses, nil
}

// StopAll stops all processes on all nodes
func (p *Pool) StopAll(ctx context.Context) error {
	clients := p.GetAllClients()
	errors := make([]error, 0)

	for _, client := range clients {
		_, err := client.Client.StopAll(ctx, &pb.StopAllRequest{Force: true})
		if err != nil {
			errors = append(errors, fmt.Errorf("node %s: %w", client.Node.ID, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("stop failed on %d nodes: %v", len(errors), errors)
	}

	return nil
}

// Close closes all connections
func (p *Pool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	errors := make([]error, 0)

	for nodeID, client := range p.clients {
		if err := client.Conn.Close(); err != nil {
			errors = append(errors, fmt.Errorf("node %s: %w", nodeID, err))
		}
	}

	p.clients = make(map[string]*NodeClient)

	if len(errors) > 0 {
		return fmt.Errorf("failed to close %d connections: %v", len(errors), errors)
	}

	return nil
}

// Count returns the number of connected clients
func (p *Pool) Count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return len(p.clients)
}

// IsConnected checks if a node is connected
func (p *Pool) IsConnected(nodeID string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	_, exists := p.clients[nodeID]
	return exists
}
