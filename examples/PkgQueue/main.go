package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/weeback/bko-bankpkg/pkg/queue"
)

// First time, read file `github.com/weeback/bko-bankpkg/pkg/queue/README.md` to understand the package
// Then, use the package in your code.
// Finally, run this example with `go run examples/PkgQueue/main.go`
func main() {

	// Tạo một HTTP queue mới
	httpQueue := queue.NewHttpQueue("api-service", "https://api.example.com/data")

	// Thiết lập chế độ ưu tiên theo độ trễ
	httpQueue.SetPriorityMode(queue.PriorityLatency)

	// Thực hiện GET request
	var response Response
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := httpQueue.Get(ctx, &response,
		queue.WithBasicAuth("username", "password"),
		queue.AcceptStatus(http.StatusOK, http.StatusCreated))

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Response: %+v\n", response)

}

type Response struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}
