// HTTP frontline / lib/connection
// MIT License Copyright(c) 2020 Hiroshi Shimamoto
// vim:set sw=4 sts=4:
package connection

import (
    "fmt"
    "net"
    "time"
)

func EnableKeepAlive(conn net.Conn) error {
    tc, ok := conn.(*net.TCPConn)
    if !ok {
	return fmt.Errorf("not TCP connection")
    }
    if err := tc.SetKeepAlive(true); err != nil {
	return err
    }
    if err := tc.SetKeepAlivePeriod(time.Minute); err != nil {
	return err
    }
    return nil
}
