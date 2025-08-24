package queue

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
