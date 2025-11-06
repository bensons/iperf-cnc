package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"

	"github.com/bensons/iperf-cnc/internal/controller/aggregator"
)

// OutputData contains all data to be written
type OutputData struct {
	Summary *aggregator.Summary      `json:"summary"`
	Results []*aggregator.TestResult `json:"results"`
}

// Writer handles output generation
type Writer struct {
	jsonFile string
	csvFile  string
}

// NewWriter creates a new output writer
func NewWriter(jsonFile, csvFile string) *Writer {
	return &Writer{
		jsonFile: jsonFile,
		csvFile:  csvFile,
	}
}

// WriteJSON writes results to a JSON file
func (w *Writer) WriteJSON(data *OutputData) error {
	if w.jsonFile == "" {
		return nil // JSON output not requested
	}

	file, err := os.Create(w.jsonFile)
	if err != nil {
		return fmt.Errorf("failed to create JSON file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			fmt.Printf("Warning: failed to close JSON file: %v\n", err)
		}
	}()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
}

// WriteCSV writes results to a CSV file
func (w *Writer) WriteCSV(results []*aggregator.TestResult) error {
	if w.csvFile == "" {
		return nil // CSV output not requested
	}

	file, err := os.Create(w.csvFile)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			fmt.Printf("Warning: failed to close CSV file: %v\n", err)
		}
	}()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{
		"test_id",
		"source_node",
		"dest_node",
		"status",
		"start_time",
		"end_time",
		"duration_seconds",
		"throughput_bps",
		"throughput_mbps",
		"throughput_gbps",
		"retransmits",
		"error_message",
	}

	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write results
	for _, result := range results {
		row := []string{
			result.TestID,
			result.SourceNode,
			result.DestNode,
			result.Status,
			fmt.Sprintf("%d", result.StartTime),
			fmt.Sprintf("%d", result.EndTime),
			fmt.Sprintf("%d", result.Duration),
			fmt.Sprintf("%.0f", result.ThroughputBps),
			fmt.Sprintf("%.2f", result.ThroughputBps/1e6),
			fmt.Sprintf("%.4f", result.ThroughputBps/1e9),
			fmt.Sprintf("%d", result.Retransmits),
			result.ErrorMessage,
		}

		if err := writer.Write(row); err != nil {
			return fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	return nil
}

// WriteAll writes both JSON and CSV outputs
func (w *Writer) WriteAll(summary *aggregator.Summary, results []*aggregator.TestResult) error {
	data := &OutputData{
		Summary: summary,
		Results: results,
	}

	if err := w.WriteJSON(data); err != nil {
		return fmt.Errorf("failed to write JSON: %w", err)
	}

	if err := w.WriteCSV(results); err != nil {
		return fmt.Errorf("failed to write CSV: %w", err)
	}

	return nil
}
