package collector

import (
	"fmt"
	"sync"
	"time"

	"github.com/bensons/iperf-cnc/internal/common/iperf"
)

// TestResult represents the result of a test execution
type TestResult struct {
	TestID       string
	SourceID     string
	DestinationID string
	Status       string
	IperfJSON    string
	ErrorMessage string
	StartTime    time.Time
	EndTime      time.Time
	ExitCode     int
}

// Collector collects and stores test results
type Collector struct {
	results      map[string]*TestResult
	completed    int
	failed       int
	mu           sync.RWMutex
	resultDir    string
}

// NewCollector creates a new result collector
func NewCollector(resultDir string) *Collector {
	return &Collector{
		results:   make(map[string]*TestResult),
		resultDir: resultDir,
	}
}

// StoreResult stores a test result
func (c *Collector) StoreResult(result *TestResult) error {
	if result == nil {
		return fmt.Errorf("result cannot be nil")
	}

	if result.TestID == "" {
		return fmt.Errorf("test ID cannot be empty")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.results[result.TestID] = result

	if result.Status == "completed" {
		c.completed++
	} else if result.Status == "failed" {
		c.failed++
	}

	return nil
}

// StoreIperfResult stores a result from iperf wrapper
func (c *Collector) StoreIperfResult(testID string, result *iperf.Result) error {
	if result == nil {
		return fmt.Errorf("result cannot be nil")
	}

	status := "completed"
	if !result.Success {
		status = "failed"
	}

	testResult := &TestResult{
		TestID:       testID,
		Status:       status,
		IperfJSON:    result.JSONOutput,
		ErrorMessage: result.Error,
		StartTime:    result.StartTime,
		EndTime:      result.EndTime,
		ExitCode:     result.ExitCode,
	}

	return c.StoreResult(testResult)
}

// GetResult retrieves a specific test result
func (c *Collector) GetResult(testID string) (*TestResult, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result, exists := c.results[testID]
	if !exists {
		return nil, fmt.Errorf("result for test %s not found", testID)
	}

	return result, nil
}

// GetAllResults returns all stored results
func (c *Collector) GetAllResults() []*TestResult {
	c.mu.RLock()
	defer c.mu.RUnlock()

	results := make([]*TestResult, 0, len(c.results))
	for _, result := range c.results {
		results = append(results, result)
	}

	return results
}

// GetResults returns results for specific test IDs
func (c *Collector) GetResults(testIDs []string) []*TestResult {
	c.mu.RLock()
	defer c.mu.RUnlock()

	results := make([]*TestResult, 0, len(testIDs))
	for _, testID := range testIDs {
		if result, exists := c.results[testID]; exists {
			results = append(results, result)
		}
	}

	return results
}

// ClearResult removes a specific result
func (c *Collector) ClearResult(testID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	result, exists := c.results[testID]
	if !exists {
		return fmt.Errorf("result for test %s not found", testID)
	}

	delete(c.results, testID)

	if result.Status == "completed" {
		c.completed--
	} else if result.Status == "failed" {
		c.failed--
	}

	return nil
}

// ClearResults removes multiple results
func (c *Collector) ClearResults(testIDs []string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, testID := range testIDs {
		if result, exists := c.results[testID]; exists {
			delete(c.results, testID)

			if result.Status == "completed" {
				c.completed--
			} else if result.Status == "failed" {
				c.failed--
			}
		}
	}
}

// ClearAll removes all stored results
func (c *Collector) ClearAll() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.results = make(map[string]*TestResult)
	c.completed = 0
	c.failed = 0
}

// GetCount returns the total number of stored results
func (c *Collector) GetCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.results)
}

// GetCompletedCount returns the number of completed tests
func (c *Collector) GetCompletedCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.completed
}

// GetFailedCount returns the number of failed tests
func (c *Collector) GetFailedCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.failed
}

// HasResult checks if a result exists for a test
func (c *Collector) HasResult(testID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	_, exists := c.results[testID]
	return exists
}

// GetResultIDs returns all test IDs with stored results
func (c *Collector) GetResultIDs() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	ids := make([]string, 0, len(c.results))
	for id := range c.results {
		ids = append(ids, id)
	}

	return ids
}
