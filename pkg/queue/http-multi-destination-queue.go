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

type MultiDestination interface {
	SetPriorityMode(mode Priority)
	SetRequestTimeout(timeout time.Duration)
	SetMaxConcurrent(maxConcurrent int)                        // Set maximum number of concurrent requests
	GetConcurrentStatus() (free int, total int, status string) // Get the current status of concurrent processing

	Get(ctx context.Context, v any, url string, opts ...Option) error
	Post(ctx context.Context, v any, url string, body []byte, opts ...Option) error
	PostWithFunc(ctx context.Context, fn func([]byte) error, url string, body []byte, opts ...Option) error
}

func NewMultiDestinationQueue(name string) MultiDestination {
	return &multiDestinationQueue{
		queue: bindPagodaQueue(name),
	}
}

type multiDestinationQueue struct {
	queue queueInter
}

func (q *multiDestinationQueue) SetPriorityMode(mode Priority) {
	logger.NewEntry().
		Debug("multiDestinationQueue.SetPriorityMode() called", zap.String("mode", string(mode)))
	// No-op for multidestination queue
}

func (q *multiDestinationQueue) SetRequestTimeout(d time.Duration) {
	logger.NewEntry().
		Debug("multiDestinationQueue.SetRequestTimeout() called", zap.Duration("timeout", d))
	// Update worker with new timeout value
	q.queue.setTimeout(d)
}

func (q *multiDestinationQueue) SetMaxConcurrent(maxConcurrent int) {
	logger.NewEntry().
		Debug("multiDestinationQueue.SetMaxConcurrent() called", zap.Int("maxConcurrent", maxConcurrent))
	// Update worker with new max concurrent value
	q.queue.updateMaxConcurrent(maxConcurrent)
}

func (q *multiDestinationQueue) GetConcurrentStatus() (free int, total int, status string) {
	return 0, 0, "UNKNOWN"
}

func (q *multiDestinationQueue) Get(ctx context.Context, v any, destURL string, opts ...Option) error {

	log := logger.GetLoggerFromContext(ctx).
		With(zap.String(logger.KeyFunctionName, "multiDestinationQueue.Get")).
		With(zap.String("destination", destURL))

	// validate data before processing
	if reflect.TypeOf(v).Kind() != reflect.Pointer {
		return fmt.Errorf("v must be a pointer")
	}
	// validate URL before processing
	urlParsed, err := url.Parse(destURL)
	if err != nil || urlParsed.Scheme == "" || urlParsed.Host == "" {
		return fmt.Errorf("invalid URL: %s", destURL)
	}
	// logging on debug
	defer func(t time.Time) {
		if r := recover(); r != nil {
			log.Error("panic recovered", zap.Any("error", r))
		}
		log.Debug("HTTP-QUEUE", zap.String("urlParsed", urlParsed.String()),
			zap.String("httpMethod", http.MethodGet), zap.Duration("duration", time.Since(t)))
	}(time.Now())

	var (
		opt = WithMultiOptions(opts...)
	)

	select {
	case r, ok := <-q.queue.listen(http.MethodGet, urlParsed.String(), nil, opt):
		log.Debug("listen() received")
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
		log.Debug("context deadline exceeded")
		return ctx.Err()
	}
}

func (q *multiDestinationQueue) Post(ctx context.Context, v any, destURL string, body []byte, opts ...Option) error {
	log := logger.GetLoggerFromContext(ctx).With(
		zap.String(logger.KeyFunctionName, "multiDestinationQueue.Post"),
		zap.String("destination", destURL))

	// validate data before processing
	if reflect.TypeOf(v).Kind() != reflect.Pointer {
		return fmt.Errorf("v must be a pointer")
	}
	// validate URL before processing
	urlParsed, err := url.Parse(destURL)
	if err != nil || urlParsed.Scheme == "" || urlParsed.Host == "" {
		return fmt.Errorf("invalid URL: %s", destURL)
	}
	// logging on debug
	defer func(t time.Time) {
		if r := recover(); r != nil {
			log.Error("panic recovered", zap.Any("error", r))
		}
		log.Debug("HTTP-QUEUE", zap.String("urlParsed", urlParsed.String()),
			zap.String("httpMethod", http.MethodPost), zap.Duration("duration", time.Since(t)))
	}(time.Now())

	var (
		opt = WithMultiOptions(opts...)
	)

	select {
	case r, ok := <-q.queue.listen(http.MethodPost, urlParsed.String(), body, opt):
		log.Debug("listen() received")
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
		log.Debug("context deadline exceeded")
		return ctx.Err()
	}
}

func (q *multiDestinationQueue) PostWithFunc(ctx context.Context, fn func([]byte) error, destURL string, body []byte, opts ...Option) error {
	log := logger.GetLoggerFromContext(ctx).With(
		zap.String(logger.KeyFunctionName, "multiDestinationQueue.PostWithFunc"),
		zap.String("destination", destURL))

	// validate URL before processing
	urlParsed, err := url.Parse(destURL)
	if err != nil || urlParsed.Scheme == "" || urlParsed.Host == "" {
		return fmt.Errorf("invalid URL: %s", destURL)
	}
	// logging on debug
	defer func(t time.Time) {
		if r := recover(); r != nil {
			log.Error("panic recovered", zap.Any("error", r))
		}
		log.Debug("HTTP-QUEUE", zap.String("urlParsed", urlParsed.String()),
			zap.String("httpMethod", http.MethodPost), zap.Duration("duration", time.Since(t)))
	}(time.Now())

	var (
		opt = WithMultiOptions(opts...)
	)
	select {
	case r, ok := <-q.queue.listen(http.MethodPost, urlParsed.String(), body, opt):
		log.Debug("listen() received")
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
			return fn(r.Payload)
		default:
			// try to match the status in the options
			if slices.Contains(opt.AcceptStatus, r.HttpStatusCode) {
				return fn(r.Payload)
			}
			return fmt.Errorf("http status: %s - body: %s", r.HttpStatus, string(r.Payload))
		}
	case <-ctx.Done():
		log.Debug("context deadline exceeded")
		return ctx.Err()
	}
}
