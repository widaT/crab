package crab

import (
	"log"
	"net"
)

type Conn struct {
	Sn      string
	Channel string
	C       net.Conn
	S       *Server
}

func (c *Conn) Close() error {
	c.S.Epoller.Remove(c)
	if len(c.Sn) > 0 {
		c.S.Deregister(c.Sn)
		log.Printf("sn:%s closed", c.Sn)
	}

	if len(c.Channel) > 0 {
		c.S.RemoveFromChannels(c.Channel, c.Sn)
		log.Printf("remove sn:%s  from %s Channels", c.Sn, c.Channel)
	}

	if c.C != nil {
		c.C.Close()
	}
	return nil
}
