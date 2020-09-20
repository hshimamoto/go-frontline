// HTTP frontline / frontline
// MIT License Copyright(c) 2020 Hiroshi Shimamoto
// vim:set sw=4 sts=4:
package main

import (
    "fmt"
    "net"
    "os"
    "time"

    "frontline/lib/connection"
    "frontline/lib/log"
    "frontline/lib/msg"

    "github.com/hshimamoto/go-session"
)

type SupplyLine struct {
    back net.Conn
    connections []msg.Connection
    q_req chan []byte
    keepalive int
}

func NewSupplyLine(conn net.Conn) (*SupplyLine, error) {
    s := &SupplyLine{
	back: conn,
    }
    s.connections = make([]msg.Connection, 256)
    for i := 0; i < 256; i++ {
	conn := &s.connections[i]
	conn.Init(i)
    }
    s.q_req = make(chan []byte, 256)
    return s, nil
}

func (s *SupplyLine)HandleLink(cmd *msg.LinkCommand) {
}

func (s *SupplyLine)HandleKeepalive(cmd *msg.KeepaliveCommand) {
    s.keepalive = 0
}

func (s *SupplyLine)HandleConnect(cmd *msg.ConnectCommand) {
    c := &s.connections[cmd.ConnId]
    if c.Used {
	// Ignore
	return
    }
    c.Used = true
    c.FlushQ()

    hostport := cmd.HostPort
    // try to connect
    lconn, err := session.Dial(hostport)
    if err != nil {
	log.Printf("Connection %d: Dial: %v\n", cmd.ConnId, err)
	s.q_req <- msg.PackedConnectAckCommand(cmd, false)
	c.Used = false
	return
    }
    s.q_req <- msg.PackedConnectAckCommand(cmd, true)

    go func () {
	c.Run(lconn, s.q_req)
	lconn.Close()
	c.Free(func(){
	    log.Printf("connection %d freed\n", c.Id)
	})
    }()
}

func (s *SupplyLine)HandleConnectAck(cmd *msg.ConnectAckCommand) {
}

func (s *SupplyLine)HandleDisconnect(cmd *msg.DisconnectCommand) {
    c := &s.connections[cmd.ConnId]
    if !c.Used {
	// something wrong
	return
    }
    c.Q <- cmd
}

func (s *SupplyLine)HandleData(cmd *msg.DataCommand) {
    c := &s.connections[cmd.ConnId]
    if !c.Used {
	// something wrong
	return
    }
    c.Q <- cmd
}

func (s *SupplyLine)HandleDataAck(cmd *msg.DataAckCommand) {
    c := &s.connections[cmd.ConnId]
    if !c.Used {
	// something wrong
	return
    }
    c.Q <- cmd
}

func (s *SupplyLine)Run() {
    conn := s.back

    tag := log.NewTag("Unknown")
    if tcp, ok := conn.(*net.TCPConn); ok {
	tag = log.NewTag(fmt.Sprintf("%v", tcp.RemoteAddr()))
    }

    q_recv := make(chan msg.Command, 256)
    q_wait := make(chan bool, 256)
    // start receiver
    go func() {
	err := msg.Receiver(conn, q_recv, q_wait)
	tag.Printf("Receiver: %v\n", err)
	close(q_wait)
    }()
    ticker := time.NewTicker(time.Minute)
    defer ticker.Stop()
    running := true
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
}

func main() {
    log.Setup("frontline")

    listen := ":8443"
    if len(os.Args) > 1 {
	listen = os.Args[1]
    }

    log.Printf("start listen %s", listen)

    serv, err := session.NewServer(listen, func(conn net.Conn) {
	defer conn.Close()
	log.Println("connected")
	if err := connection.EnableKeepAlive(conn); err != nil {
	    log.Printf("enable keepalive: %v\n", err)
	}
	// new SupplyLine
	if s, err := NewSupplyLine(conn); err == nil {
	    s.Run()
	}
	log.Println("close connection")
    })
    if err != nil {
	log.Printf("NewServer: %v\n", err)
	return
    }
    serv.Run()
}
