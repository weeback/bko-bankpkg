package queue

import (
	"encoding/base64"
	"net/http"
	"net/textproto"
)

type Option struct {
	XHeader      map[string]string
	ContentType  string
	AcceptStatus []int
}

func WithMultiOptions(opts ...Option) Option {
	var (
		opt Option
	)
	for _, o := range opts {
		// merge the status
		opt.AcceptStatus = append(opt.AcceptStatus, o.AcceptStatus...)

		// merge the content type
		if opt.ContentType == "" && o.ContentType != "" {
			opt.ContentType = o.ContentType
		}

		// merge the headers
		for k, v := range o.XHeader {
			if _, ok := opt.XHeader[k]; !ok {
				if opt.XHeader != nil {
					opt.XHeader[k] = v
				} else {
					opt.XHeader = map[string]string{k: v}
				}
				continue
			}
		}
	}

	if owh := WithHeaders(opt.XHeader); opt.ContentType == "" {
		opt.ContentType = owh.ContentType
		opt.XHeader = owh.XHeader
	} else {
		opt.XHeader = owh.XHeader
	}

	return opt
}

// make the options by the given headers.
// if the headers contain the content type, it will be set as the content type and clean them.
// Otherwise, it will be set as plain/text.
func WithHeaders(headers map[string]string) Option {
	var (
		contentType = "plain/text"
	)
	for key, val := range headers {
		if WithHeader(key, val).ContentType != "" {
			if contentType == "plain/text" {
				contentType = val
			}
			delete(headers, key)
		}
	}
	return Option{
		XHeader:     headers,
		ContentType: contentType,
	}
}

func WithHeader(key, value string) Option {
	if textproto.CanonicalMIMEHeaderKey(key) == "Content-Type" {
		return WithContentType(value)
	}
	return Option{
		XHeader: map[string]string{key: value},
	}
}

func WithContentType(contentType string) Option {
	return Option{
		XHeader:     map[string]string{"Content-Type": contentType},
		ContentType: contentType,
	}
}

func WithDataContentType(data []byte) Option {
	contentType := http.DetectContentType(data)
	return Option{
		XHeader:     map[string]string{"Content-Type": contentType},
		ContentType: contentType,
	}
}

func WithBasicAuth(username, password string) Option {
	basicAuth := func(username, password string) string {
		return base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	}
	return Option{
		XHeader: map[string]string{
			"Authorization": "Basic " + basicAuth(username, password),
		},
	}
}

func AcceptStatus(status ...int) Option {
	list := make([]int, 0, len(status))
	for _, s := range status {
		if http.StatusText(s) != "" {
			list = append(list, s)
		}
	}
	//
	return Option{
		AcceptStatus: list,
	}
}
