// HTTP frontline / lib/msg
// MIT License Copyright(c) 2020 Hiroshi Shimamoto
// vim:set sw=4 sts=4:
package msg

import (
    "net"
    "time"

    "frontline/lib/log"
    "frontline/lib/misc"
)

type Connection struct {
    Id int
    Used bool
    Next *Connection
    Q chan Command
}

func (c *Connection)Run(conn net.Conn, q_req chan []byte) {
    log.Printf("start connection %d\n", c.Id)

    id := c.Id

    buf := make([]byte, 8192)
    q_lread := make(chan int)
    q_lwait := make(chan bool)
    // start LocalReader
    go misc.LocalReader(id, conn, buf, q_lread, q_lwait)
    running := true
    for running {
	select {
	case cmd := <-c.Q:
	    switch cmd := cmd.(type) {
	    case *DataCommand:
		// write to local connection
		conn.Write(cmd.Data)
	    case *DisconnectCommand:
		// disconnect from remote
		running = false
	    }
	case r:= <-q_lread:
	    if r > 0 {
		log.Printf("Connection %d: local read %d bytes\n", id, r)
		// DataCommand
		q_req <- PackedDataCommand(id, 0, buf[:r])
	    } else {
		log.Printf("Connection %d: local closed\n", id)
		// DisconnectCommand
		q_req <- PackedDisconnectCommand(id)
		running = false
	    }
	    q_lwait <- true
	case <-time.After(time.Minute):
	    // TODO: periodic process
	}
    }

    log.Printf("end connection %d\n", c.Id)
}
