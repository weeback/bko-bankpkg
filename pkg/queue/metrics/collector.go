package metrics

import (
	"sync"
)

var (
	globalCollector *MetricsCollector
	once            sync.Once
)

// GetCollector returns the global metrics collector instance
func GetCollector() *MetricsCollector {
	once.Do(func() {
		globalCollector = NewMetricsCollector()
	})
	return globalCollector
}

// RecordQueueCreated initializes metrics for a new queue
func (mc *MetricsCollector) RecordQueueCreated(queueName string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if _, exists := mc.queues[queueName]; !exists {
		mc.queues[queueName] = &QueueMetrics{
			QueueName:     queueName,
			ServerMetrics: make(map[string]*ServerMetrics),
		}
	}
}

// UpdateQueueDepth updates the current queue depth
func (mc *MetricsCollector) UpdateQueueDepth(queueName string, depth int64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if qm, exists := mc.queues[queueName]; exists {
		qm.mu.Lock()
		qm.QueueDepth = depth
		qm.mu.Unlock()
	}
}

// GetAllMetrics returns a snapshot of all queue metrics
func (mc *MetricsCollector) GetAllMetrics() map[string]*QueueMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	snapshot := make(map[string]*QueueMetrics, len(mc.queues))
	for name, metrics := range mc.queues {
		metrics.mu.RLock()
		snapshot[name] = &QueueMetrics{
			QueueName:     metrics.QueueName,
			QueueDepth:    metrics.QueueDepth,
			TotalJobs:     metrics.TotalJobs,
			WaitingJobs:   metrics.WaitingJobs,
			ProcessedJobs: metrics.ProcessedJobs,
			FailedJobs:    metrics.FailedJobs,
			TotalLatency:  metrics.TotalLatency,
			MaxLatency:    metrics.MaxLatency,
			MinLatency:    metrics.MinLatency,
			AvgLatency:    metrics.AvgLatency,
		}
		metrics.mu.RUnlock()
	}
	return snapshot
}
