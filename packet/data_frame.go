package packet

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
)

type DataFrame struct {
	BaseFrame
	Offset uint64
	Data   []byte
}

func (f DataFrame) IsRetransmittable() bool {
	return true
}

func (f DataFrame) String() string {
	return fmt.Sprintf("%s | Offset: %d, len: %d,  Data: 0x%s", f.BaseFrame.String(), f.Offset, len(f.Data), hex.EncodeToString(f.Data))
}

func (f DataFrame) Serialize() []byte {
	f.size = uint16(FrameHeaderSize + 8 + len(f.Data))
	buffer := make([]byte, f.size)
	offset := 0
	offset = writeUint16(buffer, offset, f.size)
	offset = writeUint8(buffer, offset, f.command)
	offset = writeUint64(buffer, offset, f.Offset)
	offset = writeBytes(buffer, offset, f.Data)
	return buffer
}

func CreateDataFrame(data []byte, offset uint64) DataFrame {
	f := DataFrame{}
	f.command = DataFrameCommand
	f.Data = data
	f.Offset = offset
	return f
}

func DeserializeDataFrame(base BaseFrame) DataFrame {
	f := DataFrame{}
	f.BaseFrame = base
	f.Offset = binary.BigEndian.Uint64(f.raw[0:8])
	f.Data = f.raw[8:]
	return f
}

func (f DataFrame) Size() int {
	return FrameHeaderSize + 8 + len(f.Data)
}
