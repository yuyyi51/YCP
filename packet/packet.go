package packet

import (
	"bytes"
	"fmt"
)

type Packet struct {
	Conv   uint32
	Seq    uint64
	Rcv    uint64
	Frames []Frame
	Round  uint64
}

func (p Packet) String() string {
	buffer := bytes.Buffer{}
	buffer.WriteString(fmt.Sprintf("Packet Conv: %d, Seq: %d, Rcv: %d, Size: %d\n", p.Conv, p.Seq, p.Rcv, p.Size()))
	for _, frame := range p.Frames {
		buffer.WriteString(fmt.Sprintf("--%s\n", frame.String()))
	}
	return buffer.String()
}

func (p Packet) Pack() []byte {
	result := make([]byte, MTU)
	offset := 0
	offset = writeUint32(result, offset, p.Conv)
	offset = writeUint64(result, offset, p.Seq)
	offset = writeUint64(result, offset, p.Rcv)
	for _, frame := range p.Frames {
		offset = writeBytes(result, offset, frame.Serialize())
	}
	return result[:offset]
}

func Unpack(b []byte) (*Packet, error) {
	offset := 0
	conv, offset := readUint32(b, offset)
	seq, offset := readUint64(b, offset)
	rcv, offset := readUint64(b, offset)
	frames := make([]Frame, 0, 10)
	for offset < len(b) {
		var command uint8
		var size uint16
		var data []byte
		size, offset = readUint16(b, offset)
		command, offset = readUint8(b, offset)
		data, offset = readBytes(b, offset, int(size-FrameHeaderSize))
		baseFrame := BaseFrame{
			command: command,
			size:    size,
			raw:     data,
		}
		frame := baseFrame.Deserialize()
		frames = append(frames, frame)
	}
	p := &Packet{
		Conv:   conv,
		Seq:    seq,
		Rcv:    rcv,
		Frames: frames,
	}
	return p, nil
}

func NewPacket(conv uint32, seq, rcv uint64) Packet {
	return Packet{
		Conv: conv,
		Seq:  seq,
		Rcv:  rcv,
	}
}

func (p *Packet) AddFrame(frame Frame) {
	p.Frames = append(p.Frames, frame)
}

func (p *Packet) Size() int {
	size := PacketHeader
	for _, frame := range p.Frames {
		size += frame.Size()
	}
	return size
}

func (p *Packet) IsRetransmittable() bool {
	retransmittable := false
	for _, frame := range p.Frames {
		retransmittable = frame.IsRetransmittable() || retransmittable
	}
	return retransmittable
}
