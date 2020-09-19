// HTTP frontline / lib/misc
// MIT License Copyright(c) 2020 Hiroshi Shimamoto
// vim:set sw=4 sts=4:
package misc

import (
    "net"
    "time"

    "frontline/lib/log"
    "frontline/lib/msg"
)

func ConnectionRun(id int, conn net.Conn, q_cmd <-chan msg.Command, q_req chan<- []byte) {
    buf := make([]byte, 8192)
    q_lread := make(chan int)
    q_lwait := make(chan bool)
    // start LocalReader
    go LocalReader(id, conn, buf, q_lread, q_lwait)
    running := true
    for running {
	select {
	case cmd := <-q_cmd:
	    switch cmd := cmd.(type) {
	    case *msg.DataCommand:
		// write to local connection
		conn.Write(cmd.Data)
	    case *msg.DisconnectCommand:
		// disconnect from remote
		running = false
	    }
	case r:= <-q_lread:
	    if r > 0 {
		log.Printf("Connection %d: local read %d bytes\n", id, r)
		// DataCommand
		q_req <- msg.PackedDataCommand(id, 0, buf[:r])
	    } else {
		log.Printf("Connection %d: local closed\n", id)
		// DisconnectCommand
		q_req <- msg.PackedDisconnectCommand(id)
		running = false
	    }
	    q_lwait <- true
	case <-time.After(time.Minute):
	    // TODO: periodic process
	}
    }
}
