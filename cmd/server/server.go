package main

import (
	"crypto/tls"
	"errors"
	"flag"
	"io"
	"log"
	"net"
	"net/url"
	"runtime"
	"syscall"

	"github.com/gobwas/ws"
	"github.com/panjf2000/ants/v2"
	"github.com/widaT/crab"
	"github.com/widaT/poller"
)

var server *crab.Server

var WakerToken = poller.Token(0)
var ClinetToken = poller.Token(1)

func main() {
	defer ants.Release()
	var isTLS bool
	flag.BoolVar(&isTLS, "tls", false, "enable tls")
	flag.Parse()

	// Increase resources limitations
	var rLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		panic(err)
	}
	rLimit.Cur = rLimit.Max
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		panic(err)
	}
	var err error
	server, err = crab.NewServer(WakerToken)
	if err != nil {
		panic(err)
	}

	var ln net.Listener
	if isTLS {
		cert, err := tls.LoadX509KeyPair("tls/cert.pem", "tls/key.pem")
		if err != nil {
			log.Println(err)
			return
		}
		config := &tls.Config{Certificates: []tls.Certificate{cert}}
		ln, err = tls.Listen("tcp", ":8888", config)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		ln, err = net.Listen("tcp", ":8888")
		if err != nil {
			log.Fatal(err)
		}
	}

	// Start epoll
	go func() {
		//lock one os thread only for this goroutine
		runtime.LockOSThread()
		server.Polling(callback)
	}()
	u := ws.Upgrader{
		//token认证
		OnRequest: func(uri []byte) error {
			values, err := url.ParseRequestURI(string(uri))
			if err != nil {
				return err
			}
			if values.Query().Get("token") != "abcd" {
				return errors.New("auth failed")
			}
			return nil
		},
	}
	for {
		c, err := ln.Accept()
		if err != nil {
			log.Fatal(err)
		}

		ants.Submit(func() {
			_, err := u.Upgrade(c)
			if err != nil {
				return
			}
			if err := server.Register(c, ClinetToken); err != nil {
				log.Printf("Failed to add connection %v", err)
				c.Close()
			}
		})
	}
}

func callback(ev *poller.Event) error {
	fd := int(ev.Fd)
	c := server.GetConn(fd)
	if c == nil {
		return nil
	}

	switch {
	case ev.IsReadable():
		ants.Submit(func() {
			if err := runJob(c); err != nil {
				server.Deregister(fd)
				//log.Println(err)
				c.Close()
			}
		})
	case ev.IsWriteClosed() || ev.IsReadClosed():
		server.Deregister(fd)
		c.Close()
	}
	return nil
}

func runJob(conn net.Conn) error {
	header, err := ws.ReadHeader(conn)
	if err != nil {
		return err
	}

	payload := make([]byte, header.Length)
	_, err = io.ReadFull(conn, payload)
	if err != nil {
		return err
	}

	if header.Masked {
		ws.Cipher(payload, header.Mask, 0)
	}

	// Reset the Masked flag, server frames must not be masked as
	// RFC6455 says.
	header.Masked = false

	server.AppendJob(func() {
		if err := ws.WriteHeader(conn, header); err != nil {
			//return err
		}
		if _, err := conn.Write(payload); err != nil {
			//return err
		}
	})
	return nil
}
