package aggregator

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	pb "github.com/bensons/iperf-cnc/api/proto"
	"github.com/bensons/iperf-cnc/internal/controller/client"
)

// TestResult represents an aggregated test result
type TestResult struct {
	TestID        string                 `json:"test_id"`
	SourceNode    string                 `json:"source_node"`
	DestNode      string                 `json:"dest_node"`
	Status        string                 `json:"status"`
	StartTime     int64                  `json:"start_time"`
	EndTime       int64                  `json:"end_time"`
	Duration      int64                  `json:"duration"`
	ErrorMessage  string                 `json:"error_message,omitempty"`
	IperfData     map[string]interface{} `json:"iperf_data,omitempty"`
	ThroughputBps float64                `json:"throughput_bps,omitempty"`
	Retransmits   int64                  `json:"retransmits,omitempty"`
}

// Summary contains aggregate statistics
type Summary struct {
	TotalTests      int     `json:"total_tests"`
	CompletedTests  int     `json:"completed_tests"`
	FailedTests     int     `json:"failed_tests"`
	AvgThroughput   float64 `json:"avg_throughput_bps"`
	MinThroughput   float64 `json:"min_throughput_bps"`
	MaxThroughput   float64 `json:"max_throughput_bps"`
	TotalRetransmits int64  `json:"total_retransmits"`
}

// Aggregator collects and aggregates results from all nodes
type Aggregator struct {
	results map[string]*TestResult
	mu      sync.RWMutex
}

// NewAggregator creates a new result aggregator
func NewAggregator() *Aggregator {
	return &Aggregator{
		results: make(map[string]*TestResult),
	}
}

// CollectResults collects results from all nodes via the client pool
func (a *Aggregator) CollectResults(ctx context.Context, clientPool *client.Pool) error {
	clients := clientPool.GetAllClients()

	for _, c := range clients {
		req := &pb.GetResultsRequest{
			ClearAfterRetrieval: false, // Don't clear yet
		}

		resp, err := c.Client.GetResults(ctx, req)
		if err != nil {
			return fmt.Errorf("failed to get results from node %s: %w", c.Node.ID, err)
		}

		// Process each result
		for _, pbResult := range resp.Results {
			result, err := a.convertResult(pbResult)
			if err != nil {
				return fmt.Errorf("failed to convert result: %w", err)
			}

			a.addResult(result)
		}
	}

	return nil
}

// convertResult converts a protobuf result to an aggregated result
func (a *Aggregator) convertResult(pbResult *pb.TestResult) (*TestResult, error) {
	result := &TestResult{
		TestID:       pbResult.TestId,
		SourceNode:   pbResult.SourceId,
		DestNode:     pbResult.DestinationId,
		Status:       pbResult.Status.String(),
		StartTime:    pbResult.StartTimeUnix,
		EndTime:      pbResult.EndTimeUnix,
		Duration:     pbResult.EndTimeUnix - pbResult.StartTimeUnix,
		ErrorMessage: pbResult.ErrorMessage,
	}

	// Parse iperf JSON if available
	if pbResult.IperfJson != "" {
		var iperfData map[string]interface{}
		if err := json.Unmarshal([]byte(pbResult.IperfJson), &iperfData); err == nil {
			result.IperfData = iperfData

			// Extract throughput
			if throughput, err := extractThroughput(iperfData); err == nil {
				result.ThroughputBps = throughput
			}

			// Extract retransmits
			if retransmits, err := extractRetransmits(iperfData); err == nil {
				result.Retransmits = retransmits
			}
		}
	}

	return result, nil
}

// addResult adds a result to the aggregator
func (a *Aggregator) addResult(result *TestResult) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.results[result.TestID] = result
}

// GetResults returns all collected results
func (a *Aggregator) GetResults() []*TestResult {
	a.mu.RLock()
	defer a.mu.RUnlock()

	results := make([]*TestResult, 0, len(a.results))
	for _, result := range a.results {
		results = append(results, result)
	}

	return results
}

// GetSummary returns aggregate statistics
func (a *Aggregator) GetSummary() *Summary {
	a.mu.RLock()
	defer a.mu.RUnlock()

	summary := &Summary{
		TotalTests: len(a.results),
		MinThroughput: -1,
	}

	var totalThroughput float64

	for _, result := range a.results {
		if result.Status == "TEST_STATUS_COMPLETED" {
			summary.CompletedTests++

			// Throughput stats
			if result.ThroughputBps > 0 {
				totalThroughput += result.ThroughputBps

				if summary.MinThroughput < 0 || result.ThroughputBps < summary.MinThroughput {
					summary.MinThroughput = result.ThroughputBps
				}

				if result.ThroughputBps > summary.MaxThroughput {
					summary.MaxThroughput = result.ThroughputBps
				}
			}

			// Retransmits
			summary.TotalRetransmits += result.Retransmits
		} else if result.Status == "TEST_STATUS_FAILED" {
			summary.FailedTests++
		}
	}

	if summary.CompletedTests > 0 {
		summary.AvgThroughput = totalThroughput / float64(summary.CompletedTests)
	}

	if summary.MinThroughput < 0 {
		summary.MinThroughput = 0
	}

	return summary
}

// GetResultCount returns the number of collected results
func (a *Aggregator) GetResultCount() int {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return len(a.results)
}

// extractThroughput extracts throughput from iperf JSON data
func extractThroughput(data map[string]interface{}) (float64, error) {
	end, ok := data["end"].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("missing 'end' section")
	}

	sumSent, ok := end["sum_sent"].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("missing 'sum_sent' section")
	}

	bps, ok := sumSent["bits_per_second"].(float64)
	if !ok {
		return 0, fmt.Errorf("missing 'bits_per_second'")
	}

	return bps, nil
}

// extractRetransmits extracts retransmit count from iperf JSON data
func extractRetransmits(data map[string]interface{}) (int64, error) {
	end, ok := data["end"].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("missing 'end' section")
	}

	sumSent, ok := end["sum_sent"].(map[string]interface{})
	if !ok {
		return 0, nil // No retransmit data available
	}

	retransmits, ok := sumSent["retransmits"].(float64)
	if !ok {
		return 0, nil // No retransmit data
	}

	return int64(retransmits), nil
}
