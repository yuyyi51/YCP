package packet

import (
	"fmt"
)

const (
	FrameHeaderSize       = 3
	DataFrameHeaderSize   = FrameHeaderSize + 8
	AckFrameHeaderSize    = FrameHeaderSize + 1
	MaxDataFrameDataSize  = Ipv4PayloadSize - DataFrameHeaderSize
	MaxAckFrameDataSize   = Ipv4PayloadSize - AckFrameHeaderSize
	MaxAckFrameRangeCount = 100

	DataFrameCommand = 1
	AckFrameCommand  = 2
	RstFrameCommand  = 3
)

type Frame interface {
	String() string
	Serialize() []byte
	Command() uint8
	Size() int
	IsRetransmittable() bool
}

type BaseFrame struct {
	command uint8
	size    uint16
	raw     []byte
}

func (f *BaseFrame) IsRetransmittable() bool {
	panic("implement me")
}

func (f BaseFrame) String() string {
	command := ""
	switch f.command {
	case DataFrameCommand:
		command = fmt.Sprintf("Data(%d)", f.command)
	case AckFrameCommand:
		command = fmt.Sprintf("Ack(%d)", f.command)
	default:
		command = fmt.Sprintf("Unknown(%d)", f.command)
	}
	return fmt.Sprintf("Frame command: %s, size: %d", command, f.size)
}

func (f *BaseFrame) Command() uint8 {
	return f.command
}

func (f *BaseFrame) Serialize() []byte {
	buffer := make([]byte, 3+len(f.raw))
	offset := 0
	offset = writeUint16(buffer, offset, f.size)
	offset = writeUint8(buffer, offset, f.command)
	offset = writeBytes(buffer, offset, f.raw)
	return buffer
}

func (f *BaseFrame) Deserialize() Frame {
	switch f.command {
	case DataFrameCommand:
		return DeserializeDataFrame(*f)
	case AckFrameCommand:
		return DeserializeAckFrame(*f)
	}
	return nil
}

func (f *BaseFrame) Size() int {
	panic("do not call from BaseFrame")
}
