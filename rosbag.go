package rosbag

import (
	"encoding/binary"
	"fmt"
)

const (
	versionFormat = "#ROSBAG V%d.%d"
)

var (
	supportedVersion = Version{
		Major: 2,
		Minor: 0,
	}
	endian = binary.LittleEndian
)

type Op uint8

const (
	// OpInvalid is an extension from the standard. This Op marks an invalid Op.
	OpInvalid     Op = 0x00
	OpBagHeader   Op = 0x03
	OpChunk       Op = 0x05
	OpConnection  Op = 0x07
	OpMessageData Op = 0x02
	OpIndexData   Op = 0x04
	OpChunkInfo   Op = 0x06
)

type Version struct {
	Major uint
	Minor uint
}

func (version *Version) String() string {
	return fmt.Sprintf("%d.%d", version.Major, version.Minor)
}

type Rosbag struct {
	Version Version
	Record  []Record
}

type Record interface {
	Header() []byte
	Data() []byte
}

type RecordBase struct {
	header []byte
	data   []byte
}

func (record *RecordBase) Header() []byte {
	return record.header
}

func (record *RecordBase) Data() []byte {
	return record.data
}
