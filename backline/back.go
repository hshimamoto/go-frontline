// HTTP frontline / backline
// MIT License Copyright(c) 2020 Hiroshi Shimamoto
// vim:set sw=4 sts=4:
package main

import (
    "net"
    "os"
    "time"

    "frontline/lib/log"
    "github.com/hshimamoto/go-session"
)

type SupplyLine struct {
    front string
}

func NewSupplyLine(front string) *SupplyLine {
    s := &SupplyLine{
	front: front,
    }
    return s
}

func (s *SupplyLine)main(conn net.Conn) {
    for {
	time.Sleep(time.Second)
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

    serv, err := session.NewServer(listen, func(conn net.Conn) {
	conn.Close()
    })
    if err != nil {
	log.Printf("NewServer: %v\n", err)
    }

    // now we can start to communicate with frontline
    s := NewSupplyLine(front)
    go s.Run()

    serv.Run()
}
