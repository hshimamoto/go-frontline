// HTTP frontline / lib/msg
// MIT License Copyright(c) 2020 Hiroshi Shimamoto
// vim:set sw=4 sts=4:
package msg

import (
    "fmt"
    "net"

    "frontline/lib/log"
)

func Receiver(conn net.Conn, q_recv chan Command, q_wait chan bool) error {
    defer close(q_recv)
    buf := make([]byte, 65536)
    n := 0
    for {
	r, err := conn.Read(buf[n:])
	if err != nil {
	    return err
	}
	if r == 0 {
	    return fmt.Errorf("no read")
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
		return fmt.Errorf("command parse error: %v\n", buf[s:n])
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
