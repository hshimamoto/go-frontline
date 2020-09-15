// HTTP frontline / lib/msg
// MIT License Copyright(c) 2020 Hiroshi Shimamoto
// vim:set sw=4 sts=4:
package msg

import (
    "log"
    "net"
)

func Receiver(conn net.Conn, q_recv chan Command, q_wait chan bool) {
    defer close(q_recv)
    buf := make([]byte, 65536)
    n := 0
    for {
	r, err := conn.Read(buf[n:])
	if err != nil {
	    log.Printf("Read: %v\n", err)
	    return
	}
	if r == 0 {
	    log.Println("no read")
	    return
	}
	n += r
	s := 0
	for s < n {
	    log.Printf("try to parse buf[%d:%d]\n", s, n)
	    cmd, clen := ParseCommand(buf[s:n])
	    if cmd == nil {
		log.Println("not enough buffer")
		break
	    }
	    if clen == 0 {
		// parse error
		log.Println("parse error")
		return
	    }
	    log.Printf("Q <- %s\n", cmd.Name())
	    q_recv <- cmd
	    // wait to finish command done
	    <-q_wait
	    log.Println("command done")
	    s += clen
	}
	if s < n {
	    copy(buf, buf[s:n])
	} else {
	    n = 0
	}
    }
}
