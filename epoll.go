package crab

import (
	"log"
	"net"
	"sync"

	"github.com/widaT/poller"
	"github.com/widaT/poller/interest"
	"github.com/widaT/poller/pollopt"
)

type Server struct {
	//
	poller *poller.Poller

	connections sync.Map
	lock        *Locker
}

func NewServer(wakerToken poller.Token) (server *Server, err error) {
	server = new(Server)
	server.poller, err = poller.NewPoller()
	if err != nil {
		return
	}
	server.lock = new(Locker)
	return
}

func (s *Server) AppendJob(f func()) {
	s.lock.Lock()
	s.poller.AddTask(f)
	s.lock.Unlock()
	if err := s.poller.Wake(); err != nil {
		log.Printf("wakeup job err ：%s \n", err)
	}
}

func (s *Server) GetConn(fd int) net.Conn {
	if c, ok := s.connections.Load(fd); ok {
		return c.(net.Conn)
	}
	return nil
}

func (s *Server) Register(conn net.Conn, token poller.Token) error {
	fd, err := poller.NetConn2Fd(conn, false)
	if err != nil {
		return err
	}
	if s.poller.Register(fd, token, interest.READABLE, pollopt.Edge) != nil {
		return err
	}
	s.connections.Store(fd, conn)
	return nil
}

func (s *Server) Deregister(fd int) error {
	err := s.poller.Deregister(fd)
	if err != nil {
		return err
	}
	s.connections.Delete(fd)
	return nil
}

func (s *Server) Polling(callback func(*poller.Event) error) {
	s.poller.Polling(callback)
}
