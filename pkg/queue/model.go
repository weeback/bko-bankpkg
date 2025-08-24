package queue

import (
	"net/http"
	"net/textproto"
)

type job struct {
	serverLabel string
	method      string
	fullURL     string
	headers     map[string]string
	contentType string
	body        []byte

	// response channel
	recv chan *recv
}

func (ins *job) acceptContentType(r *http.Request) {
	// GET method does not need content type
	if r.Method == http.MethodGet {
		r.Header.Del("Content-Type")
		return
	}
	// set the content type for the request
	if ins.contentType != "" {
		r.Header.Set("Content-Type", ins.contentType)
	} else {
		r.Header.Set("Content-Type", "plain/text")
	}
}

// Note: this function will skip the Content-Type header,
// please set it manually or use the acceptContentType function
func (ins *job) acceptHeaders(r *http.Request) {
	//
	for k, v := range ins.headers {
		if textproto.CanonicalMIMEHeaderKey(k) == "Content-Type" {
			continue
		}
		r.Header.Set(k, v)
	}
	// GET method does not need content type
	if r.Method == http.MethodGet {
		r.Header.Del("Content-Type")
		return
	}
}

func recvSafe(c chan<- *recv, r *recv) (closed bool) {

	defer func() {
		if r := recover(); r != nil {
			closed = true
			return
		}

		close(c) // close the channel after sending the response
	}()

	c <- r // send the response to the channel
	return false
}

type recv struct {
	HttpStatusCode int
	HttpStatus     string
	Payload        []byte // Body           io.ReadCloser
	err            error
}
