package packet

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"sync"
	"time"
)

const (
	MTU             = 1500
	Ipv4Header      = 60
	Ipv6Header      = 40
	UdpHeader       = 20
	Ipv4PayloadSize = MTU - Ipv4Header - UdpHeader
	Ipv6PayloadSize = MTU - Ipv6Header - UdpHeader
)

type packet struct {
	conv   uint32
	seq    uint64
	rcv    uint64
	frames []frame
}

type frame struct {
	command uint8
	size    uint16
	data    []byte
}

func (p packet) String() string {
	buffer := bytes.Buffer{}
	buffer.WriteString(fmt.Sprintf("packet conv: %d, seq: %d, rcv: %d\n", p.conv, p.seq, p.rcv))
	for _, frame := range p.frames {
		buffer.WriteString(fmt.Sprintf("--%s\n", frame.String()))
	}
	return buffer.String()
}

func (f frame) String() string {
	command := ""
	switch f.command {
	default:
		command = fmt.Sprintf("Unknown(%d)", f.command)
	}
	return fmt.Sprintf("frame command: %s, size: %d, data: 0x%s", command, f.size, hex.EncodeToString(f.data))
}

func (p packet) Pack() []byte {
	result := make([]byte, MTU)
	offset := 0
	offset = writeUint32(result, offset, p.conv)
	offset = writeUint64(result, offset, p.seq)
	offset = writeUint64(result, offset, p.rcv)
	for _, frame := range p.frames {
		offset = writeUint8(result, offset, frame.command)
		offset = writeUint16(result, offset, frame.size)
		offset = writeBytes(result, offset, frame.data)
	}
	return result[:offset]
}

func Unpack(b []byte) (packet, error) {
	offset := 0
	conv, offset := readUint32(b, offset)
	seq, offset := readUint64(b, offset)
	rcv, offset := readUint64(b, offset)
	frames := make([]frame, 0, 10)
	for offset < len(b) {
		var command uint8
		var size uint16
		var data []byte
		command, offset = readUint8(b, offset)
		size, offset = readUint16(b, offset)
		data, offset = readBytes(b, offset, int(size))
		frame := frame{
			command: command,
			size:    size,
			data:    data,
		}
		frames = append(frames, frame)
	}
	p := packet{
		conv:   conv,
		seq:    seq,
		rcv:    rcv,
		frames: frames,
	}
	return p, nil
}

func writeUint8(b []byte, offset int, v uint8) int {
	b[offset] = byte(v)
	return offset + 1
}

func writeUint16(b []byte, offset int, v uint16) int {
	binary.BigEndian.PutUint16(b[offset:], v)
	return offset + 2
}

func writeUint32(b []byte, offset int, v uint32) int {
	binary.BigEndian.PutUint32(b[offset:], v)
	return offset + 4
}

func writeUint64(b []byte, offset int, v uint64) int {
	binary.BigEndian.PutUint64(b[offset:], v)
	return offset + 8
}

func writeBytes(b []byte, offset int, v []byte) int {
	copy(b[offset:], v[:])
	return offset + len(v)
}

func readUint8(b []byte, offset int) (uint8, int) {
	v := b[offset]
	return v, offset + 1
}

func readUint16(b []byte, offset int) (uint16, int) {
	v := binary.BigEndian.Uint16(b[offset : offset+2])
	return v, offset + 2
}

func readUint32(b []byte, offset int) (uint32, int) {
	v := binary.BigEndian.Uint32(b[offset : offset+4])
	return v, offset + 4
}

func readUint64(b []byte, offset int) (uint64, int) {
	v := binary.BigEndian.Uint64(b[offset : offset+8])
	return v, offset + 8
}

func readBytes(b []byte, offset int, len int) ([]byte, int) {
	return b[offset : offset+len], offset + len
}

func CreateUdpConn(remoteHost string, remotePort int) (conn net.Conn, err error) {
	remoteAddress := fmt.Sprintf("%s:%d", remoteHost, remotePort)
	conn, err = net.Dial("udp", remoteAddress)
	return
}

func CreateUdpListener(host string, port int) (listener net.Conn, err error) {
	address := fmt.Sprintf("%s:%d", host, port)
	udpAddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return
	}
	listener, err = net.ListenUDP("udp", udpAddr)
	return
}

func server(host string, port int, wq *sync.WaitGroup) {
	listener, err := CreateUdpListener(host, port)
	if err != nil {
		fmt.Printf("server createUdpListener error: %v\n", err)
		os.Exit(1)
	}
	buffer := make([]byte, 1500)
	for {
		n, err := listener.Read(buffer)
		if err != nil {
			fmt.Printf("server read error, %v\n", err)
			break
		}
		fmt.Printf("server read packet length [%d]\n", n)
		p, _ := Unpack(buffer[:n])
		fmt.Printf("%s\n", p.String())
	}
	fmt.Println("server exit")
	wq.Done()
}

func client(host string, port int) {
	conn, err := CreateUdpConn(host, port)
	if err != nil {
		fmt.Printf("client createUdpConn error: %v", err)
		os.Exit(1)
	}
	for i := 1; i <= 100; i++ {
		p := packet{
			conv: 1234,
			seq:  uint64(i),
			rcv:  100,
		}
		frames := make([]frame, 0)
		data := make([]byte, 100)
		frame1 := frame{
			command: 0,
			size:    100,
			data:    data,
		}
		data2 := make([]byte, i)
		frame2 := frame{
			command: 5,
			size:    uint16(i),
			data:    data2,
		}
		frames = append(frames, frame1)
		frames = append(frames, frame2)
		p.frames = frames

		n, err := conn.Write(p.Pack())
		if err != nil {
			fmt.Printf("client write error: %v", err)
			break
		}
		fmt.Printf("client write packet length [%d]\n", n)
		time.Sleep(time.Millisecond * 20)
	}
	fmt.Println("client exit")
}

func Entrance() {
	wq := &sync.WaitGroup{}
	func() {
		wq.Add(1)
		go server("127.0.0.1", 8796, wq)
	}()
	go client("127.0.0.1", 8796)
	wq.Wait()
}
