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
    keepalive int
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
	conn.Init(i)
	// for free list
	conn.Next = prev
	prev = conn
    }
    s.free = prev
    s.q_req = make(chan []byte, 256)
    return s
}

func (s *SupplyLine)HandleLink(cmd *msg.LinkCommand) {
}

func (s *SupplyLine)HandleKeepalive(cmd *msg.KeepaliveCommand) {
    s.keepalive = 0
}

func (s *SupplyLine)HandleConnect(cmd *msg.ConnectCommand) {
}

func (s *SupplyLine)HandleConnectAck(cmd *msg.ConnectAckCommand) {
    c := &s.connections[cmd.ConnId]
    if !c.Used {
	log.Printf("Ack for unused connection %d\n", cmd.ConnId)
	return
    }
    c.Q <- cmd
}

func (s *SupplyLine)HandleDisconnect(cmd *msg.DisconnectCommand) {
    c := &s.connections[cmd.ConnId]
    if !c.Used {
	log.Printf("Data for unused connection %d\n", cmd.ConnId)
	return
    }
    c.Q <- cmd
}

func (s *SupplyLine)HandleData(cmd *msg.DataCommand) {
    c := &s.connections[cmd.ConnId]
    if !c.Used {
	log.Printf("Data for unused connection %d\n", cmd.ConnId)
	return
    }
    c.Q <- cmd
}

func (s *SupplyLine)HandleDataAck(cmd *msg.DataAckCommand) {
    c := &s.connections[cmd.ConnId]
    if !c.Used {
	log.Printf("DataAck for unused connection %d\n", cmd.ConnId)
	return
    }
    c.Q <- cmd
}

func (s *SupplyLine)main(conn net.Conn) {
    defer conn.Close()

    tag := log.NewTag("Unknown")
    if tcp, ok := conn.(*net.TCPConn); ok {
	tag = log.NewTag(fmt.Sprintf("%v", tcp.RemoteAddr()))
    }

    tag.Printf("connected to frontline\n")

    hostname, err := os.Hostname()
    if err != nil {
	tag.Printf("unable to get hostname: %v\n", err)
	hostname = "Unknown"
    }
    cmd := msg.PackedLinkCommand(fmt.Sprintf("%s-%d", hostname, os.Getpid()))
    if _, err := conn.Write(cmd); err != nil {
	tag.Printf("send command error: %v\n", err)
	return
    }

    // now link is established, start receiver
    ticker := time.NewTicker(time.Minute)
    defer ticker.Stop()

    running := true
    q_recv := make(chan msg.Command, 256)
    q_wait := make(chan bool, 256)
    // start receiver
    go func() {
	err := msg.Receiver(conn, q_recv, q_wait, &running)
	tag.Printf("Receiver: %v\n", err)
	close(q_wait)
    }()
    for running {
	select {
	case cmd, ok := <-q_recv:
	    if !ok {
		tag.Printf("q_recv closed\n")
		running = false
		break
	    }
	    tag.Printf("recv %s\n", cmd.Name())
	    msg.HandleCommand(s, cmd)
	    q_wait <- true
	case cmd := <-s.q_req:
	    n := 0
	    for n < len(cmd) {
		w, err := conn.Write(cmd[n:])
		if err != nil {
		    tag.Printf("write cmd: %v\n", err)
		    running = false
		    break
		}
		n += w
	    }
	case <-ticker.C:
	    // keep alive
	    conn.Write(msg.PackedKeepaliveCommand())
	    s.keepalive++
	    if s.keepalive >= 3 {
		tag.Printf("keep alive failed\n")
		running = false
		break
	    }
	}
    }

    tag.Printf("disconnected from frontline\n")

    for i := 0; i < 256; i++ {
	c := &s.connections[i]
	c.Cancel()
    }

    for i := 0; i < 256; i++ {
	c := &s.connections[i]
	for c.Used {
	    time.Sleep(time.Second)
	}
    }

    time.Sleep(time.Second * 3)

    tag.Printf("end main\n")
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
    c.FlushQ()

    cmd := msg.PackedConnectCommand(c.Id, hostport)
    s.q_req <- cmd

    go func() {
	c.Run(conn, s.q_req)
	conn.Close()
	c.Free(func(){
	    // back to free
	    c.Next = s.free
	    s.free = c
	    log.Printf("connection %d back to freelist\n", c.Id)
	})
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
