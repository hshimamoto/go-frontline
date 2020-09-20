// HTTP frontline / lib/msg
// MIT License Copyright(c) 2020 Hiroshi Shimamoto
// vim:set sw=4 sts=4:
package msg

import (
    "fmt"
    "net"

    "frontline/lib/log"
)

func Receiver(conn net.Conn, q_recv chan<- Command, q_wait <-chan bool) error {
    defer close(q_recv)
    tag := log.NewTag("Receiver")
    if tcp, ok := conn.(*net.TCPConn); ok {
	tag = log.NewTag(fmt.Sprintf("Receiver[%v]", tcp.RemoteAddr()))
    }

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
	    tag.Printf("try to parse buf[%d:%d]\n", s, n)
	    cmd, clen := ParseCommand(buf[s:n])
	    if cmd == nil {
		tag.Printf("not enough buffer (cmd == nil)\n")
		break
	    }
	    if clen == 0 {
		tag.Printf("not enough buffer (clen == 0)\n")
		break
	    }
	    if clen == -1 {
		// parse error
		if n > 8 {
		    n = 8
		}
		return fmt.Errorf("command parse error: %v\n", buf[s:n])
	    }
	    q_recv <- cmd
	    // wait to finish command done
	    <-q_wait
	    s += clen
	}
	if s < n {
	    copy(buf, buf[s:n])
	} else {
	    n = 0
	}
    }
}
