// HTTP frontline / backline
// MIT License Copyright(c) 2020 Hiroshi Shimamoto
// vim:set sw=4 sts=4:
package main

import (
    "net"
    "os"

    "frontline/lib/log"
    "github.com/hshimamoto/go-session"
)

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
    serv.Run()
}
