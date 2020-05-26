package packet

import (
	"bytes"
	"fmt"
)

type AckFrame struct {
	BaseFrame
	AckRanges  []AckRange
	RangeCount uint8
}

func (f AckFrame) String() string {
	buffer := bytes.Buffer{}
	for _, ran := range f.AckRanges {
		buffer.WriteString(fmt.Sprintf("%s ", ran.String()))
	}
	return fmt.Sprintf("%s | %s", f.BaseFrame, buffer.String())
}

func (f AckFrame) Serialize() []byte {
	f.size = uint16(FrameHeaderSize + 1 + 16*uint16(f.RangeCount))
	buffer := make([]byte, f.size)
	offset := 0
	offset = writeUint16(buffer, offset, f.size)
	offset = writeUint8(buffer, offset, f.command)
	offset = writeUint8(buffer, offset, f.RangeCount)
	for _, ran := range f.AckRanges {
		offset = writeUint64(buffer, offset, ran.Left)
		offset = writeUint64(buffer, offset, ran.Right)
	}
	return buffer
}

func DeserializeAckFrame(base BaseFrame) AckFrame {
	f := AckFrame{}
	f.BaseFrame = base
	offset := 0
	f.RangeCount, offset = readUint8(base.raw, offset)
	var left, right uint64
	for i := uint8(0); i < f.RangeCount; i++ {
		left, offset = readUint64(base.raw, offset)
		right, offset = readUint64(base.raw, offset)
		f.AckRanges = append(f.AckRanges, AckRange{Left: left, Right: right})
	}
	return f
}

func (f AckFrame) Size() int {
	return FrameHeaderSize + 1 + 16*int(f.RangeCount)
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
	f.AckRanges = ranges
	f.RangeCount = uint8(len(ranges))
	return f
}
