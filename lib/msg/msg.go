// HTTP frontline / lib/msg
// MIT License Copyright(c) 2020 Hiroshi Shimamoto
// vim:set sw=4 sts=4:
package msg

type Command struct {
    Name string
    Client string
    ConnId int
    DataLen int
    Data []byte
}

func ParseCommand(buf []byte) *Command {
    c := &Command{}
    switch buf[0] {
    case 0: c.Name = "LINK"
    default:
	return c
    }
    clen := int(buf[1])
    ptr := 2 + clen
    c.Client = string(buf[2:ptr])
    c.ConnId = int(buf[ptr])
    ptr++
    // TODO: parse data
    return c
}

func (c *Command)Pack() []byte {
    buf := make([]byte, len(c.Client) + c.DataLen + 32)
    err := []byte{}
    switch c.Name {
    case "LINK": buf[0] = 0
    default:
	return err
    }
    clen := len(c.Client)
    if clen >= 128 {
	return err
    }
    buf[1] = byte(clen)
    copy(buf[2:], c.Client)
    if c.ConnId >= 256 {
	return err
    }
    ptr := 2 + clen
    buf[ptr] = byte(c.ConnId)
    ptr++
    // TODO: pack data
    return buf[:ptr]
}
