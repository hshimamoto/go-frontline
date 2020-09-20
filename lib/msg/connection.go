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

const LocalBufferSize = 16384

type Connection struct {
    Id int
    Used bool
    Next *Connection
    Q chan Command
    SeqLocal, SeqRemote int
}

func localReader(id int, conn net.Conn, buf []byte, q_lread chan<- int, q_lwait <-chan bool, running *bool) {
    tag := log.NewTag(fmt.Sprintf("C[%d] localReader", id))
    tag.Printf("start")
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
	    tag.Printf("Read: %v\n", err)
	    break
	}
	if r == 0 {
	    tag.Printf("closed\n")
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
    tag.Printf("end\n")
}

func (c *Connection)Run(conn net.Conn, q_req chan<- []byte) {
    id := c.Id
    tag := log.NewTag(fmt.Sprintf("C[%d]", id))
    tag.Printf("start")

    buf := make([]byte, LocalBufferSize)
    q_lread := make(chan int, 32)
    q_lwait := make(chan bool, 32)
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
		    tag.Printf("invalid seq %d\n", seq)
		}
		dataackcmd := PackedDataAckCommand(cmd)
		q_req <- dataackcmd
		c.SeqRemote++
		if len(cmd.Data) > 0 {
		    conn.Write(cmd.Data)
		}
	    case *DataAckCommand:
		// TODO: ACK
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
		tag.Printf("local closed\n")
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
    close(q_lwait)

    tag.Printf("end")
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
