// HTTP frontline / frontline
// MIT License Copyright(c) 2020 Hiroshi Shimamoto
// vim:set sw=4 sts=4:
package main

import (
    "fmt"
    "log"
    "net"
    "os"

    "github.com/hshimamoto/go-session"
)

func main() {
    log.SetFlags(log.Flags() | log.Lmsgprefix)
    log.SetPrefix(fmt.Sprintf("[%d <frontline>] ", os.Getpid()))

    listen := ":8443"
    if len(os.Args) > 1 {
	listen = os.Args[1]
    }

    log.Printf("start listen %s", listen)

    serv, err := session.NewServer(listen, func(conn net.Conn) {
	conn.Close()
    })
    if err != nil {
	log.Printf("NewServer: %v\n", err)
    }
    serv.Run()
}
