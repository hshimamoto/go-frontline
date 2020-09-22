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
    "frontline/lib/supplyline"

    "github.com/hshimamoto/go-session"
)

type SupplyLine struct {
    cm *msg.ConnectionManager
    q_req chan []byte
}

func NewSupplyLine() *SupplyLine {
    s := &SupplyLine{}
    s.cm = msg.NewConnectionManager()
    s.q_req = make(chan []byte, 256)
    return s
}

func (s *SupplyLine)HandleLink(cmd *msg.LinkCommand) {
}

func (s *SupplyLine)HandleKeepalive(cmd *msg.KeepaliveCommand) {
}

func (s *SupplyLine)HandleConnect(cmd *msg.ConnectCommand) {
    c := s.cm.Get(cmd.ConnId)
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
    // never happen ignore
}

func (s *SupplyLine)HandleDisconnect(cmd *msg.DisconnectCommand) {
    s.cm.Queue(cmd)
}

func (s *SupplyLine)HandleData(cmd *msg.DataCommand) {
    s.cm.Queue(cmd)
}

func (s *SupplyLine)HandleDataAck(cmd *msg.DataAckCommand) {
    s.cm.Queue(cmd)
}

func (s *SupplyLine)Run(conn net.Conn) {
    tag := log.NewTag("Unknown")
    if tcp, ok := conn.(*net.TCPConn); ok {
	tag = log.NewTag(fmt.Sprintf("%v", tcp.RemoteAddr()))
    }
    tag.Printf("start main\n")

    tag.Printf("connected from backline\n")
    supplyline.Main(conn, s, s.q_req)
    tag.Printf("disconnected from backline\n")

    s.cm.Clean()
    time.Sleep(time.Second * 3)

    tag.Printf("end main\n")
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
	s := NewSupplyLine()
	s.Run(conn)
	log.Println("close connection")
    })
    if err != nil {
	log.Printf("NewServer: %v\n", err)
	return
    }
    serv.Run()
}
