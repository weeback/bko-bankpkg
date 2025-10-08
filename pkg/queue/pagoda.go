package queue

import (
	"net/http"
	"os"

	"github.com/weeback/bko-bankpkg/pkg/logger"
	"go.uber.org/zap"
)

type pagoda struct {
	worker
}

// func (*pagoda) listen(string, string, []byte, Option) <-chan *recv {
// 	return nil
// }

func (p *pagoda) init() queueInter {

	// set the worker status to started
	p.code = 1 // started

	// ignore fields not used
	p.priorityMode = Priority("IGNORE")
	p.primaryServer = []*counter{}
	p.secondaryServerList = []*counter{}

	// setup and start the worker
	go p.start()

	return p
}

func (p *pagoda) start() {

	defer func() {
		// set the worker status to stopped
		p.code = -1 // stopped
		// release the resources
		close(p.jobs)
	}()

	log := logger.NewEntry()

	// this is a dummy call to free the counter.
	// this prevents the ide from giving syntax warnings no-used.
	p.freeCounter("do nothing", 0)

	// listen for incoming jobs
	for {
		select {
		case s := <-p.signal:
			if s == os.Interrupt {
				log.Info("worker stopped - interrupt signal received")
				return
			}
		case task, ok := <-p.jobs:
			if !ok {
				log.Info("jobs channel closed")
				return
			}
			go func(bp *job, limit int) {
				// execute the job
				rv := p.call(bp)
				if rv.HttpStatusCode == 0 {
					rv.HttpStatusCode = http.StatusBadGateway
				}
				// send the response to the channel
				if closed := recvSafe(bp.recv, &rv); closed {
					log.Debug("job processing - receive channel closed",
						zap.String("method", bp.method),
						zap.String("url", bp.fullURL))
				}
			}(task, 0)
		}
	}

}
