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
    freeing bool
    ctrl_q chan bool
    connected bool
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
    localwaiter := func() {
	for {
	    r := <-q_lread
	    q_lwait <- true
	    if r == 0 {
		break
	    }
	}
    }
    for running {
	select {
	case cmd := <-c.Q:
	    switch cmd := cmd.(type) {
	    case *ConnectAckCommand:
		if c.connected {
		    // ignore
		    break
		}
		if !cmd.Ok {
		    conn.Write([]byte("HTTP/1.0 400 Bad Request\r\n\r\n"))
		    running = false
		    go localwaiter()
		    break
		}
		conn.Write([]byte("HTTP/1.0 200 Established\r\n\r\n"))
		c.connected = true
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
		go localwaiter()
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
	case <-c.ctrl_q:
	    // cancel
	    running = false
	    // need to wait localReader done
	    go localwaiter()
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
    c.Q = make(chan Command, 32)
    c.SeqLocal = 0
    c.SeqRemote = 0
    c.ctrl_q = make(chan bool)
    c.connected = false
    c.freeing = false
}

func (c *Connection)Cancel() {
    if c.Used {
	if !c.freeing {
	    c.ctrl_q <- true
	}
    }
}

func (c *Connection)FlushQ() {
    close(c.Q)
    c.Q = make(chan Command, 32)
    close(c.ctrl_q)
    c.ctrl_q = make(chan bool)
}

func (c *Connection)Free(done func()) {
    c.freeing = true
    go func() {
	time.Sleep(time.Minute)
	c.Used = false
	c.connected = false
	done()
	c.freeing = false
    }()
}

type ConnectionManager struct {
    connections []Connection
    free *Connection
}

func NewConnectionManager() *ConnectionManager {
    cm := &ConnectionManager{}
    cm.connections = make([]Connection, 256)
    var prev *Connection = nil
    for i := 0; i < 256; i++ {
	c := &cm.connections[i]
	c.Init(i)
	c.Next = prev
	prev = c
    }
    cm.free = prev
    return cm
}

func (cm *ConnectionManager)Queue(cmd Command) {
    connId := cmd.Id()
    if connId < 0 || connId >= 256 {
	return
    }
    c := &cm.connections[connId]
    if !c.Used {
	return
    }
    c.Q <- cmd
}

func (cm *ConnectionManager)GetFree() *Connection {
    c := cm.free
    if c != nil {
	cm.free = c.Next
    }
    return c
}

func (cm *ConnectionManager)Get(i int) *Connection {
    if i < 0 || i >= 256 {
	return nil
    }
    return &cm.connections[i]
}

func (cm *ConnectionManager)PutFree(c *Connection) {
    c.Next = cm.free
    cm.free = c
}

func (cm *ConnectionManager)Connections() []Connection {
    return cm.connections
}
