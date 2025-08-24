package queue

import (
	"net/url"
	"testing"
)

func Test_parseWithTemplate(t *testing.T) {
	tests := []struct {
		name       string
		template   string
		host       string
		wantResult string
	}{
		{
			name:       "Test 1",
			template:   "http://template.com/check/iphone",
			host:       "https://example.com/test-liveness",
			wantResult: "https://example.com/test-liveness/check/iphone",
		},
		{
			name:       "Test 2",
			template:   "http://template.com/check/iphone",
			host:       "https://example.com/",
			wantResult: "https://example.com/check/iphone",
		},
		{
			name:       "Test 3",
			template:   "http://template.com/check/iphone",
			host:       "https://example.com/check",
			wantResult: "https://example.com/check/iphone",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			templateURL, _ := url.Parse(tt.template)
			hostURL, _ := url.Parse(tt.host)

			got := parseWithTemplate(templateURL, hostURL)
			if got != tt.wantResult {
				t.Errorf("parseWithTemplate() = %v, want %v", got, tt.wantResult)
			}
		})
	}
}
