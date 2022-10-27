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
	Sn2Conn map[string]*Conn
	lock    sync.RWMutex
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
		//Sn: msg.Payload,
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
	http.ListenAndServe(":9333", nil)
}

func sendMessage(sn string, message []byte) error {
	c := server.GetConnBySn(sn)
	if c != nil {
		frame := ws.NewTextFrame(message)
		ws.WriteFrame(c.C, frame)
		return nil
	}
	return errors.New("sn2connection error")
}

func Run() {
	go runApiServer()

	epoller, err := MkEpoll()
	if err != nil {
		panic(err)
	}

	server = &Server{
		Sn2Conn: make(map[string]*Conn),
		Handler: HandleMsg,
		Epoller: epoller,
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
				conn.Close() //这边不需要收动close epool会自动处理
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
	s.lock.Lock()
	s.Sn2Conn[sn] = conn
	s.lock.Unlock()
}

func (s *Server) GetConnBySn(sn string) *Conn {
	s.lock.RLock()
	defer s.lock.RUnlock()
	if c, ok := s.Sn2Conn[sn]; ok {
		return c
	}
	return nil

}

func (s *Server) Deregister(sn string) {
	s.lock.Lock()
	delete(s.Sn2Conn, sn)
	s.lock.Unlock()
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
	default:
		return errors.New("unreachable")
	}

	frame := ws.NewTextFrame([]byte("welcome: " + c.Sn))
	ws.WriteFrame(c.C, frame)
	return nil
}
