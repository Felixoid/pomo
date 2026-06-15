package pomo

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"time"
)

// Server listens on a Unix domain socket
// for Pomo status requests
type Server struct {
	listener          net.Listener
	runner            *TaskRunner
	running           bool
	publish           bool
	publishJson       bool
	publishSocketPath string
}

func (s *Server) listen() {
	for s.running {
		conn, err := s.listener.Accept()
		if err != nil {
			break
		}
		buf := make([]byte, 512)
		// Ignore any content
		if _, err := conn.Read(buf); err != nil {
			log.Printf("server: read: %v", err)
			_ = conn.Close()
			continue
		}
		raw, _ := json.Marshal(s.runner.Status())
		if _, err := conn.Write(raw); err != nil {
			log.Printf("server: write: %v", err)
		}
		_ = conn.Close()
	}
}

func (s *Server) push() {
	ticker := time.NewTicker(1 * time.Second)
	for s.running {
		conn, err := net.Dial("unix", s.publishSocketPath)
		if err != nil {
			<-ticker.C
			continue
		}
		status := s.runner.Status()
		if s.publishJson {
			raw, _ := json.Marshal(status)
			if err := json.NewEncoder(conn).Encode(raw); err != nil {
				log.Printf("server: publish encode: %v", err)
			}
		} else {
			if _, err := conn.Write([]byte(FormatStatus(*status) + "\n")); err != nil {
				log.Printf("server: publish write: %v", err)
			}
		}
		_ = conn.Close()
		<-ticker.C
	}
}

func (s *Server) Start() {
	s.running = true
	if s.publish {
		go s.push()
	}

	go s.listen()
}

func (s *Server) Stop() {
	s.running = false
	if s.listener != nil {
		if err := s.listener.Close(); err != nil {
			log.Printf("server: listener close: %v", err)
		}
	}
}

func NewServer(runner *TaskRunner, config *Config) (*Server, error) {
	//check if socket file exists
	if _, err := os.Stat(config.SocketPath); err == nil {
		_, err := net.Dial("unix", config.SocketPath)
		//if error then sock file was saved after crash
		if err != nil {
			if rmErr := os.Remove(config.SocketPath); rmErr != nil {
				return nil, fmt.Errorf("remove stale socket %s: %w", config.SocketPath, rmErr)
			}
		} else {
			// another instance of pomo is running
			return nil, fmt.Errorf("socket %s is already in use", config.SocketPath)
		}
	}
	listener, err := net.Listen("unix", config.SocketPath)
	if err != nil {
		return nil, err
	}

	server := &Server{
		listener:          listener,
		runner:            runner,
		publish:           config.Publish,
		publishJson:       config.PublishJson,
		publishSocketPath: config.PublishSocketPath,
	}

	return server, nil
}

// Client makes requests to a listening
// pomo server to check the status of
// any currently running task session.
type Client struct {
	conn net.Conn
}

func (c Client) read(statusCh chan *Status) {
	buf := make([]byte, 512)
	n, err := c.conn.Read(buf)
	if err != nil {
		log.Printf("client: read: %v", err)
		statusCh <- &Status{}
		return
	}
	status := &Status{}
	if err := json.Unmarshal(buf[0:n], status); err != nil {
		log.Printf("client: unmarshal: %v", err)
	}
	statusCh <- status
}

func (c Client) Status() (*Status, error) {
	statusCh := make(chan *Status)
	if _, err := c.conn.Write([]byte("status")); err != nil {
		return nil, fmt.Errorf("client: write: %w", err)
	}
	go c.read(statusCh)
	return <-statusCh, nil
}

func (c Client) Close() error { return c.conn.Close() }

func NewClient(path string) (*Client, error) {
	conn, err := net.Dial("unix", path)
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn}, nil
}
