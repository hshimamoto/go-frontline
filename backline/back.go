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

type Connection struct {
    Id int
    Used bool
    Next *Connection
    Conn net.Conn
    Q chan msg.Command
    LocalLive, RemoteLive bool
}

func (c *Connection)LocalReader(conn net.Conn, buf []byte, q_lread chan int, q_lwait chan bool) {
    for c.LocalLive {
	r, err := conn.Read(buf)
	if err != nil {
	    log.Printf("Connection %d: Read: %v\n", c.Id, err)
	    break
	}
	if r == 0 {
	    log.Printf("Connection %d: closed\n", c.Id)
	    break
	}
	// send
	q_lread <- r
	// wait handled
	<-q_lwait
    }
    q_lread <- 0
    <-q_lwait
    c.LocalLive = false
}

func (c *Connection)Run(conn net.Conn, q_req chan []byte) {
    log.Printf("start connection %d\n", c.Id)
    // get CONNECT request
    buf := make([]byte, 256)
    n, err := conn.Read(buf)
    if err != nil {
	log.Printf("Read: %v\n", err)
	return
    }
    for {
	if bytes.Index(buf, []byte{13, 10, 13, 10}) > 0 {
	    break
	}
	r, err := conn.Read(buf[n:n+1])
	if err != nil {
	    log.Printf("Read: %v\n", err)
	    return
	}
	if r == 0 {
	    log.Println("no Read\n")
	    return
	}
	n += r
	if n >= 256 {
	    log.Println("header too long")
	    return
	}
    }
    lines := strings.Split(string(buf[:n]), "\r\n")
    w := strings.Split(lines[0], " ")
    if len(w) < 3 {
	log.Println("bad request")
	return
    }
    if w[0] != "CONNECT" {
	log.Printf("uknown request method %s\n", w[0])
	return
    }
    log.Printf("CONNECT %s\n", w[1])
    cmd := msg.PackedConnectCommand(c.Id, w[1])
    q_req <- cmd

    // now connected
    c.LocalLive = true
    c.RemoteLive = true

    // established
    conn.Write([]byte("HTTP/1.0 200 Established\r\n\r\n"))

    // TODO: this is adhoc implement
    lbuf := make([]byte, 8192)
    q_lread := make(chan int)
    q_lwait := make(chan bool)
    // start reading
    go c.LocalReader(conn, lbuf, q_lread, q_lwait)
    // start main loop
    running := true
    for running {
	select {
	case cmd := <-c.Q:
	    // recv data command
	    switch cmd := cmd.(type) {
	    case *msg.DataCommand:
		log.Printf("Data: %d %v\n", cmd.Seq, cmd.Data)
		// send to local connection
		conn.Write(cmd.Data)
	    case *msg.DisconnectCommand:
		// disconnect from remote
		c.RemoteLive = false
		running = false
	    }
	case r := <-q_lread:
	    if r > 0 {
		log.Printf("Connection %d: local read %d bytes\n", c.Id, r)
		// send data
		q_req <- msg.PackedDataCommand(c.Id, 0, lbuf[:r])
	    } else {
		// local closed
		log.Println("local connection closed")
		// send Disconnect
		q_req <- msg.PackedDisconnectCommand(c.Id)
		c.RemoteLive = false
		running = false
	    }
	    q_lwait <- true
	case <-time.After(time.Minute):
	    // periodic
	}
    }
    log.Printf("end connection %d\n", c.Id)
}

type SupplyLine struct {
    front string
    connections []Connection
    free *Connection
    q_req chan []byte
}

func NewSupplyLine(front string) *SupplyLine {
    s := &SupplyLine{
	front: front,
    }
    s.connections = make([]Connection, 256)
    // initialize
    var prev *Connection = nil
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
    // get one
    c := s.free
    s.free = s.free.Next
    // mark it used
    c.Used = true
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
