// HTTP frontline / lib/msg
// MIT License Copyright(c) 2020 Hiroshi Shimamoto
// vim:set sw=4 sts=4:
package msg

import (
    "net"
    "time"

    "frontline/lib/log"
)

type Connection struct {
    Id int
    Used bool
    Next *Connection
    Q chan Command
}

func localReader(id int, conn net.Conn, buf []byte, q_lread chan int, q_lwait chan bool) {
    log.Printf("start localReader %d\n", id)
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
    close(q_lread)
    close(q_lwait)
    log.Printf("end localReader %d\n", id)
}

func (c *Connection)Run(conn net.Conn, q_req chan []byte) {
    log.Printf("start connection %d\n", c.Id)

    id := c.Id

    buf := make([]byte, 8192)
    q_lread := make(chan int)
    q_lwait := make(chan bool)
    // start LocalReader
    go localReader(id, conn, buf, q_lread, q_lwait)
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
		// need to wait localReader done
		go func() {
		    for {
			r := <-q_lread
			q_lwait <- true
			if r == 0 {
			    break
			}
		    }
		}()
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

func (c *Connection)FlushQ() {
    close(c.Q)
    c.Q = make(chan Command)
}

func (c *Connection)Free(done func()) {
    go func() {
	time.Sleep(time.Minute)
	c.Used = false
	done()
    }()
}
