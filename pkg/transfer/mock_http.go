package transfer

import (
	"context"
	"errors"
	"time"

	"github.com/weeback/bko-bankpkg/pkg/queue"
)

// MockHTTP implements queue.HTTP interface for testing
type MockHTTP struct {
	postFunc            func(ctx context.Context, result any, payload []byte) error
	getFunc             func(ctx context.Context, result any) error
	getConcurrentStatus func() (free, total int, status string)
	forceError          bool
	forceOccupied       bool
	forceBusy           bool
	postCallCount       int
	getCallCount        int
	getConcStatusCount  int
	priorityMode        queue.Priority
	requestTimeout      time.Duration
	maxConcurrent       int
}

func NewMockHTTP() *MockHTTP {
	return &MockHTTP{
		postFunc: func(ctx context.Context, result any, payload []byte) error {
			return nil
		},
		getFunc: func(ctx context.Context, result any) error {
			return nil
		},
		getConcurrentStatus: func() (free, total int, status string) {
			return 5, 10, "AVAILABLE"
		},
		maxConcurrent: 10,
	}
}

func (m *MockHTTP) Post(ctx context.Context, result any, payload []byte, opts ...queue.Option) error {
	m.postCallCount++
	if m.forceError {
		return errors.New("forced error")
	}
	return m.postFunc(ctx, result, payload)
}

func (m *MockHTTP) Get(ctx context.Context, result any, opts ...queue.Option) error {
	m.getCallCount++
	if m.forceError {
		return errors.New("forced error")
	}
	return m.getFunc(ctx, result)
}

func (m *MockHTTP) GetConcurrentStatus() (free, total int, status string) {
	m.getConcStatusCount++
	if m.forceOccupied {
		return 0, m.maxConcurrent, "OCCUPIED"
	}
	if m.forceBusy {
		return 2, m.maxConcurrent, "BUSY"
	}
	current := m.postCallCount
	if current > m.maxConcurrent {
		current = m.maxConcurrent
	}
	free = m.maxConcurrent - current
	if free < 0 {
		free = 0
	}
	return free, m.maxConcurrent, "AVAILABLE"
}

func (m *MockHTTP) SetPriorityMode(mode queue.Priority) {
	m.priorityMode = mode
}

func (m *MockHTTP) SetRequestTimeout(timeout time.Duration) {
	m.requestTimeout = timeout
}

func (m *MockHTTP) SetMaxConcurrent(maxConcurrent int) {
	m.maxConcurrent = maxConcurrent
}

func (m *MockHTTP) String() string {
	return "MockHTTP"
}

func (m *MockHTTP) ResetCounters() {
	if m.forceError {
		m.forceError = false
	}
	m.forceOccupied = false
	m.forceBusy = false
	m.postCallCount = 0
	m.getCallCount = 0
	m.getConcStatusCount = 0
}
