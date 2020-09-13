// HTTP frontline / frontline
// MIT License Copyright(c) 2020 Hiroshi Shimamoto
// vim:set sw=4 sts=4:
package main

import (
    "net"
    "os"

    "frontline/lib/connection"
    "frontline/lib/log"
    "github.com/hshimamoto/go-session"
)

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
	}
	log.Println("close connection")
    })
    if err != nil {
	log.Printf("NewServer: %v\n", err)
    }
    serv.Run()
}
