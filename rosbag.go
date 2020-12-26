package rosbag

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
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

type Compression string

const (
	CompressionNone Compression = "none"
	CompressionBZ2  Compression = "bz2"
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
	String() string
	unmarshall() error
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

func (record *RecordBase) String() string {
	return fmt.Sprintf(`
header_len : %d bytes
data_len   : %d bytes
`, len(record.header), len(record.data))
}

type RecordBagHeader struct {
	*RecordBase
	IndexPos   uint64
	ConnCount  uint32
	ChunkCount uint32
}

func (record *RecordBagHeader) String() string {
	return fmt.Sprintf(`
index_pos   : %d
conn_count  : %d
chunk_count : %d
`, record.IndexPos, record.ConnCount, record.ChunkCount)
}

func (record *RecordBagHeader) unmarshall() error {
	err := iterateHeaderFields(record.header, func(key, value []byte) bool {
		if bytes.Equal(key, []byte("index_pos")) {
			record.IndexPos = endian.Uint64(value)
		} else if bytes.Equal(key, []byte("conn_count")) {
			record.ConnCount = endian.Uint32(value)
		} else if bytes.Equal(key, []byte("chunk_count")) {
			record.ChunkCount = endian.Uint32(value)
		} else if bytes.Equal(key, []byte("op")) {
			// explicit ignore
		} else {
			log.Printf("unknown %s. Ignoring...", string(key))
		}

		return true
	})

	return err
}

type RecordChunk struct {
	*RecordBase
	Compression Compression
	Size        uint32
}

func (record *RecordChunk) String() string {
	return fmt.Sprintf(`
compression : %s
size        : %d bytes
`, record.Compression, record.Size)
}

func (record *RecordChunk) unmarshall() error {
	err := iterateHeaderFields(record.header, func(key, value []byte) bool {
		if bytes.Equal(key, []byte("compression")) {
			record.Compression = Compression(value)
		} else if bytes.Equal(key, []byte("size")) {
			record.Size = endian.Uint32(value)
		} else if bytes.Equal(key, []byte("op")) {
			// explicit ignore
		} else {
			log.Printf("unknown %s. Ignoring...", string(key))
		}

		return true
	})

	return err
}
