package queue

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

func toJson(v any) string {
	w := &bytes.Buffer{}
	json.NewEncoder(w).Encode(v)
	return w.String()
}

func parseWithTemplate(templateUrl, host *url.URL) string {
	// Replace the template URL with the host URL
	templateUrl.Scheme = host.Scheme
	templateUrl.Host = host.Host

	// if the path is not empty, not root, and not the same as the template path
	// then set the path to the voted path + template path
	if host.Path != "" && host.Path != "/" {
		// Example of the condition:
		// templateUrl: http://template.com/check/iphone, host: https://example.com/test-liveness ==> https://example.com/test-liveness/check/iphone
		// templateUrl: http://template.com/check/iphone, host: https://example.com/              ==> https://example.com/check/iphone
		// templateUrl: http://template.com/check/iphone, host: https://example.com/check         ==> https://example.com/check/iphone
		if !strings.HasPrefix(templateUrl.Path, host.Path) {
			if path, err := url.JoinPath("/", host.Path, templateUrl.Path); err != nil {
				templateUrl.Path = fmt.Sprintf("%s/%s", host.Path, templateUrl.Path)
			} else {
				templateUrl.Path = path
			}
		}
	}

	return templateUrl.String()
}
