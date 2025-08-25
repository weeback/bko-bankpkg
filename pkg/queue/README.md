# Package queue - HTTP Interface

Package queue của bko-bankpkg cung cấp một interface HTTP để quản lý, xử lý và tối ưu các HTTP request đến nhiều máy chủ. Package này thiết kế theo cơ chế hàng đợi (queue) với khả năng sao lưu (replication) tự động.

## Interface `HTTP`

```go
type HTTP interface {
    String() string
    SetPriorityMode(mode Priority)
    SetMaxConcurrent(maxConcurrent int)
    GetConcurrentStatus() (free int, total int, status string) // Get the current status of concurrent processing
    
    Get(ctx context.Context, v any, opts ...Option) error
    Post(ctx context.Context, v any, body []byte, opts ...Option) error
}
```

Interface `HTTP` là giao diện chính của package, cung cấp các phương thức để thực hiện các HTTP request thông qua hàng đợi với nhiều tùy chọn cao cấp.

### Các phương thức

#### `String() string`

- Trả về thông tin mô tả về hàng đợi HTTP dưới dạng chuỗi JSON
- Hiển thị URL mẫu và lỗi (nếu có)
- Hữu ích cho việc gỡ lỗi và kiểm tra trạng thái của hàng đợi

#### `SetPriorityMode(mode Priority)`

- Thiết lập chế độ ưu tiên cho hàng đợi
- Hỗ trợ các chế độ:
  - `PriorityFrequency`: Ưu tiên theo tần suất sử dụng
  - `PriorityLatency`: Ưu tiên theo thời gian phản hồi

#### `SetMaxConcurrent(maxConcurrent int)`

- Thiết lập số lượng request tối đa có thể thực hiện đồng thời
- Tham số:
  - `maxConcurrent`: Số lượng request tối đa. Giá trị 0 nghĩa là không giới hạn

#### `GetConcurrentStatus() (free int, limit int, status string)`

- Trả về trạng thái của hàng đợi xử lý đồng thời tại thời điểm được gọi
- Các giá trị:
  - `free`: Số lượng sẵn sàng
  - `limit`: Giới hạn đồng thời đã thiết lập. Giá trị 0 nghĩa là không giới hạn
  - `status`: thể hiện các trạng thái của hàng đợi, cụ thể:
    - `FREE` - All slots are free;
    - `AVAILABLE` - Many slots are free;
    - `BUSY` - Some slots are free;
    - `OCCUPIED` - All slots are occupied;

#### `Get(ctx context.Context, v any, opts ...Option) error`

- Thực hiện HTTP GET request
- Tham số:
  - `ctx`: Context để kiểm soát thời gian chờ và hủy request
  - `v`: Con trỏ đến biến để lưu kết quả JSON từ response
  - `opts`: Các tùy chọn bổ sung (headers, content type, mã trạng thái chấp nhận được)
- Trả về lỗi nếu có

#### `Post(ctx context.Context, v any, body []byte, opts ...Option) error`

- Thực hiện HTTP POST request
- Tham số:
  - `ctx`: Context để kiểm soát thời gian chờ và hủy request
  - `v`: Con trỏ đến biến để lưu kết quả JSON từ response
  - `body`: Dữ liệu để gửi trong request
  - `opts`: Các tùy chọn bổ sung
- Trả về lỗi nếu có

## Khởi tạo HTTP Queue

Package cung cấp hai hàm khởi tạo:

```go
// Create a queue with a single server
func NewHttpQueue(name, fullURL string) HTTP

// Create a queue with multiple backup servers
func NewHttpQueueWithMultiHost(name, fullURL string, hosts ...*url.URL) HTTP
```

### Tham số

- `name`: Tên định danh cho queue
- `fullURL`: URL mẫu đầy đủ để thực hiện request
- `hosts`: Danh sách các URL máy chủ sao lưu (chỉ dùng với `NewHttpQueueWithMultiHost`)

## Triển khai chi tiết

Interface `HTTP` được triển khai bởi struct `httpQueue` với các thành phần chính:

```go
type httpQueue struct {
    templateURL *url.URL   // Template URL
    err         error      // Error storage
    queue       queueInter // Queue management object
}
```

### Cơ chế hoạt động

1. **Hệ thống hàng đợi**:
   - Sử dụng mô hình worker-job để xử lý các request
   - Mỗi request tạo ra một `job` và đưa vào hàng đợi
   - `worker` quản lý các job và xử lý chúng theo thứ tự

2. **Quản lý máy chủ sao lưu**:
   - Hỗ trợ máy chủ chính (primary) và các máy chủ phụ (secondary)
   - Theo dõi số lượng request và thời gian phản hồi
   - Chọn máy chủ phù hợp dựa trên chế độ ưu tiên đã thiết lập

3. **Xử lý request và response**:
   - Mỗi request được gửi qua phương thức `listen()`
   - Kết quả trả về qua channel trong struct `recv`
   - Hỗ trợ xử lý lỗi và timeout thông qua `context`

4. **Kiểm soát lưu lượng**:
   - Giới hạn số lượng request đồng thời thông qua semaphore
   - Hai cách thiết lập giới hạn:
     - Sử dụng phương thức `SetMaxConcurrent()`
     - Truyền tùy chọn `WithMaxConcurrent()` cho mỗi request

5. **Xử lý lỗi thông minh**:
   - Cơ chế "lazy loading" được triển khai qua `lazyGuys()` để xử lý lỗi khởi tạo
   - Tự động xác thực URL và các tham số khác trước khi xử lý

## Tùy chọn (Options)

Package cung cấp nhiều tùy chọn để cấu hình request thông qua struct `Option`:

```go
type Option struct {
    XHeader      map[string]string
    ContentType  string
    AcceptStatus []int
}
```

### Các hàm tiện ích

- `WithHeaders(headers map[string]string) Option`: Thiết lập nhiều header
- `WithHeader(key, value string) Option`: Thiết lập một header cụ thể
- `WithContentType(contentType string) Option`: Thiết lập content type
- `WithBasicAuth(username, password string) Option`: Thiết lập xác thực Basic
- `AcceptStatus(status ...int) Option`: Thiết lập danh sách mã trạng thái HTTP được chấp nhận

## Ví dụ sử dụng

```go
package main

import (
    "context"
    "fmt"
    "net/http"
    "time"
    
    "github.com/weeback/bko-bankpkg/pkg/queue"
)

type Response struct {
    Status  string `json:"status"`
    Message string `json:"message"`
}

func main() {
    // Create a new HTTP queue
    httpQueue := queue.NewHttpQueue("api-service", "https://api.example.com/data")
    
    // Set priority mode to latency
    httpQueue.SetPriorityMode(queue.PriorityLatency)
    
    // Set maximum concurrent requests to 10
    httpQueue.SetMaxConcurrent(10)
    
    // Perform GET request
    var response Response
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    err := httpQueue.Get(ctx, &response, 
        queue.WithBasicAuth("username", "password"),
        queue.AcceptStatus(http.StatusOK, http.StatusCreated),
    
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    
    fmt.Printf("Response: %+v\n", response)
}
```

## Tùy chỉnh điều hướng

Tính năng kiểm soát lưu lượng request và tùy chỉnh điều hướng cho phép ứng dụng chủ động xử lý ưu tiên các request quan trọng khi hệ thống bận. Việc này đặc biệt hữu ích cho các hệ thống tài chính nơi một số giao dịch (như giao dịch đơn lẻ) cần được ưu tiên xử lý trước các giao dịch khác.

```go
package main

import (
    "context"
    "fmt"
    "net/http"
    "time"
    
    "github.com/weeback/bko-bankpkg/pkg/queue"
)

func processRequest(httpQueue queue.HTTP, reqType string) {
   // Check current queue status
   free, total, status := httpQueue.GetConcurrentStatus()
   
   // Handle routing logic based on request type and queue status
   // Example: If the queue is busy and the request is not a single transfer,
   // then defer processing to prioritize single transfers
   if status == "BUSY" && reqType != "SINGLE-TRANSFER" {
      fmt.Printf("The queue is busy (%d/%d slots used). ", total-free, total)
      fmt.Println("Priority processing of SINGLE-TRANSFER transaction; other transactions retry later!")
      
      // Can put request in another queue or schedule a retry later
      return
   }

   // If it's a priority transaction or the queue isn't overloaded, process it
   var response Response
   ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
   defer cancel()
    
   // Execute the request
   err := httpQueue.Get(ctx, &response, 
      queue.WithBasicAuth("username", "password"),
      queue.AcceptStatus(http.StatusOK, http.StatusCreated),
   )
   
   if err != nil {
      fmt.Printf("Error: %v\n", err)
      return
   }
    
   fmt.Printf("Response: %+v\n", response)
}

func main() {
   // Initialize HTTP queue
   httpQueue := queue.NewHttpQueue("api-service", "https://api.example.com/data")
    
   // Set priority mode to frequency
   httpQueue.SetPriorityMode(queue.PriorityFrequency)
    
   // Set maximum concurrent requests to 10
   httpQueue.SetMaxConcurrent(10)

   // Process requests with different types
   processRequest(httpQueue, "SINGLE-TRANSFER") // Single transfer (high priority)
   processRequest(httpQueue, "MULTI-TRANSFER")  // Multi-person transfer (low priority)
}
```

## Ưu điểm và đặc điểm nổi bật

1. **Khả năng sao lưu tự động**: Tự động chuyển request giữa các máy chủ khi gặp lỗi
2. **Quản lý trạng thái**: Theo dõi hiệu suất của từng máy chủ
3. **Tùy biến cao**: Nhiều tùy chọn cho các request
4. **Xử lý lỗi mạnh mẽ**: Tích hợp với context Go để hủy request
5. **Gỡ lỗi chi tiết**: Nhiều thông báo debug để theo dõi hoạt động
6. **Kiểm soát lưu lượng**: Khả năng giới hạn số lượng request đồng thời

Package này được thiết kế để xử lý các HTTP request một cách mạnh mẽ, với khả năng sao lưu và quản lý ưu tiên, phù hợp cho các ứng dụng cần độ tin cậy cao khi giao tiếp với các API bên ngoài.
