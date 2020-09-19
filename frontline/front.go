// HTTP frontline / frontline
// MIT License Copyright(c) 2020 Hiroshi Shimamoto
// vim:set sw=4 sts=4:
package main

import (
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
}

func NewSupplyLine(conn net.Conn) (*SupplyLine, error) {
    s := &SupplyLine{
	back: conn,
    }
    s.connections = make([]msg.Connection, 256)
    for i := 0; i < 256; i++ {
	conn := &s.connections[i]
	conn.Id = i
	conn.Used = false
	conn.Q = make(chan msg.Command)
    }
    s.q_req = make(chan []byte)
    return s, nil
}

func (s *SupplyLine)handleConnect(conn net.Conn, cmd *msg.ConnectCommand) {
    c := &s.connections[cmd.ConnId]
    if c.Used {
	// TODO: disconnect
	return
    }
    c.Used = true
    c.FlushQ()

    hostport := cmd.HostPort
    // try to connect
    lconn, err := session.Dial(hostport)
    if err != nil {
	log.Printf("Connection %d: Dial: %v\n", cmd.ConnId, err)
	conn.Write(msg.PackedDisconnectCommand(cmd.ConnId))
	c.Used = false
	return
    }

    go func () {
	defer lconn.Close()
	c.Run(lconn, s.q_req)
    }()
}

func (s *SupplyLine)handleDisconnect(conn net.Conn, cmd *msg.DisconnectCommand) {
    c := &s.connections[cmd.ConnId]
    if !c.Used {
	// something wrong
	return
    }
    c.Q <- cmd
    c.Used = false
}

func (s *SupplyLine)handleData(conn net.Conn, cmd *msg.DataCommand) {
    c := &s.connections[cmd.ConnId]
    if !c.Used {
	// something wrong
	return
    }
    //log.Printf("Data: %d %v\n", cmd.Seq, cmd.Data)
    c.Q <- cmd
}

func (s *SupplyLine)handleCommand(conn net.Conn, cmd msg.Command) {
    log.Printf("handle cmd: %s\n", cmd.Name())
    switch cmd := cmd.(type) {
    case *msg.LinkCommand:
	log.Printf("link from %s\n", cmd.Client)
    case *msg.ConnectCommand:
	log.Printf("connect to %s [%d]\n", cmd.HostPort, cmd.ConnId)
	s.handleConnect(conn, cmd)
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

func (s *SupplyLine)Run() {
    conn := s.back
    q_recv := make(chan msg.Command)
    q_wait := make(chan bool, 1)
    // start receiver
    go msg.Receiver(conn, q_recv, q_wait)
    running := true
    for running {
	log.Println("waiting cmd")
	select {
	case cmd, ok := <-q_recv:
	    if !ok {
		log.Println("q_recv closed")
		running = false
		break
	    }
	    s.handleCommand(conn, cmd)
	    q_wait <- true
	case cmd := <-s.q_req:
	    conn.Write(cmd)
	case <-time.After(time.Minute):
	    log.Println("timeout")
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
