package queue

import (
	"fmt"
	"strings"
)

const (
	dialTcpMask     = "dial tcp"
	schemeHttpMask  = "http://"
	schemeHttpsMask = "https://"
)

type responseError struct {
	value error
}

func (re *responseError) withDialTcpMask() *responseError {
	str := re.value.Error()
	if !strings.Contains(str, dialTcpMask) {
		return re
	}
	if idx := strings.LastIndex(str, ": "); idx >= 0 {
		str = str[idx+2:]
	}
	// remove the dial tcp mask
	return &responseError{fmt.Errorf("%s", str)}
}

func (re *responseError) withReplaceIP() *responseError {
	str := re.value.Error()
	if !strings.Contains(str, schemeHttpMask) && !strings.Contains(str, schemeHttpsMask) {
		return re
	}
	for _, mask := range []string{schemeHttpMask, schemeHttpsMask} {
		arr := strings.Split(str, mask)
		for idx, txt := range arr {
			if idx == 0 {
				continue
			}
			// replace the ip with *
			for i, r := range txt {
				if r == ':' || r == '/' {
					break
				}
				if i == len(txt)-1 {
					arr[idx] = fmt.Sprintf("%s*", txt[:i])
					txt = arr[idx]
				} else {
					arr[idx] = fmt.Sprintf("%s*%s", txt[:i], txt[i+1:])
					txt = arr[idx]
				}
			}
		}
		str = strings.Join(arr, mask)
	}
	return &responseError{fmt.Errorf("%s", str)}
}

func filterError(err error) error {
	if err == nil {
		return nil
	}
	re := &responseError{err}
	return re.withDialTcpMask().withReplaceIP().value
}
