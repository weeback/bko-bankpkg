package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"slices"
	"time"

	"github.com/weeback/bko-bankpkg/pkg/logger"
	"go.uber.org/zap"
)

type HTTP interface {
	String() string
	SetPriorityMode(mode Priority)
	SetRequestTimeout(timeout time.Duration)
	SetMaxConcurrent(maxConcurrent int)                        // Set maximum number of concurrent requests
	GetConcurrentStatus() (free int, total int, status string) // Get the current status of concurrent processing

	Get(ctx context.Context, v any, opts ...Option) error
	Post(ctx context.Context, v any, body []byte, opts ...Option) error
}

// NewHttpQueue creates a new HTTP queue, it takes a full URL as input.
// Withs options to configure replication and more features
func NewHttpQueue(name, fullURL string) HTTP {
	desc, err := url.Parse(fullURL)
	if err != nil {
		return &httpQueue{queue: lazyGuys(), templateURL: &url.URL{}, err: err}
	}

	return &httpQueue{
		queue:       bindHttpQueue(name, desc),
		templateURL: desc,
		err:         nil,
	}
}

// NewHttpQueueWithMultiHost: creates a new HTTP queue with multiple hosts
// fullURL is template url, hosts are the replication servers
func NewHttpQueueWithMultiHost(name, fullURL string, hosts ...*url.URL) HTTP {
	desc, err := url.Parse(fullURL)
	if err != nil {
		return &httpQueue{queue: lazyGuys(), templateURL: &url.URL{}, err: err}
	}

	if len(hosts) == 0 {
		return &httpQueue{queue: lazyGuys(), templateURL: &url.URL{}, err: fmt.Errorf("missing hosts")}
	}

	return &httpQueue{
		queue:       bindHttpQueue(name, hosts...),
		templateURL: desc,
		err:         nil,
	}
}

type httpQueue struct {
	templateURL *url.URL
	err         error

	// with one of HOST:PORT by url to get the worker
	// to process the request
	queue queueInter
}

func (q *httpQueue) validate() error {
	if q.err != nil {
		return q.err
	}
	if q.templateURL.Scheme != "http" && q.templateURL.Scheme != "https" {
		if q.templateURL.Scheme == "" {
			q.templateURL.Scheme = "<empty>"
		}
		return fmt.Errorf("invalid scheme: %s", q.templateURL.Scheme)
	}
	if q.templateURL.Host == "" {
		return fmt.Errorf("missing host")
	}
	return nil
}

func (q *httpQueue) String() string {
	log := logger.NewEntry()
	log.Debug("httpQueue.String() called")

	// validate data before processing
	if err := q.validate(); err != nil {
		return err.Error()
	}
	w, ok := q.queue.(*worker)
	if ok {
		for i := 1; i <= len(w.primaryServer)+len(w.secondaryServerList); i++ {
			val, label, _ := w.vote(q.templateURL.String())
			log.Debug("httpQueue.String() vote",
				zap.String("voted", label),
				zap.String("vote", val))
		}
	}
	// toJson is defined in internal/queue/util.go
	return toJson(map[string]any{"templateURL": q.templateURL.String(), "err": q.err})
}

func (q *httpQueue) SetPriorityMode(mode Priority) {
	log := logger.NewEntry()
	log.Debug("httpQueue.SetPriorityMode() called")

	// validate data before processing
	if err := q.validate(); err != nil {
		log.Debug("httpQueue.SetPriorityMode() error", zap.Error(err))
		return
	}
	q.queue.setPriorityMode(mode)
}

func (q *httpQueue) SetRequestTimeout(d time.Duration) {
	log := logger.NewEntry()
	log.Debug("httpQueue.SetRequestTimeout() called", zap.Duration("timeout", d))

	// validate data before processing
	if err := q.validate(); err != nil {
		log.Debug("httpQueue.SetRequestTimeout() error", zap.Error(err))
		return
	}

	// Update worker with new timeout value
	q.queue.setTimeout(d)
}

// SetMaxConcurrent sets the maximum number of concurrent requests
func (q *httpQueue) SetMaxConcurrent(maxConcurrent int) {
	log := logger.NewEntry()
	log.Debug("httpQueue.SetMaxConcurrent() called", zap.Int("maxConcurrent", maxConcurrent))

	// validate data before processing
	if err := q.validate(); err != nil {
		log.Debug("httpQueue.SetMaxConcurrent() error", zap.Error(err))
		return
	}

	// Update worker with new limit value
	q.queue.updateMaxConcurrent(maxConcurrent)
}

func (q *httpQueue) GetConcurrentStatus() (free int, total int, status string) {
	return q.queue.getConcurrentStatus()
}

func (q *httpQueue) Get(ctx context.Context, v any, opts ...Option) error {
	log := logger.GetLoggerFromContext(ctx)

	// validate data before processing
	if reflect.TypeOf(v).Kind() != reflect.Pointer {
		return fmt.Errorf("v must be a pointer")
	}

	var (
		opt = WithMultiOptions(opts...)
	)
	log.Debug("httpQueue.Get() called")

	select {
	case r, ok := <-q.queue.listen(http.MethodGet, q.templateURL.String(), nil, opt):
		log.Debug("httpQueue.Get() listen() received")
		// validate data before processing
		if !ok {
			return fmt.Errorf("queue closed")
		}
		if r.err != nil {
			return filterError(r.err)
		}
		if len(r.Payload) == 0 {
			return fmt.Errorf("http status: %s - response body", r.HttpStatus)
		}
		switch r.HttpStatusCode {
		case http.StatusOK:
			return json.Unmarshal(r.Payload, v) // default status OK
		default:
			// try to match the status in the options
			if slices.Contains(opt.AcceptStatus, r.HttpStatusCode) {
				return json.Unmarshal(r.Payload, v)
			}
			return fmt.Errorf("http status: %s - body: %s", r.HttpStatus, string(r.Payload))
		}
	case <-ctx.Done():
		log.Debug("httpQueue.Get() context deadline exceeded")
		return ctx.Err()
	}
}

func (q *httpQueue) Post(ctx context.Context, v any, body []byte, opts ...Option) error {
	log := logger.GetLoggerFromContext(ctx)

	// validate data before processing
	if reflect.TypeOf(v).Kind() != reflect.Pointer {
		return fmt.Errorf("v must be a pointer")
	}

	var (
		opt = WithMultiOptions(opts...)
	)
	log.Debug("httpQueue.Post() called")

	select {
	case r, ok := <-q.queue.listen(http.MethodPost, q.templateURL.String(), body, opt):
		log.Debug("httpQueue.Post() listen() received")
		// validate data before processing
		if !ok {
			return fmt.Errorf("queue closed")
		}
		if r.err != nil {
			return filterError(r.err)
		}
		if len(r.Payload) == 0 {
			return fmt.Errorf("http status: %s - response body", r.HttpStatus)
		}
		switch r.HttpStatusCode {
		case http.StatusOK:
			// default status OK
			return json.Unmarshal(r.Payload, v)
		default:
			// try to match the status in the options
			if slices.Contains(opt.AcceptStatus, r.HttpStatusCode) {
				return json.Unmarshal(r.Payload, v)
			}
			return fmt.Errorf("http status: %s - body: %s", r.HttpStatus, string(r.Payload))
		}
	case <-ctx.Done():
		log.Debug("httpQueue.Post() context deadline exceeded")
		return ctx.Err()
	}
}
