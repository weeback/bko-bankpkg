package queue

import (
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

func Test_worker_status(t *testing.T) {
	tests := []struct {
		name string
		host *url.URL
		w    func(desc *url.URL) *worker
		n    int
		want string
	}{
		{
			name: "test case 1",
			host: func() *url.URL {
				desc, err := url.Parse("http://localhost:3001/get")
				if err != nil {
					t.Fatalf("failed to parse url")
				}
				return desc
			}(),
			w: func(desc *url.URL) *worker {
				w, ok := bindHttpQueue("Test_worker_status test_case_1", desc).(*worker)
				if !ok {
					t.Fatalf("failed to bind http queue")
				}
				w.code = 0
				return w
			},
			n:    30,
			want: "INACTIVE",
		},
		{
			name: "test case 2",
			host: func() *url.URL {
				desc, err := url.Parse("http://localhost:3002/get")
				if err != nil {
					t.Fatalf("failed to parse url")
				}
				return desc
			}(),
			w: func(desc *url.URL) *worker {
				w, ok := bindHttpQueue("Test_worker_status test_case_2", desc).(*worker)
				if !ok {
					t.Fatalf("failed to bind http queue")
				}
				w.code = -1
				return w
			},
			n:    30,
			want: "STOPPED",
		},
		{
			name: "test case 3",
			host: func() *url.URL {
				desc, err := url.Parse("http://localhost:3003/get")
				if err != nil {
					t.Fatalf("failed to parse url")
				}
				return desc
			}(),
			w: func(desc *url.URL) *worker {
				w, ok := bindHttpQueue("Test_worker_status test_case_3", desc).(*worker)
				if !ok {
					t.Fatalf("failed to bind http queue")
				}
				w.code = 1
				return w
			},
			n:    0,
			want: "STARTED",
		},
		{
			name: "test case 4",
			host: func() *url.URL {
				desc, err := url.Parse("http://localhost:3004/get")
				if err != nil {
					t.Fatalf("failed to parse url")
				}
				return desc
			}(),
			w: func(desc *url.URL) *worker {
				w, ok := bindHttpQueue("Test_worker_status test_case_4", desc).(*worker)
				if !ok {
					t.Fatalf("failed to bind http queue")
				}
				return w
			},
			n:    1,
			want: "IDLE",
		},
		{
			name: "test case 5",
			host: func() *url.URL {
				desc, err := url.Parse("http://localhost:3005/get")
				if err != nil {
					t.Fatalf("failed to parse url")
				}
				return desc
			}(),
			w: func(desc *url.URL) *worker {
				w, ok := bindHttpQueue("Test_worker_status test_case_5", desc).(*worker)
				if !ok {
					t.Fatalf("failed to bind http queue")
				}
				return w
			},
			n:    30,
			want: "WORKING",
		},
		{
			name: "test case 6",
			host: func() *url.URL {
				desc, err := url.Parse("http://localhost:3006/get")
				if err != nil {
					t.Fatalf("failed to parse url")
				}
				return desc
			}(),
			w: func(desc *url.URL) *worker {
				w, ok := bindHttpQueue("Test_worker_status test_case_6", desc).(*worker)
				if !ok {
					t.Fatalf("failed to bind http queue")
				}
				return w
			},
			n:    120,
			want: "BUSY",
		},
	}

	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			w := tt.w(tt.host)
			//
			wg := new(sync.WaitGroup)
			wg.Add(1)
			for count := 1; count <= 150; count++ {
				// break the loop if the number of jobs is greater than n+5
				if tt.n == 0 || count > tt.n+5 {
					wg.Done()
					break
				}
				pack := job{
					method:  "GET",
					fullURL: tt.host.String(),
					recv:    make(chan *recv),
				}
				select {
				case w.jobs <- &pack:
					// next
				case <-time.After(5 * time.Second):
					t.Fatalf("failed to send job to worker")
				}
			}

			wg.Wait()

			if got := w.status(); strings.HasPrefix(got, tt.want) {
				t.Logf("case %s - worker.status(): %v\n", tt.name, got)
			} else {
				t.Errorf("case %s - worker.status() = %v, want %v\n", tt.name, got, tt.want)
			}

			printQueueStatus(tt.host.Host)
		})
	}
}

func Test_worker_vote(t *testing.T) {
	tests := []struct {
		name    string
		hosts   []string
		w       func(hosts ...string) *worker
		wantErr bool
	}{
		{
			name: "test case 1",
			hosts: []string{
				"http://localhost:3001/get",
				"http://localhost:3002/get",
				"http://localhost:3003/get",
			},
			w: func(hosts ...string) *worker {
				desc := make([]*url.URL, 0)
				for _, host := range hosts {
					p, err := url.Parse(host)
					if err != nil {
						t.Fatalf("failed to parse url: %s", host)
					}
					desc = append(desc, p)
				}
				w, ok := bindHttpQueue("Test_worker_vote case_1", desc...).(*worker)
				if !ok {
					t.Fatalf("failed to bind http queue")
				}
				w.code = 0
				return w
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := tt.w(tt.hosts...)
			desc, label, err := w.vote("http://localhost:3000/template") // template is not same as the hosts
			if (err != nil) != tt.wantErr {
				t.Errorf("case %s - worker.vote() error = %v, wantErr %v\n", tt.name, err, tt.wantErr)
				return
			}
			t.Logf("case %s - worker.vote(): %v, label=%s\n", tt.name, desc, label)
		})
	}
}
