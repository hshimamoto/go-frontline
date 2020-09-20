// HTTP frontline / lib/msg
// MIT License Copyright(c) 2020 Hiroshi Shimamoto
// vim:set sw=4 sts=4:
package msg

const (
    linkCommand = iota
    keepaliveCommand
    connectCommand
    connectAckCommand
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
    // mask with 0xaa
    for i, b := range []byte(client) {
	buf[i + 2] = b ^ 0xaa
    }
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
    c.Client = ""
    for i := 0; i < clen; i++ {
	c.Client += string(buf[i + 2] ^ 0xaa)
    }
    return c, ptr
}

func (c *LinkCommand)Name() string {
    return "LinkCommand"
}

func PackedKeepaliveCommand() []byte {
    buf := make([]byte, 1)
    buf[0] = keepaliveCommand
    return buf
}

type KeepaliveCommand struct {
}

func ParseKeepaliveCommand(buf []byte) (*KeepaliveCommand, int) {
    c := &KeepaliveCommand{}
    return c, 1
}

func (c *KeepaliveCommand)Name() string {
    return "KeepaliveCommand"
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
    // mask with 0xaa
    for i, b := range []byte(hostport) {
	buf[i + 3] = b ^ 0xaa
    }
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
    c := &ConnectCommand{}
    c.ConnId = connId
    c.HostPort = ""
    for i := 0; i < hlen; i++ {
	c.HostPort += string(buf[i + 3] ^ 0xaa)
    }
    return c, ptr
}

func (c *ConnectCommand)Name() string {
    return "ConnectCommand"
}

func PackedConnectAckCommand(cmd *ConnectCommand, ok bool) []byte {
    buf := make([]byte, 3)
    buf[0] = connectAckCommand
    buf[1] = byte(cmd.ConnId)
    if ok {
	buf[2] = 1
    } else {
	buf[2] = 0
    }
    return buf
}

type ConnectAckCommand struct {
    ConnId int
    Ok bool
}

func ParseConnectAckCommand(buf []byte) (*ConnectAckCommand, int) {
    if len(buf) < 3 {
	return nil, 0
    }
    connId := int(buf[1])
    ok := int(buf[2])
    c := &ConnectAckCommand{}
    c.ConnId = connId
    c.Ok = true
    if ok == 0 {
	c.Ok = false
    }
    return c, 3
}

func (c *ConnectAckCommand)Name() string {
    return "ConnectAckCommand"
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
    case keepaliveCommand: return ParseKeepaliveCommand(buf)
    case connectCommand: return ParseConnectCommand(buf)
    case connectAckCommand: return ParseConnectAckCommand(buf)
    case disconnectCommand: return ParseDisconnectCommand(buf)
    case dataCommand: return ParseDataCommand(buf)
    }
    return &UnknownCommand{}, 0
}
