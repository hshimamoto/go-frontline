// HTTP frontline / lib/msg
// MIT License Copyright(c) 2020 Hiroshi Shimamoto
// vim:set sw=4 sts=4:
package msg

import (
    "net"
    "time"

    "frontline/lib/log"
)

const LocalBufferSize = 16384

type Connection struct {
    Id int
    Used bool
    Next *Connection
    Q chan Command
    SeqLocal, SeqRemote int
}

func localReader(id int, conn net.Conn, buf []byte, q_lread chan int, q_lwait chan bool, running *bool) {
    log.Printf("start localReader %d\n", id)
    for *running {
	now := time.Now()
	conn.SetReadDeadline(now.Add(time.Second))
	r, err := conn.Read(buf)
	if err != nil {
	    if operr, ok := err.(*net.OpError); ok {
		if operr.Timeout() {
		    continue
		}
	    }
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
	// less than 100ms
	if time.Now().Before(now.Add(time.Millisecond * 100)) {
	    if r < (LocalBufferSize / 2) {
		time.Sleep(time.Millisecond * 100)
	    }
	}
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

    buf := make([]byte, LocalBufferSize)
    q_lread := make(chan int)
    q_lwait := make(chan bool)
    // start LocalReader
    running := true
    go localReader(id, conn, buf, q_lread, q_lwait, &running)
    for running {
	select {
	case cmd := <-c.Q:
	    switch cmd := cmd.(type) {
	    case *DataCommand:
		// write to local connection
		seq := cmd.Seq
		if seq != c.SeqRemote {
		    log.Printf("invalid seq %d\n", seq)
		}
		c.SeqRemote++
		if len(cmd.Data) > 0 {
		    conn.Write(cmd.Data)
		}
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
		// DataCommand
		datacmd := PackedDataCommand(id, c.SeqLocal, buf[:r])
		c.SeqLocal++
		q_req <- datacmd
	    } else {
		log.Printf("Connection %d: local closed\n", id)
		// DisconnectCommand
		q_req <- PackedDisconnectCommand(id)
		running = false
	    }
	    q_lwait <- true
	case <-time.After(time.Minute):
	    // TODO: periodic process
	    // Send Empty Data
	    datacmd := PackedDataCommand(id, c.SeqLocal, []byte{})
	    c.SeqLocal++
	    q_req <- datacmd
	}
    }

    time.Sleep(time.Second * 3)

    log.Printf("end connection %d\n", c.Id)
}

func (c *Connection)Init(id int) {
    c.Id = id
    c.Used = false
    c.Next = nil
    c.Q = make(chan Command)
    c.SeqLocal = 0
    c.SeqRemote = 0
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
