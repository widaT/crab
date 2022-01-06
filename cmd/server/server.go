package main

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
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

func HandleMsg(c *crab.Conn, in []byte) error {
	log.Printf("got messge %q", in)

	var message = crab.Message{}
	err := json.Unmarshal(in, &message)
	if err != nil {
		return err
	}

	switch message.Type {
	case crab.TJoin:
		if c.Sn == "" {
			c.Sn = message.Payload
			c.S.StoreConn(c.Sn, c)
		}
	default:
		return errors.New("unreachable")
	}

	frame := ws.NewTextFrame([]byte("welcome: " + c.Sn))
	//异步处理 建议异步处理
	server.AddTask(func() {
		ws.WriteFrame(c.C, frame)
	})
	//同步处理
	//ws.WriteFrame(c, frame)
	return nil
}

func sendMessage(sn string, message []byte) error {
	c := server.GetConnBySn(sn)
	fmt.Println(c)
	if c != nil {
		frame := ws.NewTextFrame(message)
		server.AddTask(func() {
			ws.WriteFrame(c.C, frame)
		})
		return nil
	}
	return errors.New("sn2connection error")
}

func runApiServer() {
	http.HandleFunc("/send_msg", func(rw http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		var sn, message string
		if len(r.PostForm["sn"]) > 0 {
			sn = r.PostForm["sn"][0]
		}
		if len(r.PostForm["msg"]) > 0 {
			message = r.PostForm["msg"][0]
		}

		log.Printf("post data sn:%q msg:%q", sn, message)
		sendMessage(sn, []byte(message))
	})
	http.ListenAndServe(":9333", nil)
}

func main() {

	defer ants.Release()
	var isTLS bool
	flag.BoolVar(&isTLS, "tls", false, "enable tls")
	flag.Parse()

	// ulimit放开限制
	var rLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		panic(err)
	}
	rLimit.Cur = rLimit.Max
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		panic(err)
	}
	var err error
	server, err = crab.NewServer(WakerToken, HandleMsg)
	if err != nil {
		panic(err)
	}

	var ln net.Listener

	//wss
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

	//http 接口
	go runApiServer()

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

		//提交任务到go协程库
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

//callback 事件触发函数
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
				//log.Println(err)
				c.Close()
			}
		})
	case ev.IsWriteClosed() || ev.IsReadClosed():
		c.Close()
	}
	return nil
}

func runJob(conn *crab.Conn) error {
	header, err := ws.ReadHeader(conn.C)
	if err != nil {
		return err
	}

	switch header.OpCode {
	case ws.OpContinuation | ws.OpPing | ws.OpPong:
		return nil
	case ws.OpClose:
		//conn.Close() 这边不需要收动close epool会自动处理
		return nil
	default:
	}

	payload := make([]byte, header.Length)
	_, err = io.ReadFull(conn.C, payload)
	if err != nil {
		return err
	}
	if header.Masked {
		ws.Cipher(payload, header.Mask, 0)
	}
	return conn.S.Handler(conn, payload)
}
