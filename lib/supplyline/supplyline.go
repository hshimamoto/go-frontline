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

func Main(conn net.Conn, h msg.CommandHandler, q_req chan []byte, keepalive *int) {
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
    for running {
	select {
	case cmd, ok := <-q_recv:
	    if !ok {
		tag.Printf("q_recv closed\n")
		running = false
		break
	    }
	    tag.Printf("recv %s\n", cmd.Name())
	    msg.HandleCommand(h, cmd)
	    q_wait <- true
	case cmd := <-q_req:
	    n := 0
	    tag.Printf("send %d bytes\n", len(cmd))
	    for n < len(cmd) {
		w, err := conn.Write(cmd[n:])
		if err != nil {
		    tag.Printf("write cmd: %v\n", err)
		    running = false
		    break
		}
		n += w
	    }
	case <-ticker.C:
	    // keep alive
	    q_req <- msg.PackedKeepaliveCommand()
	    *keepalive++
	    if *keepalive >= 3 {
		tag.Printf("keep alive failed\n")
		running = false
		break
	    }
	}
    }
}

func CleanConnections(connections []msg.Connection) {
    for i := 0; i < 256; i++ {
	c := &connections[i]
	c.Cancel()
    }

    for i := 0; i < 256; i++ {
	c := &connections[i]
	for c.Used {
	    time.Sleep(time.Second)
	}
    }
}
