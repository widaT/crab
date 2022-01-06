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
	poller      *poller.Poller
	connections sync.Map
	sn2fd       sync.Map
	lock        *Locker
	Handler     func(c *Conn, in []byte) error
}

func NewServer(wakerToken poller.Token, handler func(c *Conn, in []byte) error) (server *Server, err error) {
	server = new(Server)
	server.poller, err = poller.NewPoller()
	if err != nil {
		return
	}
	server.lock = new(Locker)
	server.Handler = handler
	return
}

func (s *Server) AddTask(f func()) {
	s.lock.Lock()
	s.poller.AddTask(f)
	s.lock.Unlock()
	if err := s.poller.Wake(); err != nil {
		log.Printf("wakeup job err ：%s \n", err)
	}
}

func (s *Server) GetConnBySn(sn string) *Conn {
	if c, ok := s.sn2fd.Load(sn); ok {
		return c.(*Conn)
	}
	return nil

}

func (s *Server) StoreConn(sn string, conn *Conn) {
	if temp, ok := s.sn2fd.Load(sn); ok {
		//remove old connection
		c := temp.(*Conn)
		c.Close()
	}
	s.sn2fd.Store(sn, conn)
}

func (s *Server) GetConn(fd int) *Conn {
	if c, ok := s.connections.Load(fd); ok {
		return c.(*Conn)
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
	s.connections.Store(fd, &Conn{C: conn, S: s})
	return nil
}

func (s *Server) Deregister(fd int) error {
	s.connections.Delete(fd)
	err := s.poller.Deregister(fd)
	if err != nil {
		return err
	}
	return nil
}

func (s *Server) Delete(sn string) {
	s.sn2fd.Delete(sn)
}

func (s *Server) Polling(callback func(*poller.Event) error) {
	s.poller.Polling(callback)
}
