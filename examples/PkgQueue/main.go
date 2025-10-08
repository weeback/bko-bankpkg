package main

import (
	"context"
	"log"
	"os"
	"sync"
	"time"

	"github.com/weeback/bko-bankpkg/pkg/queue"
	"github.com/weeback/bko-bankpkg/pkg/transfer"
)

// First time, read file `github.com/weeback/bko-bankpkg/pkg/queue/README.md` to understand the package
// Then, use the package in your code.
// Finally, run this example with `go run examples/PkgQueue/main.go`
func main() {

	os.Setenv("DEPLOYMENT_ENVIRONMENT", "development")

	// Tạo một HTTP queue mới
	multiDestinationQueue := queue.NewMultiDestinationQueue("api-service")

	// Thiết lập chế độ ưu tiên theo độ trễ
	multiDestinationQueue.SetPriorityMode(queue.PriorityLatency)

	multiDestinationQueue.SetRequestTimeout(5 * time.Second)

	// Thiết lập giới hạn tối đa 20 request đồng thời
	multiDestinationQueue.SetMaxConcurrent(20)

	transferQueue := transfer.NewTransfer(multiDestinationQueue)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Tạo một gói dữ liệu để gửi
	pack := transfer.Pack{
		Payload: []byte("test payload"),
	}
	// Gửi một gói dữ liệu sử dụng SingleTransfer (đợi đến khi hoàn thành)
	if err := transferQueue.SingleTransfer(ctx, &Response{}, "https://google.com/api/v1/data", &pack); err != nil {
		log.Printf("[ERR] SingleTransfer error: %v", err)
	}

	// Tạo một danh sách các gói dữ liệu để gửi
	packs := []*transfer.Pack{
		{ID: "1", Payload: []byte("test payload"), CreatedAt: time.Now()},
	}
	// Bộ control kiểm tra kết quả trả về
	wg := sync.WaitGroup{}
	// Đánh dấu số lượng gói dữ liệu cần chờ
	wg.Add(len(packs))

	// Sử dụng callback để nhận kết quả hoặc lỗi
	callbackFunc := func(id string, payloadResponse []byte, execError error) {
		// Đánh dấu hoàn thành một gói dữ liệu
		defer wg.Done()
		// TODO: Xử lý kết quả trả về (payloadResponse)
		processResponse(id, payloadResponse, execError)
	}
	// Gửi một gói dữ liệu sử dụng MultiTransfer (không đợi, sử dụng callback để nhận kết quả)
	if err := transferQueue.MultiTransferWaitCallback(ctx, callbackFunc, "https://googleaaa.com/khong-ton-tai", packs); err != nil {
		log.Printf("[ERR] MultiTransfer error: %v", err)
	}

	// Đảm bảo tất cả các tác vụ đã hoàn thành trước khi thoát
	wg.Wait()
	log.Println("All tasks completed.")
}

func processResponse(id string, payloadResponse []byte, execError error) {
	// Kiểm tra lỗi
	if execError != nil {
		// Xử lý lỗi
		log.Printf("MultiTransferWaitCallback error for pack %s: %v", id, execError)
		return
	}

	log.Printf("Processing response for pack %s: %s\n", id, payloadResponse)

	// Giả sử update thông tin 1 giây
	time.Sleep(1 * time.Second)
	log.Printf("Finished processing response for pack %s\n", id)
}

type Response struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}
