package queue

func lazyGuys() queueInter {
	return &lazy{}

}

// lazy is a type that implements the queueInter interface (mirrors of workers).
//
// all method of laze always return happy result.
// And them not process any thing.
type lazy struct{}

func (*lazy) setPriorityMode(mode Priority) {}

func (*lazy) listen(string, string, []byte, Option) <-chan *recv {
	return nil
}
