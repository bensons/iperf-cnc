package orchestrator

import (
	"fmt"
	"sync"
	"time"
)

// Progress tracks test execution progress
type Progress struct {
	mu sync.RWMutex

	// Total counts
	TotalNodes   int
	TotalTests   int
	TotalServers int
	TotalClients int

	// Progress counts
	ConnectedNodes   int
	PreparedNodes    int
	StartedServers   int
	StartedClients   int
	CompletedTests   int
	FailedTests      int
	CollectedResults int

	// Timing
	StartTime    time.Time
	CurrentPhase string
	PhaseStart   time.Time

	// Errors
	Errors []string
}

// NewProgress creates a new progress tracker
func NewProgress() *Progress {
	return &Progress{
		StartTime: time.Now(),
		Errors:    make([]string, 0),
	}
}

// SetTotals sets the total counts
func (p *Progress) SetTotals(nodes, tests, servers, clients int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.TotalNodes = nodes
	p.TotalTests = tests
	p.TotalServers = servers
	p.TotalClients = clients
}

// SetPhase sets the current phase
func (p *Progress) SetPhase(phase string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.CurrentPhase = phase
	p.PhaseStart = time.Now()
}

// IncrementConnected increments connected nodes count
func (p *Progress) IncrementConnected(count int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ConnectedNodes += count
}

// IncrementPrepared increments prepared nodes count
func (p *Progress) IncrementPrepared(count int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.PreparedNodes += count
}

// IncrementStartedServers increments started servers count
func (p *Progress) IncrementStartedServers(count int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.StartedServers += count
}

// IncrementStartedClients increments started clients count
func (p *Progress) IncrementStartedClients(count int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.StartedClients += count
}

// IncrementCompleted increments completed tests count
func (p *Progress) IncrementCompleted(count int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.CompletedTests += count
}

// IncrementFailed increments failed tests count
func (p *Progress) IncrementFailed(count int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.FailedTests += count
}

// IncrementCollected increments collected results count
func (p *Progress) IncrementCollected(count int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.CollectedResults += count
}

// AddError adds an error message
func (p *Progress) AddError(err string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Errors = append(p.Errors, err)
}

// GetSummary returns a summary string
func (p *Progress) GetSummary() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	elapsed := time.Since(p.StartTime)
	phaseElapsed := time.Since(p.PhaseStart)

	summary := fmt.Sprintf(`
Test Progress Summary
=====================
Phase: %s (elapsed: %s)
Total Elapsed: %s

Nodes:
  Connected: %d/%d
  Prepared:  %d/%d

Servers:
  Started: %d/%d

Clients:
  Started:   %d/%d
  Completed: %d/%d
  Failed:    %d/%d

Results:
  Collected: %d/%d

Errors: %d
`,
		p.CurrentPhase, phaseElapsed.Round(time.Second),
		elapsed.Round(time.Second),
		p.ConnectedNodes, p.TotalNodes,
		p.PreparedNodes, p.TotalNodes,
		p.StartedServers, p.TotalServers,
		p.StartedClients, p.TotalClients,
		p.CompletedTests, p.TotalTests,
		p.FailedTests, p.TotalTests,
		p.CollectedResults, p.TotalTests,
		len(p.Errors),
	)

	return summary
}

// GetPercentComplete returns the overall completion percentage
func (p *Progress) GetPercentComplete() float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.TotalTests == 0 {
		return 0
	}

	return float64(p.CompletedTests+p.FailedTests) / float64(p.TotalTests) * 100
}

// GetPhasePercent returns the current phase completion percentage
func (p *Progress) GetPhasePercent() float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	switch p.CurrentPhase {
	case "connecting":
		if p.TotalNodes == 0 {
			return 0
		}
		return float64(p.ConnectedNodes) / float64(p.TotalNodes) * 100
	case "preparing":
		if p.TotalNodes == 0 {
			return 0
		}
		return float64(p.PreparedNodes) / float64(p.TotalNodes) * 100
	case "starting_servers":
		if p.TotalServers == 0 {
			return 0
		}
		return float64(p.StartedServers) / float64(p.TotalServers) * 100
	case "starting_clients":
		if p.TotalClients == 0 {
			return 0
		}
		return float64(p.StartedClients) / float64(p.TotalClients) * 100
	case "collecting":
		if p.TotalTests == 0 {
			return 0
		}
		return float64(p.CollectedResults) / float64(p.TotalTests) * 100
	default:
		return 0
	}
}

// Print prints a progress update
func (p *Progress) Print() {
	p.mu.RLock()
	defer p.mu.RUnlock()

	percent := p.GetPercentComplete()
	phasePercent := p.GetPhasePercent()

	fmt.Printf("[%s] Phase: %s (%.1f%%) | Overall: %.1f%% | Completed: %d/%d | Failed: %d\n",
		time.Since(p.StartTime).Round(time.Second),
		p.CurrentPhase,
		phasePercent,
		percent,
		p.CompletedTests,
		p.TotalTests,
		p.FailedTests,
	)
}
