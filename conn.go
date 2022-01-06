package crab

import (
	"log"
	"net"

	"github.com/widaT/poller"
)

type Conn struct {
	Sn string
	C  net.Conn
	S  *Server
}

func (c *Conn) Close() error {
	fd, err := poller.NetConn2Fd(c.C, false)
	if err != nil {
		return err
	}
	c.S.Deregister(fd)
	if len(c.Sn) > 0 {
		c.S.Delete(c.Sn)
		log.Printf("sn:%s closed", c.Sn)
	}
	c.C.Close()
	return nil
}
