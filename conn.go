package crab

import (
	"log"
	"net"
)

type Conn struct {
	Sn string
	C  net.Conn
	S  *Server
}

func (c *Conn) Close() error {
	// log.Printf("%#v is closed", *c)
	c.S.Epoller.Remove(c)
	if len(c.Sn) > 0 {
		c.S.Deregister(c.Sn)
		log.Printf("sn:%s closed", c.Sn)
	}
	if c.C != nil {
		c.C.Close()
	}
	return nil
}
