// HTTP frontline / backline
// MIT License Copyright(c) 2020 Hiroshi Shimamoto
// vim:set sw=4 sts=4:
package main

import (
    "fmt"
    "net"
    "os"
    "time"

    "frontline/lib/log"
    "frontline/lib/msg"
    "github.com/hshimamoto/go-session"
)

type Connection struct {
    Id int
    Used bool
    Next *Connection
    Conn net.Conn
    Q chan *msg.Command
}

func (c *Connection)Run(conn net.Conn, q_req chan *msg.Command) {
    log.Printf("start connection %d\n", c.Id)
    cmd := &msg.Command{}
    cmd.Name = "CONNECT"
    cmd.Client = "Unknown" // No need?
    cmd.ConnId = c.Id
    cmd.DataLen = 0
    cmd.Data = []byte{}
    q_req <- cmd
    log.Printf("end connection %d\n", c.Id)
}

type SupplyLine struct {
    front string
    connections []Connection
    free *Connection
    q_req chan *msg.Command
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
	conn.Q = make(chan *msg.Command)
	prev = conn
    }
    s.free = prev
    s.q_req = make(chan *msg.Command)
    return s
}

func (s *SupplyLine)main(conn net.Conn) {
    hostname, err := os.Hostname()
    if err != nil {
	log.Printf("unable to get hostname: %v\n", err)
	hostname = "Unknown"
    }
    cmd := &msg.Command{}
    cmd.Name = "LINK"
    cmd.Client = fmt.Sprintf("%s-%d", hostname, os.Getpid())
    cmd.ConnId = 0
    cmd.DataLen = 0
    cmd.Data = []byte{}
    if _, err := conn.Write(cmd.Pack()); err != nil {
	log.Printf("send command error: %v\n", err)
	return
    }
    for {
	select {
	case cmd := <-s.q_req:
	    conn.Write(cmd.Pack())
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
