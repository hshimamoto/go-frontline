// HTTP frontline / lib/msg
// MIT License Copyright(c) 2020 Hiroshi Shimamoto
// vim:set sw=4 sts=4:
package msg

const (
    linkCommand = iota
    connectCommand
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

func ParseLinkCommand(buf []byte) *LinkCommand {
    c := &LinkCommand{}
    clen := int(buf[1])
    ptr := 2 + clen
    c.Client = string(buf[2:ptr])
    return c
}

func (c *LinkCommand)Name() string {
    return "LinkCommand"
}

func PackedConnectCommand(connId int) []byte {
    err := []byte{}
    if connId >= 256 {
	return err
    }
    buf := make([]byte, 2)
    buf[0] = connectCommand
    buf[1] = byte(connId)
    return buf
}

type ConnectCommand struct {
    ConnId int
}

func ParseConnectCommand(buf []byte) *ConnectCommand {
    return &ConnectCommand{ ConnId: int(buf[1]) }
}

func (c *ConnectCommand)Name() string {
    return "ConnectCommand"
}

type UnknownCommand struct {
}

func (c *UnknownCommand)Name() string {
    return "UnknownCommand"
}

func ParseCommand(buf []byte) Command {
    switch buf[0] {
    case linkCommand: return ParseLinkCommand(buf)
    case connectCommand: return ParseConnectCommand(buf)
    }
    return &UnknownCommand{}
}
