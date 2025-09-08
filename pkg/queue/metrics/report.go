package metrics

import (
	"fmt"
	"strings"
	"time"
)

// FormatDuration formats a duration in a human-readable format
func FormatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%.2fµs", float64(d.Microseconds()))
	}
	if d < time.Second {
		return fmt.Sprintf("%.2fms", float64(d.Milliseconds()))
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

// GenerateMetricsReport creates a formatted string report of current metrics
func (mc *MetricsCollector) GenerateMetricsReport() string {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	var report strings.Builder
	report.WriteString("\n=== Queue Performance Metrics ===\n")

	for queueName, qm := range mc.queues {
		qm.mu.RLock()
		report.WriteString(fmt.Sprintf("\nQueue: %s\n", queueName))
		report.WriteString(fmt.Sprintf("  Current Depth: %d\n", qm.QueueDepth))
		report.WriteString(fmt.Sprintf("  Total Jobs: %d\n", qm.TotalJobs))
		report.WriteString(fmt.Sprintf("  Processed: %d\n", qm.ProcessedJobs))
		report.WriteString(fmt.Sprintf("  Failed: %d\n", qm.FailedJobs))
		report.WriteString(fmt.Sprintf("  Average Latency: %s\n", FormatDuration(qm.AvgLatency)))
		report.WriteString(fmt.Sprintf("  Max Latency: %s\n", FormatDuration(qm.MaxLatency)))

		report.WriteString("\n  Server Metrics:\n")
		for _, sm := range qm.ServerMetrics {
			report.WriteString(fmt.Sprintf("    %s:\n", sm.ServerLabel))
			report.WriteString(fmt.Sprintf("      Requests: %d\n", sm.RequestCount))
			report.WriteString(fmt.Sprintf("      Errors: %d\n", sm.ErrorCount))
			report.WriteString(fmt.Sprintf("      Avg Latency: %s\n", FormatDuration(sm.AvgLatency)))
			if sm.LastError != nil {
				report.WriteString(fmt.Sprintf("      Last Error: %s\n", sm.LastError))
			}
		}
		qm.mu.RUnlock()
	}

	return report.String()
}

// GenerateMetricsTable creates a formatted table of current metrics
func (mc *MetricsCollector) GenerateMetricsTable() string {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	var table strings.Builder
	header := "+================+===========+===========+===========+================+================+\n"
	format := "| %-14s | %9d | %9d | %9d | %14s | %14s |\n"

	table.WriteString(header)
	table.WriteString("|     Queue     |   Depth   |   Total   | Processed |  Avg Latency  |  Max Latency  |\n")
	table.WriteString(header)

	for _, qm := range mc.queues {
		qm.mu.RLock()
		table.WriteString(fmt.Sprintf(format,
			truncateString(qm.QueueName, 14),
			qm.QueueDepth,
			qm.TotalJobs,
			qm.ProcessedJobs,
			FormatDuration(qm.AvgLatency),
			FormatDuration(qm.MaxLatency),
		))
		qm.mu.RUnlock()
	}
	table.WriteString(header)

	return table.String()
}

// truncateString truncates a string to the specified length
func truncateString(s string, length int) string {
	if len(s) <= length {
		return s
	}
	return s[:length-3] + "..."
}
