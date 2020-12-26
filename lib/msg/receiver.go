// HTTP frontline / lib/msg
// MIT License Copyright(c) 2020 Hiroshi Shimamoto
// vim:set sw=4 sts=4:
package msg

import (
    "fmt"
    "net"
    "time"

    "frontline/lib/log"
)

func Receiver(conn net.Conn, q_recv chan<- Command, q_wait <-chan bool, running *bool) error {
    defer close(q_recv)
    tag := log.NewTag("Receiver")
    if tcp, ok := conn.(*net.TCPConn); ok {
	tag = log.NewTag(fmt.Sprintf("Receiver[%v]", tcp.RemoteAddr()))
    }

    buf := make([]byte, 65536)
    n := 0
    s := 0
    for *running {
	now := time.Now()
	conn.SetReadDeadline(now.Add(time.Second))
	r, err := conn.Read(buf[n:])
	if err != nil {
	    if operr, ok := err.(*net.OpError); ok {
		if operr.Timeout() {
		    continue
		}
	    }
	    return err
	}
	if r == 0 {
	    return fmt.Errorf("no read")
	}
	n += r
	for s < n {
	    //tag.Printf("try to parse buf[%d:%d]\n", s, n)
	    cmd, clen := ParseCommand(buf[s:n])
	    if clen == 0 {
		//tag.Printf("not enough buffer (clen == 0)\n")
		break
	    }
	    if clen == -1 {
		// parse error
		if n - s > 8 {
		    n = s + 8
		}
		return fmt.Errorf("command parse error: %v", buf[s:n])
	    }
	    q_recv <- cmd
	    // wait to finish command done
	    <-q_wait
	    s += clen
	}
	if s < n {
	    //tag.Printf("check %d %d\n", s, n)
	    if s > 32768 {
		tag.Printf("slide buffer %d %d\n", s, n)
		copy(buf, buf[s:n])
		n -= s
		s = 0
	    }
	} else {
	    n = 0
	    s = 0
	}
    }
    return fmt.Errorf("not running")
}
