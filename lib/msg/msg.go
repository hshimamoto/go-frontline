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
    dataAckCommand
)

type Command interface {
    Name() string
    Id() int
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

func (c *LinkCommand)Id() int {
    return -1
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

func (c *KeepaliveCommand)Id() int {
    return -1
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

func (c *ConnectCommand)Id() int {
    return c.ConnId
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

func (c *ConnectAckCommand)Id() int {
    return c.ConnId
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

func (c *DisconnectCommand)Id() int {
    return c.ConnId
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
    if len(buf) < 5 {
	return nil, 0
    }
    connId := int(buf[1])
    seq := int(buf[2])
    datalen := (int(buf[3]) << 8) | int(buf[4])
    if datalen >= 32768 {
	return nil, -1
    }
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

func (c *DataCommand)Id() int {
    return c.ConnId
}

func PackedDataAckCommand(cmd *DataCommand) []byte {
    buf := make([]byte, 5)
    datalen := len(cmd.Data)
    buf[0] = dataAckCommand
    buf[1] = byte(cmd.ConnId)
    buf[2] = byte(cmd.Seq)
    buf[3] = byte((datalen >> 8) & 0xff)
    buf[4] = byte(datalen & 0xff)
    return buf
}

type DataAckCommand struct {
    ConnId int
    Seq int
    DataLen int
}

func ParseDataAckCommand(buf []byte) (*DataAckCommand, int) {
    if len(buf) < 5 {
	return nil, 0
    }
    connId := int(buf[1])
    seq := int(buf[2])
    datalen := (int(buf[3]) << 8) | int(buf[4])
    c := &DataAckCommand{}
    c.ConnId = connId
    c.Seq = seq
    c.DataLen = datalen
    return c, 5
}

func (c *DataAckCommand)Name() string {
    return "DataAckCommand"
}

func (c *DataAckCommand)Id() int {
    return c.ConnId
}

type UnknownCommand struct {
}

func (c *UnknownCommand)Name() string {
    return "UnknownCommand"
}

func (c *UnknownCommand)Id() int {
    return -1
}

func ParseCommand(buf []byte) (Command, int) {
    switch buf[0] {
    case linkCommand: return ParseLinkCommand(buf)
    case keepaliveCommand: return ParseKeepaliveCommand(buf)
    case connectCommand: return ParseConnectCommand(buf)
    case connectAckCommand: return ParseConnectAckCommand(buf)
    case disconnectCommand: return ParseDisconnectCommand(buf)
    case dataCommand: return ParseDataCommand(buf)
    case dataAckCommand: return ParseDataAckCommand(buf)
    }
    return &UnknownCommand{}, -1
}

type CommandHandler interface {
    HandleLink(cmd *LinkCommand)
    HandleKeepalive(cmd *KeepaliveCommand)
    HandleConnect(cmd *ConnectCommand)
    HandleConnectAck(cmd *ConnectAckCommand)
    HandleDisconnect(cmd *DisconnectCommand)
    HandleData(cmd *DataCommand)
    HandleDataAck(cmd *DataAckCommand)
}

func HandleCommand(h CommandHandler, cmd Command) {
    switch cmd := cmd.(type) {
    case *LinkCommand: h.HandleLink(cmd)
    case *KeepaliveCommand: h.HandleKeepalive(cmd)
    case *ConnectCommand: h.HandleConnect(cmd)
    case *ConnectAckCommand: h.HandleConnectAck(cmd)
    case *DisconnectCommand: h.HandleDisconnect(cmd)
    case *DataCommand: h.HandleData(cmd)
    case *DataAckCommand: h.HandleDataAck(cmd)
    }
}
