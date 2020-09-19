// HTTP frontline / lib/misc
// MIT License Copyright(c) 2020 Hiroshi Shimamoto
// vim:set sw=4 sts=4:
package misc

import (
    "net"

    "frontline/lib/log"
)

func LocalReader(id int, conn net.Conn, buf []byte, q_lread chan int, q_lwait chan bool) {
    for {
	r, err := conn.Read(buf)
	if err != nil {
	    log.Printf("Connection %d: Read: %v\n", id, err)
	    break
	}
	if r == 0 {
	    log.Printf("Connection %d: closed\n", id)
	    break
	}
	// send
	q_lread <- r
	// wait handled
	<-q_lwait
    }
    q_lread <- 0
    <-q_lwait
}

