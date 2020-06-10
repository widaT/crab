package crab

import (
	"log"
	"net"
	"sync"

	"github.com/widaT/poller"
	"github.com/widaT/poller/interest"
	"github.com/widaT/poller/pollopt"
	"github.com/widaT/poller/waker"
	"golang.org/x/sys/unix"
)

type Server struct {
	//
	poll       *poller.Selector
	waker      *waker.Waker
	wakerToken poller.Token

	connections sync.Map
	asyncJobs   []func()
	lock        *Locker
}

func NewServer() (server *Server, err error) {
	server = new(Server)
	server.poll, err = poller.New()
	if err != nil {
		return
	}
	wakerToken := poller.Token(0)
	server.waker, err = waker.New(server.poll, wakerToken)
	if err != nil {
		return
	}

	server.lock = new(Locker)
	return
}

func (s *Server) AppendJob(f func()) {
	s.lock.Lock()
	s.asyncJobs = append(s.asyncJobs, f)
	s.lock.Unlock()
	if err := s.waker.Wake(); err != nil {
		log.Printf("wakeup job err ：%s \n", err)
	}
}

func (s *Server) GetConn(fd int) net.Conn {
	if c, ok := s.connections.Load(fd); ok {
		return c.(net.Conn)
	}
	return nil
}

func (s *Server) Register(conn net.Conn) error {
	fd, err := poller.NetConn2Fd(conn, false)
	if err != nil {
		return err
	}
	if s.poll.Register(fd, poller.Token(fd), interest.READABLE, pollopt.Edge) != nil {
		return err
	}
	s.connections.Store(fd, conn)
	return nil
}

func (s *Server) Deregister(fd int) error {
	err := s.poll.Deregister(fd)
	if err != nil {
		return err
	}
	s.connections.Delete(fd)
	return nil
}

func (s *Server) Polling(callback func(*poller.Event)) {
	events := poller.MakeEvents(128)
	doJob := false
	for {
		n, err := s.poll.Select(events, -1)
		if err != nil && err != unix.EINTR {
			log.Println(err)
			continue
		}

		for i := 0; i < n; i++ {
			ev := events[i]
			switch ev.Token() {
			case s.wakerToken:
				s.waker.Reset()
				doJob = true
			default:
				callback(&ev)
			}
		}
		if doJob {
			doJob = false
			s.doJob()
		}
	}
}

func (s *Server) doJob() {
	s.lock.Lock()
	jobs := s.asyncJobs
	s.asyncJobs = nil
	s.lock.Unlock()
	length := len(jobs)
	for i := 0; i < length; i++ {
		jobs[i]()
	}
}
