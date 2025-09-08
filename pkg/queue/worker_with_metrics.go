package queue

import (
	"github.com/weeback/bko-bankpkg/pkg/queue/metrics"
)

type metricsWorker struct {
	*worker
	collector *metrics.MetricsCollector
}

// NewWorkerWithMetrics creates a new worker with metrics collection enabled
func NewWorkerWithMetrics(w *worker) *metricsWorker {
	mw := &metricsWorker{
		worker:    w,
		collector: metrics.GetCollector(),
	}
	// Initialize metrics for this queue
	mw.collector.RecordQueueCreated(w.name)

	return mw
}

// init wraps the original worker init with metrics
func (mw *metricsWorker) init() *metricsWorker {
	mw.worker.init()
	return mw
}

// func (mw *metricsWorker) call(work *job) recv {
// 	startTime := time.Now()
// 	result := mw.worker.call(work)
// 	duration := time.Since(startTime)

// 	// Record metrics for the job processing
// 	mw.collector.RecordJobProcessed(
// 		mw.name,
// 		work.serverLabel,
// 		duration,
// 		result.HttpStatusCode,
// 		result.err,
// 	)

// 	return result
// }

func (mw *metricsWorker) listen(method string, fullURL string, body []byte, opt Option) <-chan *recv {
	// Update queue depth before adding new job
	mw.collector.UpdateQueueDepth(mw.name, int64(len(mw.jobs)))

	return mw.worker.listen(method, fullURL, body, opt)
}

// GetMetrics returns the current metrics for this worker's queue
func (mw *metricsWorker) GetMetrics() *metrics.QueueMetrics {
	return mw.collector.GetQueueMetrics(mw.name)
}
