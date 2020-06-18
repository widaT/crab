package main

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net"
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

	for {
		c, err := ln.Accept()
		if err != nil {
			log.Fatal(err)
		}

		ants.Submit(func() {
			_, err = ws.Upgrade(c)
			if err != nil {
				return
			}

			//User Authentication
			frame, err := ws.ReadFrame(c)
			if err != nil {
				return
			}
			if frame.Header.Masked {
				ws.Cipher(frame.Payload, frame.Header.Mask, 0)
			}
			type Info struct {
				Token string
			}
			info := Info{}
			json.Unmarshal(frame.Payload, &info)
			if info.Token != "abcd" {
				frame := ws.NewCloseFrame([]byte(`{"ret":"Authentication faild"}`))
				ws.WriteFrame(c, frame)
				c.Close()
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
