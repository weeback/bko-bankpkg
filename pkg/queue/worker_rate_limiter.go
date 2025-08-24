package queue

import (
	"fmt"
	"log"
	"time"
)

// updateMaxConcurrent cập nhật giới hạn số lượng request đồng thời
func (w *worker) updateMaxConcurrent(maxConcurrent int) {
	if maxConcurrent <= 0 {
		// Nếu giá trị không hợp lệ hoặc = 0 (không giới hạn), không làm gì cả
		return
	}

	// Nếu giá trị đã thiết lập và bằng với giá trị hiện tại, không cần thay đổi
	if w.maxConcurrent == maxConcurrent && w.activeSem != nil {
		return
	}

	// Cập nhật giới hạn mới
	w.maxConcurrent = maxConcurrent

	// Tạo semaphore mới nếu chưa có hoặc kích thước thay đổi
	if w.activeSem == nil || cap(w.activeSem) != maxConcurrent {
		// Tạo semaphore mới với kích thước mới
		w.activeSem = make(chan struct{}, maxConcurrent)
		log.Printf("Rate limiter updated: max concurrent requests = %d", maxConcurrent)
	}
}

func (w *worker) getConcurrentStatus() (int, int, string) {
	n := w.maxConcurrent - len(w.activeSem)
	status := "FREE"
	log.Printf("Current concurrent requests: %d/%d", n, w.maxConcurrent)
	switch {
	case n == w.maxConcurrent:
		log.Printf("All slots are free")
		status = "FREE"
	case n > w.maxConcurrent/10:
		log.Printf("Many slots are free")
		status = "AVAILABLE"
	case n > 0:
		log.Printf("Some slots are free")
		status = "BUSY"
	default:
		log.Printf("All slots are occupied")
		status = "OCCUPIED"
	}
	return n, w.maxConcurrent, status
}

// acquireSemaphore lấy một slot từ semaphore, trả về hàm giải phóng
// Trả về error nếu không thể lấy slot trong thời gian chờ
func (w *worker) acquireSemaphore(timeout time.Duration) (release func(), err error) {
	// Nếu không có giới hạn, trả về hàm trống
	if w.maxConcurrent <= 0 || w.activeSem == nil {
		return func() {}, nil
	}

	// Thử lấy semaphore với timeout
	select {
	case w.activeSem <- struct{}{}:
		// Đã lấy được slot
		return func() {
			// Hàm giải phóng slot
			select {
			case <-w.activeSem:
				// Đã giải phóng slot
			default:
				// Semaphore có thể đã được đóng
			}
		}, nil
	case <-time.After(timeout):
		// Timeout khi chờ slot
		return nil, fmt.Errorf("rate limit exceeded: too many concurrent requests")
	}
}
