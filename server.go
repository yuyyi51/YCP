package YCP

import (
	"github.com/yuyyi51/YCP/packet"
	"github.com/yuyyi51/ylog"
	"fmt"
	"net"
	"sync"
)

type Server struct {
	address       string
	listener      net.PacketConn
	sessionMap    map[uint32]*Session
	sessionMapMux *sync.RWMutex
	sessionChan   chan *Session
	logger        ylog.ILogger
}

func NewServer(address string, logger ylog.ILogger) *Server {
	return &Server{
		address:       address,
		sessionMap:    make(map[uint32]*Session),
		sessionMapMux: new(sync.RWMutex),
		sessionChan:   make(chan *Session, 100),
		logger:        logger,
	}
}

func (server *Server) Listen() error {
	var err error
	server.listener, err = net.ListenPacket("udp", server.address)
	if err != nil {
		return err
	}
	go server.run()
	return nil
}

func (server Server) Accept() *Session {
	return <-server.sessionChan
}

func (server *Server) run() {
	buffer := make([]byte, 1500)
	for {
		n, addr, err := server.listener.ReadFrom(buffer)
		//n, err := listener.Read(buffer)
		if err != nil {
			fmt.Printf("server read error, %v\n", err)
			break
		}
		//fmt.Printf("server read Packet from %s, length [%d]\n", addr, n)
		p, _ := packet.Unpack(buffer[:n])
		server.handlePacket(p, addr)
	}
	fmt.Println("server exit")
}

func (server *Server) handlePacket(packet *packet.Packet, remoteAddr net.Addr) {
	server.sessionMapMux.RLock()
	session, ok := server.sessionMap[packet.Conv]
	if ok {
		session.receivedPackets <- packet
		server.sessionMapMux.RUnlock()
		return
	}
	server.sessionMapMux.RUnlock()

	server.sessionMapMux.Lock()
	session = NewSession(server.listener, remoteAddr, packet.Conv, server.logger)
	server.sessionMap[packet.Conv] = session
	server.sessionChan <- session
	server.sessionMapMux.Unlock()
	server.logger.Debug("new connection %s", session)

	session.receivedPackets <- packet
}
