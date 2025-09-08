package metrics

import (
	"sync"
	"time"
)

// QueueMetrics represents metrics for a single queue
type QueueMetrics struct {
	mu sync.RWMutex

	// Queue metrics
	QueueName     string
	QueueDepth    int64
	TotalJobs     int64
	WaitingJobs   int64
	ProcessedJobs int64
	FailedJobs    int64

	// Latency metrics
	TotalLatency   time.Duration
	MaxLatency     time.Duration
	MinLatency     time.Duration
	AvgLatency     time.Duration
	LastUpdateTime time.Time

	// Server metrics
	ServerMetrics map[string]*ServerMetrics
}

// ServerMetrics represents metrics for a single server
type ServerMetrics struct {
	ServerLabel    string
	RequestCount   int64
	ErrorCount     int64
	TotalLatency   time.Duration
	MaxLatency     time.Duration
	MinLatency     time.Duration
	AvgLatency     time.Duration
	LastStatusCode int
	LastError      error
	LastUpdateTime time.Time
}

// MetricsCollector handles metrics collection and aggregation
type MetricsCollector struct {
	mu     sync.RWMutex
	queues map[string]*QueueMetrics
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		queues: make(map[string]*QueueMetrics),
	}
}

// GetQueueMetrics returns metrics for a specific queue
func (mc *MetricsCollector) GetQueueMetrics(queueName string) *QueueMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.queues[queueName]
}

// RecordJobProcessed records metrics for a processed job
func (mc *MetricsCollector) RecordJobProcessed(queueName, serverLabel string, latency time.Duration, statusCode int, err error) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	qm, ok := mc.queues[queueName]
	if !ok {
		qm = &QueueMetrics{
			QueueName:      queueName,
			ServerMetrics:  make(map[string]*ServerMetrics),
			LastUpdateTime: time.Now(),
		}
		mc.queues[queueName] = qm
	}

	qm.mu.Lock()
	defer qm.mu.Unlock()

	// Update queue metrics
	qm.ProcessedJobs++
	qm.TotalLatency += latency
	qm.AvgLatency = qm.TotalLatency / time.Duration(qm.ProcessedJobs)

	if latency > qm.MaxLatency {
		qm.MaxLatency = latency
	}
	if qm.MinLatency == 0 || latency < qm.MinLatency {
		qm.MinLatency = latency
	}

	// Update server metrics
	sm, ok := qm.ServerMetrics[serverLabel]
	if !ok {
		sm = &ServerMetrics{
			ServerLabel:    serverLabel,
			LastUpdateTime: time.Now(),
		}
		qm.ServerMetrics[serverLabel] = sm
	}

	sm.RequestCount++
	sm.TotalLatency += latency
	sm.AvgLatency = sm.TotalLatency / time.Duration(sm.RequestCount)

	if latency > sm.MaxLatency {
		sm.MaxLatency = latency
	}
	if sm.MinLatency == 0 || latency < sm.MinLatency {
		sm.MinLatency = latency
	}

	sm.LastStatusCode = statusCode
	if err != nil {
		sm.ErrorCount++
		sm.LastError = err
	}

	sm.LastUpdateTime = time.Now()
}
