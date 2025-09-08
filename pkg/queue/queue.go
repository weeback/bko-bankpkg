package queue

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/weeback/bko-bankpkg/pkg/queue/metrics"
)

const (
	labelPrimary         = "PRIMARY"
	labelSecondaryPrefix = "SECONDARY"
)

var (
	publicQueue = make(map[string]*worker, 100)
)

type queueInter interface {
	setPriorityMode(mode Priority)
	setTimeout(d time.Duration)
	updateMaxConcurrent(maxConcurrent int)
	getConcurrentStatus() (int, int, string)
	listen(method string, fullURL string, body []byte, opt Option) <-chan *recv
	GetMetrics() *metrics.QueueMetrics // Optional - implemented by metricsWorker
}

func bindHttpQueue(name string, desc ...*url.URL) queueInter {
	var (
		primaryServer    []*counter // *url.URL
		secondaryServers []*counter
	)
	for i, value := range desc {
		if value.Host == "" {
			continue
		}
		if len(primaryServer) == 0 {
			// add the primary server
			primaryServer = []*counter{{label: labelPrimary, count: 0, url: value}}
		} else {
			// add the secondary servers
			secondaryServers = append(secondaryServers, &counter{label: fmt.Sprintf("%s_%02d", labelSecondaryPrefix, i), count: 0, url: value})
		}
	}
	if primaryServer == nil {
		return lazyGuys()
	}

	// set the queue name to the primary server host
	queueName := fmt.Sprintf("%s - %s", name, primaryServer[0].url.Host)
	//
	existingWorker, ok := publicQueue[queueName]
	if ok {
		// wrap with metrics and start the worker
		return NewWorkerWithMetrics(existingWorker).init()
	}
	// create a new queue
	w := &worker{
		name:                queueName,
		code:                0, // inactive
		primaryServer:       primaryServer,
		secondaryServerList: secondaryServers,
		jobs:                make(chan *job, 1000),
		signal:              make(chan os.Signal, 1),
		httpClient: func() *http.Client {
			return &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				},
				Timeout: 30 * time.Second,
			}
		},
		timeout:       30 * time.Second,
		maxConcurrent: 1000, // Default 1000 req/s
		activeSem:     nil,  // Will be initialized when needed
	}

	// Wrap with metrics
	mw := NewWorkerWithMetrics(w)
	publicQueue[queueName] = w

	return mw.init()
}

func printQueueStatus(star string) {
	header := "+============= QUEUE-NAME =======================================+============================+"
	footer := "+================================================================+============================+"
	txts := make([]string, 0, len(publicQueue))
	for k, v := range publicQueue {

		// templates:
		// ➚ ➛ ➜ ➞ ➟ ➠ ➡ ➢ ➣ ➤ ➥ ➦ ➶ ➵ ➳ ➴ ➲ ➱ ➯ ➾ ➽ ➭ ➬ ➼ ➻ ➫ ➪ ➺ ➹ ➩ ➨ ➸ ➷ ➧ ⃕ ⪡ ⨣ ▶ ▷ ◀ ◁ ▬
		//    ↨ ↧ ↦ ↥ ↤ ↣ ↢ ↡ ↠ ↟ ↞ ↝ ↜ ↛ ↚ ← ↑ → ↓ ↔ ↕ ↖ ↗ ↘ ↙ ↤ ↥ ↦
		//    《 》 « » ⇨ ⇒ ⇔ ⇚ ⇶ ⇵ ⇴ ⇳ ⇰ ⇯ ⇮ ⇭ ⇬ ⇫ ⇩ ⇨ ⇧ ⇦ ↻ ↺ ↨ ↧ ↦ ↥ ↤ ↣ ↢ ↡ ↠ ↟
		//    ↞ ↝ ↜ ↛ ↚ ↙ ↘ ↗ ↖ ← ↑ → ↓ ↔ ↕ ↖ ↗ ↘ ↙ ↤ ↥ ↦ ↧ ↨ ↸ ↹ ↮ ⇤ ⇥ ⇲ ⇞ ⇟ ↩ ↪ ↫ ↬
		//    ⇝ ↰ ↱ ↲ ↳ ↴ ↵ ↯ ↷ ↺ ↻ ⇜ ↶ ↼ ↽ ↾ ↿ ⇀ ⇁ ⇂ ⇃ ⇄ ⇅ ⇆ ⇇ ⇈ ⇉ ⇊ ⇍ ⇎ ⇏ ⇐ ⇑ ⇒ ⇓ ⇔ ⇕ ⇖ ⇗ ⇘ ⇙ ⇦ ⇧ ⇪ ⇫ ➔ ➙ ➘ ➚ ➛
		//
		//    ★✲ ⋆ ❄ ❅ ❇ ❈ ❖ ✫ ✪ ✩ ✬ ✮ ✭ ✯ ✰ ✹ ✸ ✷ ✶ ✵ ✳ ✱ ❊≛ ❉ ✾ ✽ ✼ ✠ ☆ ★ ✡
		//    ✡✺✼✴ ✺ ☼ ☸ ❋ ✽ ✻ ❆ ۞ ۝ ☀ ❃ ❂ ✿ ❀ ❁

		var (
			s string
		)
		if k == star {
			s = "»"
		}
		if len(k) > 60 {
			k = "[...] " + k[len(k)-50:]
		}
		txts = append(txts, header)
		txts = append(txts, fmt.Sprintf("| %1s %-60s | %-2s \t %20s |", s, k, v.priority(), v.status()))
		for _, srv := range v.primaryServer {
			txts = append(txts, fmt.Sprintf("|   %-60s |  \t %20s |",
				fmt.Sprintf("  + %s", srv.url.Host), fmt.Sprintf("%s : %02d", srv.took.String(), srv.count)))
		}
		for _, srv := range v.secondaryServerList {
			txts = append(txts, fmt.Sprintf("|   %-60s |  \t %20s |",
				fmt.Sprintf("  - %s", srv.url.Host), fmt.Sprintf("%s : %02d", srv.took.String(), srv.count)))
		}
	}
	println(strings.Join(txts, "\n") + "\n" + footer)
}
