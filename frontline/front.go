// HTTP frontline / frontline
// MIT License Copyright(c) 2020 Hiroshi Shimamoto
// vim:set sw=4 sts=4:
package main

import (
    "fmt"
    "log"
    "os"
)

func main() {
    log.SetFlags(log.Flags() | log.Lmsgprefix)
    log.SetPrefix(fmt.Sprintf("[%d <frontline>] ", os.Getpid()))

    log.Println("start")
}
