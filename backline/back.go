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
    "frontline/lib/supplyline"

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
    cm *msg.ConnectionManager
    q_req chan []byte
    connecting int
    live bool
}

func NewSupplyLine(front string) *SupplyLine {
    s := &SupplyLine{
	front: front,
    }
    s.cm = msg.NewConnectionManager()
    s.q_req = make(chan []byte, 256)
    s.connecting = 0
    s.live = false
    return s
}

func (s *SupplyLine)HandleLink(cmd *msg.LinkCommand) {
}

func (s *SupplyLine)HandleKeepalive(cmd *msg.KeepaliveCommand) {
}

func (s *SupplyLine)HandleConnect(cmd *msg.ConnectCommand) {
    // never happen ignore
}

func (s *SupplyLine)HandleConnectAck(cmd *msg.ConnectAckCommand) {
    s.cm.Queue(cmd)
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

func (s *SupplyLine)main2(conn net.Conn) {
    tag := log.NewTag("Unknown")
    if tcp, ok := conn.(*net.TCPConn); ok {
	tag = log.NewTag(fmt.Sprintf("%v", tcp.RemoteAddr()))
    }

    s.live = true
    supplyline.Main(conn, s, s.q_req)
    s.live = false

    tag.Printf("disconnected from frontline\n")

    s.cm.Clean()

    // wait
    for s.connecting > 0 {
	time.Sleep(time.Second)
    }

    time.Sleep(time.Second * 3)

    tag.Printf("end main\n")
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
    s.main2(conn)
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
    if !s.live {
	log.Println("no link")
	return
    }
    s.connecting++
    // keep ConnectionManager at this moment
    cm := s.cm
    c := cm.GetFree()
    t := time.Now().Add(time.Minute)
    for c == nil {
	if time.Now().After(t) {
	    log.Println("no free connection slot")
	    s.connecting--
	    conn.Close()
	    return
	}
	time.Sleep(time.Second)
	c = cm.GetFree()
    }
    s.connecting--
    if !s.live {
	log.Println("no link")
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
	    cm.PutFree(c)
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
