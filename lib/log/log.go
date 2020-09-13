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
