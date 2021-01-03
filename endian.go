// +build !integration

package rosbag

import (
	"encoding/binary"
	"fmt"
	"unsafe"
)

var (
	hostEndian binary.ByteOrder
	endian     binary.ByteOrder = binary.LittleEndian
)

func init() {
	switch v := *(*uint16)(unsafe.Pointer(&([]byte{0x12, 0x34}[0]))); v {
	case 0x1234:
		hostEndian = binary.BigEndian
	case 0x3412:
		hostEndian = binary.LittleEndian
	default:
		panic(fmt.Sprintf("failed to determine host endianness: %x", v))
	}

	initFieldSliceDecoder(hostEndian == endian)
}
