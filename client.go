package YCP

import (
	"code.int-2.me/yuyyi51/YCP/packet"
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
	go ListenPacket(session)
	return session, nil
}

func ListenPacket(session *Session) {
	buffer := make([]byte, 1500)
	for {
		n, _, err := session.conn.ReadFrom(buffer)
		//n, err := listener.Read(buffer)
		if err != nil {
			fmt.Printf("server read error, %v\n", err)
			break
		}
		//fmt.Printf("server read Packet from %s, length [%d]\n", addr, n)
		p, _ := packet.Unpack(buffer[:n])
		session.receivedPackets <- p
		//session.handlePacket(p, addr)
	}
	fmt.Println("listener exit")
}
