package packet

import (
	"bytes"
	"fmt"
)

type AckFrame struct {
	BaseFrame
	ackRanges  []AckRange
	rangeCount uint8
}

func (f AckFrame) String() string {
	buffer := bytes.Buffer{}
	for _, ran := range f.ackRanges {
		buffer.WriteString(fmt.Sprintf("%s ", ran.String()))
	}
	return buffer.String()
}

func (f AckFrame) Serialize() []byte {
	f.size = uint16(FrameHeaderSize + 1 + 16*uint16(f.rangeCount))
	buffer := make([]byte, f.size)
	offset := 0
	offset = writeUint16(buffer, offset, f.size)
	offset = writeUint8(buffer, offset, f.command)
	offset = writeUint8(buffer, offset, f.rangeCount)
	for _, ran := range f.ackRanges {
		offset = writeUint64(buffer, offset, ran.Left)
		offset = writeUint64(buffer, offset, ran.Right)
	}
	return buffer
}

func (f AckFrame) Size() int {
	return FrameHeaderSize + 1 + 16*int(f.rangeCount)
}

func (f AckFrame) IsRetransmittable() bool {
	return false
}

type AckRange struct {
	Left  uint64
	Right uint64
}

func (ran AckRange) String() string {
	return fmt.Sprintf("[%d,%d]", ran.Left, ran.Right)
}

func CreateAckFrame(ranges []AckRange) AckFrame {
	f := AckFrame{}
	f.command = AckFrameCommand
	f.ackRanges = ranges
	f.rangeCount = uint8(len(ranges))
	return f
}
