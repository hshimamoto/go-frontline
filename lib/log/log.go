// HTTP frontline / lib/log
// MIT License Copyright(c) 2020 Hiroshi Shimamoto
// vim:set sw=4 sts=4:
package log

import (
    "fmt"
    "log"
    "os"
)

func Setup(cmd string) {
    log.SetFlags(log.Flags() | log.Lmsgprefix)
    log.SetPrefix(fmt.Sprintf("[%d <%s>] ", os.Getpid(), cmd))
}

func Println(v ...interface{}) {
    log.Println(v...)
}

func Printf(format string, v ...interface{}) {
    log.Printf(format, v...)
}

type tag interface {
    Printf(format string, v ...interface{})
}

type tagString struct {
    s string
}

type tagFunc struct {
    f func() string
}

func NewTag(t interface{}) tag {
    switch t := t.(type) {
    case string: return &tagString{ s:t }
    case func() string: return &tagFunc{ f:t }
    default: return nil
    }
}

func (t *tagString)Printf(format string, v ...interface{}) {
    msg := fmt.Sprintf(format, v...)
    Printf("%s: %s", t.s, msg)
}

func (t *tagFunc)Printf(format string, v ...interface{}) {
    msg := fmt.Sprintf(format, v...)
    Printf("%s: %s", t.f(), msg)
}
