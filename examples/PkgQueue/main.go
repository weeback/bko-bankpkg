package main

import (
	"context"
	"log"
	"time"

	"github.com/weeback/bko-bankpkg/pkg/queue"
	"github.com/weeback/bko-bankpkg/pkg/transfer"
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

	transferQueue := transfer.NewTransfer(httpQueue)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Tạo một gói dữ liệu để gửi
	pack := transfer.Pack{
		Payload: []byte("test payload"),
	}
	// Gửi một gói dữ liệu sử dụng SingleTransfer (chặn đến khi hoàn thành)
	if err := transferQueue.SingleTransfer(ctx, &Response{}, &pack); err != nil {
		log.Fatalf("SingleTransfer error: %v", err)
	}

	// Tạo một danh sách các gói dữ liệu để gửi
	packs := []transfer.Pack{
		{Payload: []byte("test payload")},
	}
	// Sử dụng callback để nhận kết quả hoặc lỗi
	callbackFunc := func(id string, payloadResponse []byte, execError error) error {
		if execError != nil {
			log.Printf("MultiTransferWaitCallback error for pack %s: %v", id, execError)
			return execError
		}
		log.Printf("MultiTransferWaitCallback success for pack %s: %s", id, payloadResponse)
		return nil
	}
	// Gửi một gói dữ liệu sử dụng MultiTransfer (không chặn)
	if err := transferQueue.MultiTransferWaitCallback(ctx, callbackFunc, packs); err != nil {
		log.Fatalf("MultiTransfer error: %v", err)
	}
}

type Response struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}
