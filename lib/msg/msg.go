// HTTP frontline / lib/msg
// MIT License Copyright(c) 2020 Hiroshi Shimamoto
// vim:set sw=4 sts=4:
package msg

const (
    linkCommand = iota
    connectCommand
    disconnectCommand
    dataCommand
)

type Command interface {
    Name() string
}

func PackedLinkCommand(client string) []byte {
    err := []byte{}
    clen := len(client)
    if clen >= 128 {
	return err
    }
    buf := make([]byte, len(client) + 2)
    buf[0] = linkCommand
    buf[1] = byte(clen)
    copy(buf[2:], client)
    return buf
}

type LinkCommand struct {
    Client string
}

func ParseLinkCommand(buf []byte) (*LinkCommand, int) {
    if len(buf) < 2 {
	return nil, 0
    }
    c := &LinkCommand{}
    clen := int(buf[1])
    ptr := 2 + clen
    if len(buf) < ptr {
	return nil, 0
    }
    c.Client = string(buf[2:ptr])
    return c, ptr
}

func (c *LinkCommand)Name() string {
    return "LinkCommand"
}

func PackedConnectCommand(connId int, hostport string) []byte {
    err := []byte{}
    if connId >= 256 {
	return err
    }
    hlen := len(hostport)
    if hlen >= 128 {
	return err
    }
    buf := make([]byte, 3 + hlen)
    buf[0] = connectCommand
    buf[1] = byte(connId)
    buf[2] = byte(hlen)
    copy(buf[3:], hostport)
    return buf
}

type ConnectCommand struct {
    ConnId int
    HostPort string
}

func ParseConnectCommand(buf []byte) (*ConnectCommand, int) {
    if len(buf) < 3 {
	return nil, 0
    }
    connId := int(buf[1])
    hlen := int(buf[2])
    ptr := 3 + hlen
    if len(buf) < ptr {
	return nil, 0
    }
    return &ConnectCommand{ ConnId: connId, HostPort: string(buf[3:3+hlen]) }, ptr
}

func (c *ConnectCommand)Name() string {
    return "ConnectCommand"
}

func PackedDisconnectCommand(connId int) []byte {
    err := []byte{}
    if connId >= 256 {
	return err
    }
    buf := make([]byte, 2)
    buf[0] = disconnectCommand
    buf[1] = byte(connId)
    return buf
}

type DisconnectCommand struct {
    ConnId int
}

func ParseDisconnectCommand(buf []byte) (*DisconnectCommand, int) {
    if len(buf) < 2 {
	return nil, 0
    }
    connId := int(buf[1])
    return &DisconnectCommand{ ConnId: connId }, 2
}

func (c *DisconnectCommand)Name() string {
    return "DisconnectCommand"
}

func PackedDataCommand(connId, seq int, data []byte) []byte {
    err := []byte{}
    if connId >= 256 {
	return err
    }
    if seq >= 256 {
	return err
    }
    datalen := len(data)
    if datalen >= 32768 {
	return err
    }
    msglen := 3 + 2 + datalen
    buf := make([]byte, msglen)
    buf[0] = dataCommand
    buf[1] = byte(connId)
    buf[2] = byte(seq)
    buf[3] = byte((datalen >> 8) & 0xff)
    buf[4] = byte(datalen & 0xff)
    // mask with 0xaa
    for i := 0; i < datalen; i++ {
	buf[i + 5] = data[i] ^ 0xaa
    }
    return buf
}

type DataCommand struct {
    ConnId int
    Seq int
    Data []byte
}

func ParseDataCommand(buf []byte) (*DataCommand, int) {
    if len(buf) < 4 {
	return nil, 0
    }
    connId := int(buf[1])
    seq := int(buf[2])
    datalen := (int(buf[3]) << 8) | int(buf[4])
    if len(buf) < 5 + datalen {
	return nil, 0
    }
    c := &DataCommand{}
    c.ConnId = connId
    c.Seq = seq
    c.Data = make([]byte, datalen)
    // mask with 0xaa
    for i := 0; i < datalen; i++ {
	c.Data[i] = buf[i + 5] ^ 0xaa
    }
    return c, 5 + datalen
}

func (c *DataCommand)Name() string {
    return "DataCommand"
}

type UnknownCommand struct {
}

func (c *UnknownCommand)Name() string {
    return "UnknownCommand"
}

func ParseCommand(buf []byte) (Command, int) {
    switch buf[0] {
    case linkCommand: return ParseLinkCommand(buf)
    case connectCommand: return ParseConnectCommand(buf)
    case disconnectCommand: return ParseDisconnectCommand(buf)
    case dataCommand: return ParseDataCommand(buf)
    }
    return &UnknownCommand{}, 0
}
