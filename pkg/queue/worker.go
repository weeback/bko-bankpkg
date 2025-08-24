package queue

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"
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

	// Kiểm soát lưu lượng request
	activeSem     chan struct{} // Semaphore để giới hạn số lượng request đồng thời
	maxConcurrent int           // Số lượng request tối đa có thể thực hiện đồng thời

	// Deprecated
	// primaryDesc   *url.URL
}

//	starts the worker. It listens for incoming jobs and processes them.
//
// This function must be called in a goroutine.
func (w *worker) init() *worker {

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
		log.Printf("delete %s from publicQueue because jobs channel closed", w.name)
		delete(publicQueue, w.name)
		log.Printf("publicQueue to delete success")
	}()

	return w
}

// Deprecated: no used
// func (w *worker)stop {
// 	defer func() {
// 		if r := recover(); r != nil {
// 			println("worker.stop() panic recovered")
// 		}
// 	}()
// 	w.signal <- os.Interrupt
// }

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
				log.Printf("worker stop by Interrupt signal received")
				return
			}
		case p, ok := <-w.jobs:
			if !ok {
				log.Printf("jobs channel closed")
				return
			}
			go func(bp *job, limit int) {

				// use the original URL for the first vote
				// to avoid the URL is changed by the vote()
				templateURL := bp.fullURL

				// loop for the job
				for i := 1; true; i++ {
					// vote the URL host
					if voted, label, err := w.vote(templateURL); err != nil {
						log.Printf("job processed method=%s fullURL=%s vote= error: %v", bp.method, bp.fullURL, err)
						bp.serverLabel = labelPrimary
						bp.fullURL = w.withPrimary(bp.fullURL)
						log.Printf("job processed method=%s fullURL=%s voted=%s retry with primary server",
							bp.method, bp.fullURL, bp.serverLabel)
					} else {
						bp.serverLabel = label
						bp.fullURL = voted
						log.Printf("job processed method=%s fullURL=%s voted=%s", bp.method, bp.fullURL, bp.serverLabel)
					}

					rv := w.call(bp)

					// loop condition:
					if rv.err != nil && i < limit {
						// set this url to not responding (mockup response time=30s)
						w.setLastTook(bp.serverLabel, 30*time.Second)
						// vote again to new url
						log.Printf("job processed method=%s fullURL=%s voted=%s error=%v try voting to new url (%d)",
							bp.method, bp.fullURL, bp.serverLabel, rv.err, i)
						continue
					}
					if rv.HttpStatusCode == 0 {
						rv.HttpStatusCode = http.StatusBadGateway
					}
					// send the response to the channel
					if closed := recvSafe(bp.recv, &rv); closed {
						log.Printf("job processed method=%s fullURL=%s receive channel closed", bp.method, bp.fullURL)
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
		log.Printf("[ERROR] failed to parse url: %s", template)
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

	// Lấy slot từ semaphore nếu có giới hạn lưu lượng
	release, err := w.acquireSemaphore(w.timeout / 2) // Sử dụng nửa timeout cho việc chờ slot
	if err != nil {
		return recv{err: err}
	}
	// Đảm bảo sẽ giải phóng slot khi hàm kết thúc
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
		log.Printf("worker.call() method=%s fullURL=%s voted=%s duration=%s",
			work.method, work.fullURL, work.serverLabel, time.Since(t).String())

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
			log.Printf("worker.listen() context deadline exceeded, but receive channel closed")
		}
		return pack.recv
	}
}
