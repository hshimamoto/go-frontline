// HTTP frontline / backline
// MIT License Copyright(c) 2020 Hiroshi Shimamoto
// vim:set sw=4 sts=4:
package main

import (
    "bytes"
    "fmt"
    "net"
    "os"
    "strings"
    "time"

    "frontline/lib/connection"
    "frontline/lib/log"
    "frontline/lib/msg"

    "github.com/hshimamoto/go-session"
)

func waitHTTPConnect(conn net.Conn) (string, error) {
    buf := make([]byte, 256)
    n, err := conn.Read(buf)
    if err != nil {
	log.Printf("Read: %v\n", err)
	return "", err
    }
    for {
	if bytes.Index(buf, []byte{13, 10, 13, 10}) > 0 {
	    break
	}
	r, err := conn.Read(buf[n:n+1])
	if err != nil {
	    log.Printf("Read: %v\n", err)
	    return "", err
	}
	if r == 0 {
	    log.Println("no Read\n")
	    return "", fmt.Errorf("no Read")
	}
	n += r
	if n >= 256 {
	    log.Println("header too long")
	    return "", fmt.Errorf("request header too long")
	}
    }
    lines := strings.Split(string(buf[:n]), "\r\n")
    w := strings.Split(lines[0], " ")
    if len(w) < 3 {
	log.Println("bad request")
	return "", fmt.Errorf("bad request")
    }
    if w[0] != "CONNECT" {
	log.Printf("uknown request method %s\n", w[0])
	return "", fmt.Errorf("unknown request method %s", w[0])
    }
    return w[1], nil
}

type SupplyLine struct {
    front string
    connections []msg.Connection
    free *msg.Connection
    q_req chan []byte
}

func NewSupplyLine(front string) *SupplyLine {
    s := &SupplyLine{
	front: front,
    }
    s.connections = make([]msg.Connection, 256)
    // initialize
    var prev *msg.Connection = nil
    for i := 0; i < 256; i++ {
	conn := &s.connections[i]
	conn.Id = i
	conn.Used = false
	conn.Next = prev
	conn.Q = make(chan msg.Command)
	prev = conn
    }
    s.free = prev
    s.q_req = make(chan []byte)
    return s
}

func (s *SupplyLine)handleDisconnect(conn net.Conn, cmd *msg.DisconnectCommand) {
    c := &s.connections[cmd.ConnId]
    if !c.Used {
	log.Printf("Data for unused connection %d\n", cmd.ConnId)
	return
    }
    c.Q <- cmd
}

func (s *SupplyLine)handleData(conn net.Conn, cmd *msg.DataCommand) {
    c := &s.connections[cmd.ConnId]
    if !c.Used {
	log.Printf("Data for unused connection %d\n", cmd.ConnId)
	return
    }
    c.Q <- cmd
}

func (s *SupplyLine)handleCommand(conn net.Conn, cmd msg.Command) {
    log.Printf("handle cmd: %s\n", cmd.Name())
    switch cmd := cmd.(type) {
    case *msg.LinkCommand:
	log.Printf("link from %s\n", cmd.Client)
    case *msg.ConnectCommand:
	log.Printf("connect to %s [%d]\n", cmd.HostPort, cmd.ConnId)
    case *msg.DisconnectCommand:
	log.Printf("disconnect [%d]\n", cmd.ConnId)
	s.handleDisconnect(conn, cmd)
    case *msg.DataCommand:
	log.Printf("data [%d] %dbytes\n", cmd.ConnId, len(cmd.Data))
	s.handleData(conn, cmd)
    case *msg.UnknownCommand:
	log.Println("unknown command")
    }
    log.Println("handle command done")
}

func (s *SupplyLine)main(conn net.Conn) {
    hostname, err := os.Hostname()
    if err != nil {
	log.Printf("unable to get hostname: %v\n", err)
	hostname = "Unknown"
    }
    cmd := msg.PackedLinkCommand(fmt.Sprintf("%s-%d", hostname, os.Getpid()))
    if _, err := conn.Write(cmd); err != nil {
	log.Printf("send command error: %v\n", err)
	return
    }
    // now link is established, start receiver
    q_recv := make(chan msg.Command)
    q_wait := make(chan bool)
    go msg.Receiver(conn, q_recv, q_wait)
    running := true
    for running {
	select {
	case cmd := <-s.q_req:
	    conn.Write(cmd)
	case cmd, ok := <-q_recv:
	    if !ok {
		log.Println("q_recv closed")
		running = false
		break
	    }
	    log.Printf("recv %s\n", cmd.Name())
	    s.handleCommand(conn, cmd)
	    q_wait <- true
	case <-time.After(time.Minute):
	}
    }
}

func (s *SupplyLine)Run() {
    for {
	if conn, err := session.Dial(s.front); err == nil {
	    s.main(conn)
	} else {
	    log.Printf("SupplyLine %s: %v\n", s.front, err)
	}
	// interval
	time.Sleep(time.Second)
    }
}

func (s *SupplyLine)Connect(conn net.Conn) {
    log.Println("accept new stream")
    if s.free == nil {
	log.Println("no free connection slot")
	conn.Close()
	return
    }
    if err := connection.EnableKeepAlive(conn); err != nil {
	log.Printf("enable keepalive: %v\n", err)
    }
    hostport, err := waitHTTPConnect(conn)
    if err != nil {
	conn.Close()
	return
    }
    log.Printf("CONNECT %s\n", hostport)

    // get one
    c := s.free
    s.free = s.free.Next
    // mark it used
    c.Used = true

    cmd := msg.PackedConnectCommand(c.Id, hostport)
    s.q_req <- cmd
    // send back established response in HTTP CONNECT METHOD
    conn.Write([]byte("HTTP/1.0 200 Established\r\n\r\n"))

    go func() {
	defer conn.Close()
	c.Run(conn, s.q_req)
	// back to free
	c.Next = s.free
	s.free = c
    }()
}

func main() {
    log.Setup("backline")

    if len(os.Args) < 2 {
	log.Println("backline <frontline> [listen]")
	return
    }

    listen := ":8443"
    front := os.Args[1]
    if len(os.Args) > 2 {
	listen = os.Args[2]
    }

    log.Printf("start front %s listen %s", front, listen)

    s := NewSupplyLine(front)

    serv, err := session.NewServer(listen, s.Connect)
    if err != nil {
	log.Printf("NewServer: %v\n", err)
	return
    }

    // now we can start to communicate with frontline
    go s.Run()

    serv.Run()
}
