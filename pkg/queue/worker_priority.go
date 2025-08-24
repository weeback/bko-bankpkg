package queue

import (
	"fmt"
	"net/url"
	"sort"
	"time"
)

const (
	PriorityFrequency      Priority = "FREQUENCY"
	PriorityShortFrequency Priority = "FR"
	PriorityLatency        Priority = "LATENCY"
	PriorityShortLatency   Priority = "LA"
)

type Priority string

func (w *worker) priority() Priority {
	switch w.priorityMode {
	case "":
		return PriorityShortFrequency
	case PriorityFrequency:
		return PriorityShortFrequency
	case PriorityLatency:
		return PriorityShortLatency
	default:
		return w.priorityMode
	}
}

func (w *worker) vote(template string) (string, string, error) {
	if template == "" {
		return "", "", fmt.Errorf("empty template URL")
	}
	templateUrl, err := url.Parse(template)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse url %s: %w", template, err)
	}
	if templateUrl == nil {
		return "", "", fmt.Errorf("nil template URL after parsing")
	}
	var (
		label string
		voted *url.URL
	)
	switch w.priority() {
	case PriorityLatency, PriorityShortLatency:
		label, voted = w.voteLatency()
	case PriorityFrequency, PriorityShortFrequency:
		label, voted = w.voteFrequency()
	default:
		label, voted = w.voteFrequency()
	}
	return parseWithTemplate(templateUrl, voted), label, nil
}

func (w *worker) voteFrequency() (label string, voted *url.URL) {
	// var (
	// 	label string
	// 	voted *url.URL
	// )
	// templateUrl, err := url.Parse(template)
	// if err != nil {
	// 	return "", label, fmt.Errorf("failed to parse url: %s", template)
	// }

	if len(w.secondaryServerList) > 0 {
		// sort the voter by counter ascending
		sort.Slice(w.secondaryServerList, func(i, j int) bool {
			return w.secondaryServerList[i].count < w.secondaryServerList[j].count
		})

		// reset the counter too large
		for _, val := range w.secondaryServerList {
			val.resetCountersIfNeeded()
		}
		// vote host secondary lowest counter to the template
		if w.secondaryServerList[0].count < w.primaryServer[0].count {
			label = w.secondaryServerList[0].label
			voted = w.secondaryServerList[0].url
			w.secondaryServerList[0].increment()
		}
	}

	// vote primary server if voted is not set by secondary server
	if voted == nil {
		//
		label = w.primaryServer[0].label
		voted = w.primaryServer[0].url
		w.primaryServer[0].increment()

		// the primary server only 1 host
		// reset the counter too large
		for _, val := range w.primaryServer {
			val.resetCountersIfNeeded()
		}
	}

	// Deprecated: moved to parseTemplate()
	//
	// templateUrl.Scheme = voted.Scheme
	// templateUrl.Host = voted.Host

	// // if the path is not empty, not root, and not the same as the template path
	// // then set the path to the voted path + template path
	// if voted.Path != "" && voted.Path != "/" && voted.Path != templateUrl.Path {
	// 	templateUrl.Path = voted.Path + templateUrl.Path
	// }

	// return parseWithTemplate(templateUrl, voted), label, nil
	return label, voted
}

func (w *worker) voteLatency() (label string, voted *url.URL) {
	if len(w.secondaryServerList) > 0 {
		// sort the voter by counter ascending
		sort.Slice(w.secondaryServerList, func(i, j int) bool {
			return w.secondaryServerList[i].took < w.secondaryServerList[j].took
		})
		// reset the counter too large
		for _, val := range w.secondaryServerList {
			val.resetCountersIfNeeded()
		}
		// vote host secondary lowest counter to the template
		if w.secondaryServerList[0].took < w.primaryServer[0].took {
			label = w.secondaryServerList[0].label
			voted = w.secondaryServerList[0].url
			w.secondaryServerList[0].increment()
		}
	}
	// vote primary server if voted is not set by secondary server
	if voted == nil {
		//
		label = w.primaryServer[0].label
		voted = w.primaryServer[0].url
		w.primaryServer[0].increment()

		// the primary server only 1 host
		// reset the counter too large
		// reset the counter too large
		for _, val := range w.primaryServer {
			val.resetCountersIfNeeded()
		}
	}
	return label, voted
}

// freeCounter : free the counter for the server.
// this function is same as setLastTook()
func (w *worker) freeCounter(label string, took time.Duration) {
	// Kiểm tra nil pointer
	if w == nil || w.primaryServer == nil {
		return
	}

	// Kiểm tra tham số đầu vào
	if label == "" {
		return
	}

	// Tạo slice mới để tránh race condition khi append
	servers := make([]*counter, 0, len(w.primaryServer)+len(w.secondaryServerList))
	servers = append(servers, w.primaryServer...)
	servers = append(servers, w.secondaryServerList...)

	for _, val := range servers {
		// Kiểm tra nil pointer
		if val == nil {
			continue
		}

		// Đặt thời gian reset phù hợp
		resetDuration := w.timeout
		if val.took < w.timeout {
			resetDuration = 5 * time.Second
		}

		// Reset took nếu cần
		val.resetTookIfNeeded(resetDuration)

		//
		if val.label == label {
			// Decrease the counter
			val.decreasing()
			// Cập nhật took và thời gian
			val.tookWithLabel(label, took)
		}
	}
}

// setLastTook: set the last took time for the server
func (w *worker) setLastTook(label string, took time.Duration) {
	// Kiểm tra nil pointer
	if w == nil || w.primaryServer == nil {
		return
	}

	// Kiểm tra tham số đầu vào
	if label == "" || took < 0 {
		return
	}

	// Tạo slice mới để tránh race condition khi append
	servers := make([]*counter, 0, len(w.primaryServer)+len(w.secondaryServerList))
	servers = append(servers, w.primaryServer...)
	servers = append(servers, w.secondaryServerList...)

	for _, val := range servers {
		// Kiểm tra nil pointer
		if val == nil {
			continue
		}

		// Đặt thời gian reset phù hợp
		resetDuration := w.timeout
		if val.took < w.timeout {
			resetDuration = 5 * time.Second
		}

		// Reset took nếu cần
		val.resetTookIfNeeded(resetDuration)

		// Cập nhật took và thời gian
		val.tookWithLabel(label, took)
	}
}
