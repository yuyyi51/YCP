package packet

import (
	"encoding/binary"
)

func writeBool(b []byte, offset int, v bool) int {
	if v {
		return writeUint8(b, offset, 1)
	}
	return writeUint8(b, offset, 0)
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

func readBool(b []byte, offset int) (bool, int) {
	v := b[offset]
	if v == 0 {
		return false, offset + 1
	}
	return true, offset + 1
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
	ret := make([]byte, len)
	copy(ret, b[offset:offset+len])
	return ret, offset + len
}
