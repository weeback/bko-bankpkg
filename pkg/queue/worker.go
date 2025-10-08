package queue

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/weeback/bko-bankpkg/pkg/logger"
	"github.com/weeback/bko-bankpkg/pkg/queue/metrics"
	"go.uber.org/zap"
)

type worker struct {
	name                string
	code                int
	priorityMode        Priority
	primaryServer       []*counter
	secondaryServerList []*counter
	jobs                chan *job

	signal     chan os.Signal
	httpClient func() *http.Client
	timeout    time.Duration

	// Rate limiting control
	activeSem     chan struct{} // Semaphore to limit the number of concurrent requests
	maxConcurrent int           // Maximum number of concurrent requests

	// Deprecated
	// primaryDesc   *url.URL
}

//	starts the worker. It listens for incoming jobs and processes them.
//
// This function must be called in a goroutine.
func (w *worker) init() queueInter {

	// the primary server must be only 1
	if len(w.primaryServer) != 1 {
		panic("primary server must be only 1")
	}

	// set the worker status to started
	w.code = 1 // started

	// setup and start the worker
	go func() {
		// start the worker
		w.start()
		// when the worker not working:
		// remove the queue from the public queue
		log := logger.NewEntry()
		log.Debug("deleting from publicQueue - jobs channel closed", zap.String("worker_name", w.name))
		delete(publicQueue, w.name)
		log.Debug("publicQueue deletion successful")
	}()

	return w
}

func (w *worker) start() {

	defer func() {
		// set the worker status to stopped
		w.code = -1 // stopped
		// release the resources
		close(w.jobs)
	}()

	// this is a dummy call to free the counter.
	// this prevents the ide from giving syntax warnings no-used.
	w.freeCounter("do nothing", 0)

	// listen for incoming jobs
	for {
		select {
		case s := <-w.signal:
			if s == os.Interrupt {
				logger.NewEntry().Info("worker stopped - interrupt signal received")
				return
			}
		case p, ok := <-w.jobs:
			if !ok {
				logger.NewEntry().Info("jobs channel closed")
				return
			}
			go func(bp *job, limit int) {

				// use the original URL for the first vote
				// to avoid the URL is changed by the vote()
				templateURL := bp.fullURL

				// loop for the job
				log := logger.NewEntry()
				for i := 1; true; i++ {
					// vote the URL host
					if voted, label, err := w.vote(templateURL); err != nil {
						log.Debug("job processing - vote error",
							zap.String("method", bp.method),
							zap.String("url", bp.fullURL),
							zap.Error(err))
						bp.serverLabel = labelPrimary
						bp.fullURL = w.withPrimary(bp.fullURL)
						log.Debug("job processing - retrying with primary server",
							zap.String("method", bp.method),
							zap.String("url", bp.fullURL),
							zap.String("voted", bp.serverLabel))
					} else {
						bp.serverLabel = label
						bp.fullURL = voted
						log.Debug("job processing - vote successful",
							zap.String("method", bp.method),
							zap.String("url", bp.fullURL),
							zap.String("voted", bp.serverLabel))
					}

					rv := w.call(bp)

					// loop condition:
					if rv.err != nil && i < limit {
						// set this url to not responding (mockup response time=30s)
						w.setLastTook(bp.serverLabel, 30*time.Second)
						// vote again to new url
						log.Debug("job processing - retrying with new URL",
							zap.String("method", bp.method),
							zap.String("url", bp.fullURL),
							zap.String("voted", bp.serverLabel),
							zap.Error(rv.err),
							zap.Int("attempt", i))
						continue
					}
					if rv.HttpStatusCode == 0 {
						rv.HttpStatusCode = http.StatusBadGateway
					}
					// send the response to the channel
					if closed := recvSafe(bp.recv, &rv); closed {
						log.Debug("job processing - receive channel closed",
							zap.String("method", bp.method),
							zap.String("url", bp.fullURL))
					}
					// end to break
					return
				}
			}(p, 2*len(w.secondaryServerList)+1)

			// continues next job
		}
	}

}

func (w *worker) status() (s string) {

	// n is the number of jobs waiting to be processed
	n := len(w.jobs)
	defer func() {
		s = fmt.Sprintf("%s %5d", s, n)
	}()
	// if the worker is working, return the worker status based on the number of jobs
	if w.code == 1 && n > 0 {
		switch true {
		case n < 10:
			return "IDLE"
		case n < 100:
			return "WORKING"
		case n >= 100:
			return "BUSY"
		}
	}

	// return the worker status based on the code
	switch w.code {
	case 0:
		return "INACTIVE"
	case 1:
		return "STARTED"
	case -1:
		return "STOPPED"
	default:
		return "UNKNOWN"
	}
}

func (w *worker) setPriorityMode(mode Priority) {
	switch mode {
	case PriorityFrequency, PriorityLatency:
		w.priorityMode = mode
	case PriorityShortFrequency:
		w.priorityMode = PriorityFrequency
	case PriorityShortLatency:
		w.priorityMode = PriorityLatency
	default:
		w.priorityMode = PriorityFrequency
	}
}

func (w *worker) setTimeout(d time.Duration) {
	if d <= 0 {
		w.timeout = 30 * time.Second
	} else {
		w.timeout = d
	}
}

func (w *worker) withPrimary(template string) string {
	templateUrl, err := url.Parse(template)
	if err != nil {
		logger.NewEntry().Error("failed to parse url",
			zap.String("url", template),
			zap.Error(err))
		return template
	}
	templateUrl.Scheme = w.primaryServer[0].url.Scheme
	templateUrl.Host = w.primaryServer[0].url.Host
	return templateUrl.String()
}

func (w *worker) call(work *job) recv {
	var (
		r   *http.Request
		err error
	)
	ctx, cancel := context.WithTimeout(context.TODO(), w.timeout)
	defer cancel()

	// Acquire a slot from semaphore if rate limiting is enabled
	release, err := w.acquireSemaphore(w.timeout / 2) // Use half of timeout for waiting for a slot
	if err != nil {
		return recv{err: err}
	}
	// Make sure to release the slot when function completes
	defer release()

	switch work.method {
	case http.MethodGet:
		r, err = http.NewRequestWithContext(ctx, http.MethodGet, work.fullURL, nil)
		if err != nil {
			return recv{err: err}
		}
	default:
		r, err = http.NewRequestWithContext(ctx, http.MethodPost, work.fullURL, bytes.NewReader(work.body))
		if err != nil {
			return recv{err: err}
		}
	}

	// set the content type from p to r
	work.acceptContentType(r)
	// set the headers from p to r
	work.acceptHeaders(r)

	defer func(t time.Time) {
		logger.NewEntry().Debug("worker call completed",
			zap.String("method", work.method),
			zap.String("url", work.fullURL),
			zap.String("voted", work.serverLabel),
			zap.Duration("duration", time.Since(t)))

		// update the counter status.
		// only use freeCounter or setLastTook
		//
		// w.freeCounter(work.serverLabel, time.Since(t))
		//
		w.setLastTook(work.serverLabel, time.Since(t))

	}(time.Now())
	// send the request
	rp, err := w.httpClient().Do(r)
	if err != nil {
		// delay to simulate the delay for test context
		time.Sleep(50 * time.Millisecond)
		// send the error to the channel
		return recv{err: err}
	}
	// wrap to read data payload
	payload, err := func(r *http.Response) ([]byte, error) {
		defer func(Body io.ReadCloser) {
			if err := Body.Close(); err != nil {
				println("httpQueue.Get() close body error")
			}
		}(r.Body)
		// read data payload
		b, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		return b, nil
	}(rp)
	if err != nil {
		// send the error to the channel
		return recv{err: err}
	}
	//
	return recv{HttpStatusCode: rp.StatusCode, HttpStatus: rp.Status, Payload: payload}
}

func (w *worker) listen(method string, fullURL string, body []byte, opt Option) <-chan *recv {
	pack := job{
		method:  method,
		fullURL: fullURL,
		body:    body,
		recv:    make(chan *recv),
	}

	if opt.ContentType != "" &&
		pack.method != http.MethodGet /* method GET contentType is unavailable */ &&
		pack.contentType == "" /* first set to pack.contentType */ {
		pack.contentType = opt.ContentType
	}
	if len(opt.XHeader) > 0 {
		pack.headers = opt.XHeader
	}

	// print the queue status
	printQueueStatus(w.name)

	select {
	case w.jobs <- &pack:
		return pack.recv
	case <-time.After(w.timeout):
		if closed := recvSafe(pack.recv, &recv{
			err: fmt.Errorf("worker processing deadline exceeded"),
		}); closed {
			logger.NewEntry().Debug("worker.listen() - context deadline exceeded and receive channel closed")
		}
		return pack.recv
	}
}

func (w *worker) getName() string {
	return w.name
}

func (w *worker) getJobsLength() int64 {
	n := len(w.jobs)
	return int64(n)
}

// GetMetrics returns nil for base worker implementation
// The metrics worker implementation will override this
func (w *worker) GetMetrics() *metrics.QueueMetrics {
	return nil
}
