package crab

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	_ "net/http/pprof"
	"sync"

	"github.com/gobwas/ws"
	"golang.org/x/sys/unix"
)

type Server struct {
	Sn2Conn  sync.Map
	Channels sync.Map

	Handler func(c *Conn, in []byte) error
	Epoller *epoll
}

var server *Server

func wsHandler(w http.ResponseWriter, r *http.Request) {
	// Upgrade connection
	c, _, _, err := ws.UpgradeHTTP(r, w)
	if err != nil {
		return
	}

	// 认证
	// if r.URL.Query().Get("token") != "abcd" {
	// 	w.Write([]byte("forbidden"))
	// 	return
	// }

	conn := &Conn{
		C: c,
		S: server,
	}
	if err := server.Epoller.Add(conn); err != nil {
		log.Printf("Failed to add connection %v", err)
		conn.Close()
	}
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
	http.HandleFunc("/broadcastinchannel", func(rw http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		var channel, message string
		if len(r.PostForm["channel"]) > 0 {
			channel = r.PostForm["channel"][0]
		}
		if len(r.PostForm["msg"]) > 0 {
			message = r.PostForm["msg"][0]
		}

		log.Printf("post data channel:%q msg:%q", channel, channel)
		broadcastinchannel(channel, []byte(message))
	})
	http.HandleFunc("/broadcast", func(rw http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		var message string

		if len(r.PostForm["msg"]) > 0 {
			message = r.PostForm["msg"][0]
		}

		log.Printf("post data msg:%q", message)
		broadcast([]byte(message))
	})

	http.ListenAndServe(":9333", nil)
}

func sendMessage(sn string, message []byte) error {
	c := server.GetConnBySn(sn)
	if c != nil {
		frame := ws.NewTextFrame(message)
		ws.WriteFrame(c.C, frame)
		return nil
	}
	return nil
	//return errors.New("sn2connection error")
}
func broadcastinchannel(channel string, message []byte) error {
	if c, ok := server.Channels.Load(channel); ok {
		frame := ws.NewTextFrame(message)
		c.(*sync.Map).Range(func(_, c any) bool {
			ws.WriteFrame(c.(*Conn).C, frame)
			return true
		})
	}
	return nil
}

func broadcast(message []byte) error {
	frame := ws.NewTextFrame(message)
	server.Sn2Conn.Range(func(_, c any) bool {
		ws.WriteFrame(c.(*Conn).C, frame)
		return true
	})
	return nil
}

func Run() {
	go runApiServer()

	epoller, err := MkEpoll()
	if err != nil {
		panic(err)
	}

	server = &Server{
		Sn2Conn:  sync.Map{},
		Channels: sync.Map{},
		Handler:  HandleMsg,
		Epoller:  epoller,
	}
	go server.Start()

	http.HandleFunc("/", wsHandler)
	if err := http.ListenAndServe("0.0.0.0:8888", nil); err != nil {
		log.Fatal(err)
	}
}

func (s *Server) Start() error {
	for {
		connections, err := s.Epoller.Wait()
		if err != nil && err != unix.EINTR {
			log.Printf("Failed to epoll wait %v", err)
			continue
		}
		for _, conn := range connections {
			if conn == nil {
				break
			}
			header, err := ws.ReadHeader(conn.C)
			if err != nil {
				conn.Close()
				continue
			}

			switch header.OpCode {
			case ws.OpPing:
				ws.WriteFrame(conn.C, ws.NewPongFrame([]byte("")))
				continue
			case ws.OpContinuation | ws.OpPong:
				continue
			case ws.OpClose:
				log.Printf("got close message")
				conn.Close()
				continue
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
			err = s.Handler(conn, payload)
			if err != nil {
				log.Printf("[err] handler error %s", err)
				conn.Close()
			}
		}
	}
}

func (s *Server) StoreConn(sn string, conn *Conn) {
	if c, ok := s.Sn2Conn.Load(sn); ok {
		c.(*Conn).Close()
	}
	s.Sn2Conn.Store(sn, conn)
}

func (s *Server) GetConnBySn(sn string) *Conn {
	if c, ok := s.Sn2Conn.Load(sn); ok {
		return c.(*Conn)
	}
	return nil
}

func (s *Server) Deregister(sn string) {
	s.Sn2Conn.Delete(sn)
}

func (s *Server) StoreChannel(channel, sn string, conn *Conn) {
	c, _ := s.Channels.LoadOrStore(channel, &sync.Map{})
	c.(*sync.Map).Store(sn, conn)
}

func (s *Server) RemoveFromChannels(channel, sn string) {
	if c, ok := s.Channels.Load(channel); ok {
		c.(*sync.Map).Delete(sn)
	}
}

func HandleMsg(c *Conn, in []byte) error {
	var message = Message{}
	err := json.Unmarshal(in, &message)
	if err != nil {
		return err
	}

	switch message.Type {
	case TJoin:
		if c.Sn == "" {
			c.Sn = message.Payload
			c.S.StoreConn(c.Sn, c)
		}
		if c.Channel == "" {
			c.Channel = message.Channel
			c.S.StoreChannel(c.Channel, c.Sn, c)
		}
	default:
		return errors.New("unreachable")
	}

	frame := ws.NewTextFrame([]byte("welcome: " + c.Sn))
	ws.WriteFrame(c.C, frame)
	return nil
}
