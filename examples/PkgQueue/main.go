package main

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/weeback/bko-bankpkg/pkg/queue"
)

// First time, read file `github.com/weeback/bko-bankpkg/pkg/queue/README.md` to understand the package
// Then, use the package in your code.
// Finally, run this example with `go run examples/PkgQueue/main.go`
func main() {

	// Tạo một HTTP queue mới
	httpQueue := queue.NewHttpQueue("api-service", "https://google.com/api/v1/data")

	// Thiết lập chế độ ưu tiên theo độ trễ
	httpQueue.SetPriorityMode(queue.PriorityLatency)

	httpQueue.SetRequestTimeout(5 * time.Second)

	// Thiết lập giới hạn tối đa 10 request đồng thời
	httpQueue.SetMaxConcurrent(100)

	pool := make([]struct{}, 10000)
	wg := sync.WaitGroup{}
	inc := 0

	// Tạo goroutine cho từng request
	for range pool {
		inc++
		t := time.Now()
		Go(&wg, func() {
			// Thực hiện GET request
			var response Response
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			free, total, status := httpQueue.GetConcurrentStatus()
			fmt.Printf("(%d - %f) Concurrent Status: free=%d, total=%d, status=%s\n", inc, time.Since(t).Seconds(), free, total, status)
			if status == "BUSY" /* && MULTI-TRANSFER */ {
				// Nếu có nhiều request đang chờ, ưu tiên xử lý SINGLE-TRANSFER,
				// đối với MULTI-TRANSFER có thể thử lại sau

				// TODO:
				// Maybe try again later
				time.Sleep(100 * time.Millisecond)
				return
			}
			if status == "OCCUPIED" {
				// Nếu tất cả các slot đều đã được chiếm dụng, chờ một chút trước khi thử lại

				// TODO:
				// Maybe try again later
				time.Sleep(100 * time.Millisecond)
				return
			}

			if err := httpQueue.Get(ctx, &response,
				queue.WithBasicAuth("username", "password"),
				queue.AcceptStatus(http.StatusOK, http.StatusCreated),
			); err != nil {
				fmt.Printf("(%d - %f) Error: %+v\n", inc, time.Since(t).Seconds(), err)
				return
			}
			fmt.Printf("(%d - %f) Response: %+v\n", inc, time.Since(t).Seconds(), response.Status)
		})
	}
	wg.Wait()
}

func Go(wg *sync.WaitGroup, f func()) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		f()
	}()
}

type Response struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}
