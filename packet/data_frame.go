package packet

import (
	"fmt"
)

type DataFrame struct {
	BaseFrame
	Offset uint64
	Data   []byte
	Fin    bool
}

func (f *DataFrame) IsRetransmittable() bool {
	return true
}

func (f *DataFrame) String() string {
	return fmt.Sprintf("%s | Offset: %d, Len: %d, Fin: %v", f.BaseFrame.String(), f.Offset, len(f.Data), f.Fin)
}

func (f *DataFrame) Serialize() []byte {
	f.size = uint16(FrameHeaderSize + 9 + len(f.Data))
	buffer := make([]byte, f.size)
	offset := 0
	offset = writeUint16(buffer, offset, f.size)
	offset = writeUint8(buffer, offset, f.command)
	offset = writeUint64(buffer, offset, f.Offset)
	offset = writeBool(buffer, offset, f.Fin)
	offset = writeBytes(buffer, offset, f.Data)
	return buffer
}

func CreateDataFrame(data []byte, offset uint64, fin bool) *DataFrame {
	return &DataFrame{
		BaseFrame: BaseFrame{
			command: DataFrameCommand,
		},
		Data:   data,
		Offset: offset,
		Fin:    fin,
	}
}

func DeserializeDataFrame(base BaseFrame) *DataFrame {
	f := &DataFrame{}
	f.BaseFrame = base
	offset := 0
	f.Offset, offset = readUint64(f.raw, offset)
	f.Fin, offset = readBool(f.raw, offset)
	f.Data = f.raw[offset:]
	return f
}

func (f *DataFrame) Size() int {
	return FrameHeaderSize + 9 + len(f.Data)
}
