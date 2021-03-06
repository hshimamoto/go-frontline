// HTTP frontline / lib/supplyline
// MIT License Copyright(c) 2020 Hiroshi Shimamoto
// vim:set sw=4 sts=4:
package supplyline

import (
    "fmt"
    "net"
    "time"

    "frontline/lib/msg"
    "frontline/lib/log"
)

func writeall(conn net.Conn, cmd []byte) error {
    n := 0
    for n < len(cmd) {
	w, err := conn.Write(cmd[n:])
	if err != nil {
	    return fmt.Errorf("writeall: %v", err)
	}
	n += w
    }
    return nil
}

func Main(conn net.Conn, h msg.CommandHandler, q_req chan []byte) {
    tag := log.NewTag("Unknown")
    if tcp, ok := conn.(*net.TCPConn); ok {
	tag = log.NewTag(fmt.Sprintf("%v", tcp.RemoteAddr()))
    }

    ticker := time.NewTicker(time.Minute)
    defer ticker.Stop()

    running := true
    q_recv := make(chan msg.Command, 256)
    q_wait := make(chan bool, 256)
    // start receiver
    go func() {
	err := msg.Receiver(conn, q_recv, q_wait, &running)
	tag.Printf("Receiver: %v\n", err)
	close(q_wait)
    }()
    lastrecv := time.Now()
    for running {
	select {
	case cmd, ok := <-q_recv:
	    if !ok {
		tag.Printf("q_recv closed\n")
		running = false
		break
	    }
	    //tag.Printf("recv %s\n", cmd.Name())
	    msg.HandleCommand(h, cmd)
	    q_wait <- true
	    lastrecv = time.Now()
	case cmd := <-q_req:
	    //tag.Printf("send %d bytes\n", len(cmd))
	    if err := writeall(conn, cmd); err != nil {
		tag.Printf("write cmd: %v\n", err)
		running = false
		break
	    }
	case <-ticker.C:
	    // keep alive
	    q_req <- msg.PackedKeepaliveCommand()
	    if time.Now().After(lastrecv.Add(time.Minute * 2)) {
		tag.Printf("keep alive failed\n")
		running = false
		break
	    }
	}
    }
}
