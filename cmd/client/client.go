package main

import (
	"flag"
	"log"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
	"github.com/widaT/crab"
)

var addr = flag.String("addr", "localhost:8888", "http service address")
var isTLS = flag.Bool("tls", false, "use websocket over tls")

func main() {
	flag.Parse()
	log.SetFlags(0)

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	Scheme := "ws"
	if *isTLS {
		Scheme = "wss"
	}
	u := url.URL{Scheme: Scheme, Host: *addr, Path: "/", RawQuery: "token=abcd"}
	log.Printf("connecting to %s", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			}
			log.Printf("recv: %s", message)
		}
	}()

	//第一条消息注册下设备sn
	message := crab.Message{
		Type:    crab.TJoin,
		Payload: "no123456",
	}
	c.WriteMessage(websocket.TextMessage, message.ToJSON())
	for {
		select {
		case <-done:
			return
		case <-interrupt:
			log.Println("interrupt")
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("write close:", err)
				return
			}
		case <-time.After(time.Second):
			log.Println("ddddd")
			c.WriteMessage(websocket.TextMessage, []byte("ddd"))
		}
	}
}
