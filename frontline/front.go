// HTTP frontline / frontline
// MIT License Copyright(c) 2020 Hiroshi Shimamoto
// vim:set sw=4 sts=4:
package main

import (
    "net"
    "os"

    "frontline/lib/connection"
    "frontline/lib/log"
    "frontline/lib/msg"
    "github.com/hshimamoto/go-session"
)

type Connection struct {
    Id int
    Used bool
    HostPort string
}

type SupplyLine struct {
    back net.Conn
    connections []Connection
}

func NewSupplyLine(conn net.Conn) (*SupplyLine, error) {
    s := &SupplyLine{
	back: conn,
    }
    s.connections = make([]Connection, 256)
    for i := 0; i < 256; i++ {
	conn := &s.connections[i]
	conn.Id = i
	conn.Used = false
    }
    return s, nil
}

func (s *SupplyLine)handleConnect(conn net.Conn, cmd *msg.ConnectCommand) {
    c := &s.connections[cmd.ConnId]
    if c.Used {
	// TODO: disconnect
	return
    }
    c.Used = true
    c.HostPort = cmd.HostPort
}

func (s *SupplyLine)handleDisconnect(conn net.Conn, cmd *msg.DisconnectCommand) {
    c := &s.connections[cmd.ConnId]
    if !c.Used {
	// something wrong
	return
    }
    c.Used = false
}

func (s *SupplyLine)Run() {
    conn := s.back
    for {
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
	    log.Printf("Read: %v\n", err)
	    break
	}
	if n == 0 {
	    log.Println("no read")
	    break
	}
	log.Printf("recv: %v\n", buf[:n])
	cmd := msg.ParseCommand(buf[:n])
	log.Printf("cmd: %s\n", cmd.Name())
	switch cmd := cmd.(type) {
	case *msg.LinkCommand:
	    log.Printf("link from %s\n", cmd.Client)
	case *msg.ConnectCommand:
	    log.Printf("connect to %s [%d]\n", cmd.HostPort, cmd.ConnId)
	    s.handleConnect(conn, cmd)
	case *msg.DisconnectCommand:
	    log.Printf("disconnect [%d]\n", cmd.ConnId)
	    s.handleDisconnect(conn, cmd)
	case *msg.UnknownCommand:
	    log.Println("unknown command")
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
