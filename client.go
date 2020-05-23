package YCP

import (
	"fmt"
	"math/rand"
	"net"
	"time"
)

func Dial(host string, port int) (*Session, error) {
	address := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.ListenPacket("udp", ":0")
	if err != nil {
		return nil, err
	}
	remoteAddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, err
	}
	rand.Seed(time.Now().UnixNano())
	conv := rand.Uint32()
	session := NewSession(conn, remoteAddr, conv)
	return session, nil
}
