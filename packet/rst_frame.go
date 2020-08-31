package packet

import (
	"fmt"
)

type RstFrame struct {
	BaseFrame
}

func (f *RstFrame) IsRetransmittable() bool {
	return true
}

func (f *RstFrame) String() string {
	return fmt.Sprintf("%s", f.BaseFrame.String())
}

func (f *RstFrame) Serialize() []byte {
	f.size = uint16(FrameHeaderSize)
	buffer := make([]byte, f.size)
	offset := 0
	offset = writeUint16(buffer, offset, f.size)
	offset = writeUint8(buffer, offset, f.command)
	return buffer
}

func CreateRstFrame() *RstFrame {
	return &RstFrame{
		BaseFrame: BaseFrame{
			command: RstFrameCommand,
		}}
}

func DeserializeRstFrame(base BaseFrame) *RstFrame {
	f := &RstFrame{}
	f.BaseFrame = base
	return f
}

func (f *RstFrame) Size() int {
	return FrameHeaderSize
}
