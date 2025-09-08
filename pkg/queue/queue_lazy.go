package queue

import (
	"time"

	"github.com/weeback/bko-bankpkg/pkg/queue/metrics"
)

func lazyGuys() queueInter {
	return &lazy{}

}

// lazy is a type that implements the queueInter interface (mirrors of workers).
//
// all method of laze always return happy result.
// And them not process any thing.
type lazy struct{}

func (*lazy) setPriorityMode(mode Priority) {}

func (*lazy) setTimeout(d time.Duration) {}

func (*lazy) updateMaxConcurrent(maxConcurrent int) {}

func (*lazy) getConcurrentStatus() (int, int, string) {
	return 0, 0, "FREE"
}

func (*lazy) listen(string, string, []byte, Option) <-chan *recv {
	return nil
}

func (*lazy) GetMetrics() *metrics.QueueMetrics {
	return nil
}
