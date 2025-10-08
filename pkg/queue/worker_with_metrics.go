package queue

import (
	"time"

	"github.com/weeback/bko-bankpkg/pkg/queue/metrics"
)

type metricsWorker struct {
	inter     queueInter
	collector *metrics.MetricsCollector
}

func NewPagodaWithMetrics(p *pagoda) *metricsWorker {
	mw := &metricsWorker{
		inter:     p,
		collector: metrics.GetCollector(),
	}
	// Initialize metrics for this queue
	mw.collector.RecordQueueCreated(p.name)

	return mw
}

// NewWorkerWithMetrics creates a new worker with metrics collection enabled
func NewWorkerWithMetrics(w *worker) *metricsWorker {
	mw := &metricsWorker{
		inter:     w,
		collector: metrics.GetCollector(),
	}
	// Initialize metrics for this queue
	mw.collector.RecordQueueCreated(w.name)

	return mw
}

// init wraps the original worker init with metrics
func (mw *metricsWorker) init() queueInter {
	mw.inter.init()
	return mw
}

func (mw *metricsWorker) getName() string {
	return mw.inter.getName()
}

func (mw *metricsWorker) getJobsLength() int64 {
	return mw.inter.getJobsLength()
}

func (mw *metricsWorker) setPriorityMode(mode Priority) {
	mw.inter.setPriorityMode(mode)
}

func (mw *metricsWorker) setTimeout(d time.Duration) {
	mw.inter.setTimeout(d)
}

func (mw *metricsWorker) updateMaxConcurrent(maxConcurrent int) {
	mw.inter.updateMaxConcurrent(maxConcurrent)
}

func (mw *metricsWorker) getConcurrentStatus() (int, int, string) {
	return mw.inter.getConcurrentStatus()
}

func (mw *metricsWorker) listen(method string, fullURL string, body []byte, opt Option) <-chan *recv {
	// Update queue depth before adding new job
	mw.collector.UpdateQueueDepth(mw.inter.getName(), mw.inter.getJobsLength())

	return mw.inter.listen(method, fullURL, body, opt)
}

// GetMetrics returns the current metrics for this worker's queue
func (mw *metricsWorker) GetMetrics() *metrics.QueueMetrics {
	return mw.collector.GetQueueMetrics(mw.inter.getName())
}
