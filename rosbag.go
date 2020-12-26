package rosbag

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"time"
)

const (
	versionFormat = "#ROSBAG V%d.%d\n"
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
	Data() *io.LimitedReader
	String() string
}

type RecordBase struct {
	header []byte
	data   *io.LimitedReader
}

func (record *RecordBase) Header() []byte {
	return record.header
}

func (record *RecordBase) Data() *io.LimitedReader {
	return record.data
}

func (record *RecordBase) String() string {
	return fmt.Sprintf(`
header_len : %d bytes
`, len(record.header))
}

type RecordBagHeader struct {
	*RecordBase
	IndexPos   uint64
	ConnCount  uint32
	ChunkCount uint32
}

func NewRecordBagHeader(base *RecordBase) (*RecordBagHeader, error) {
	record := RecordBagHeader{
		RecordBase: base,
	}
	err := iterateHeaderFields(base.header, func(key, value []byte) bool {
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

	return &record, err
}

func (record *RecordBagHeader) String() string {
	return fmt.Sprintf(`
index_pos   : %d
conn_count  : %d
chunk_count : %d
`, record.IndexPos, record.ConnCount, record.ChunkCount)
}

type RecordChunk struct {
	*RecordBase
	Compression Compression
	Size        uint32
}

func NewRecordChunk(base *RecordBase) (*RecordChunk, error) {
	record := RecordChunk{
		RecordBase: base,
	}
	err := iterateHeaderFields(base.header, func(key, value []byte) bool {
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

	return &record, err
}

func (record *RecordChunk) String() string {
	return fmt.Sprintf(`
compression : %s
size        : %d bytes
`, record.Compression, record.Size)
}

type RecordConnection struct {
	*RecordBase
	Conn  uint32
	Topic string
}

func NewRecordConnection(base *RecordBase) (*RecordConnection, error) {
	record := RecordConnection{
		RecordBase: base,
	}
	err := iterateHeaderFields(base.header, func(key, value []byte) bool {
		if bytes.Equal(key, []byte("conn")) {
			record.Conn = endian.Uint32(value)
		} else if bytes.Equal(key, []byte("topic")) {
			record.Topic = string(value)
		} else if bytes.Equal(key, []byte("op")) {
			// explicit ignore
		} else {
			log.Printf("unknown %s. Ignoring...", string(key))
		}

		return true
	})

	return &record, err
}

func (record *RecordConnection) String() string {
	return fmt.Sprintf(`
conn  : %d
topic : %s
`, record.Conn, record.Topic)
}

type RecordMessageData struct {
	*RecordBase
	Conn uint32
	Time time.Time
}

func NewRecordMessageData(base *RecordBase) (*RecordMessageData, error) {
	record := RecordMessageData{
		RecordBase: base,
	}
	err := iterateHeaderFields(base.header, func(key, value []byte) bool {
		if bytes.Equal(key, []byte("conn")) {
			record.Conn = endian.Uint32(value)
		} else if bytes.Equal(key, []byte("time")) {
			record.Time = nanoToTime(endian.Uint64(value))
		} else if bytes.Equal(key, []byte("op")) {
			// explicit ignore
		} else {
			log.Printf("unknown %s. Ignoring...", string(key))
		}

		return true
	})

	return &record, err
}

func (record *RecordMessageData) String() string {
	return fmt.Sprintf(`
conn : %d
time : %s
`, record.Conn, record.Time)
}

type RecordIndexData struct {
	*RecordBase
	Ver   uint32
	Conn  uint32
	Count uint32
}

func NewRecordIndexData(base *RecordBase) (*RecordIndexData, error) {
	record := RecordIndexData{
		RecordBase: base,
	}
	err := iterateHeaderFields(base.header, func(key, value []byte) bool {
		if bytes.Equal(key, []byte("ver")) {
			record.Ver = endian.Uint32(value)
		} else if bytes.Equal(key, []byte("conn")) {
			record.Conn = endian.Uint32(value)
		} else if bytes.Equal(key, []byte("count")) {
			record.Count = endian.Uint32(value)
		} else if bytes.Equal(key, []byte("op")) {
			// explicit ignore
		} else {
			log.Printf("unknown %s. Ignoring...", string(key))
		}

		return true
	})

	return &record, err
}

func (record *RecordIndexData) String() string {
	return fmt.Sprintf(`
ver   : %d
conn  : %d
count : %d
`, record.Ver, record.Conn, record.Count)
}

type RecordChunkInfo struct {
	*RecordBase
	Ver       uint32
	ChunkPos  uint64
	StartTime time.Time
	EndTime   time.Time
	Count     uint32
}

func NewRecordChunkInfo(base *RecordBase) (*RecordChunkInfo, error) {
	record := RecordChunkInfo{
		RecordBase: base,
	}
	err := iterateHeaderFields(base.header, func(key, value []byte) bool {
		if bytes.Equal(key, []byte("ver")) {
			record.Ver = endian.Uint32(value)
		} else if bytes.Equal(key, []byte("chunk_pos")) {
			record.ChunkPos = endian.Uint64(value)
		} else if bytes.Equal(key, []byte("start_time")) {
			record.StartTime = nanoToTime(endian.Uint64(value))
		} else if bytes.Equal(key, []byte("end_time")) {
			record.EndTime = nanoToTime(endian.Uint64(value))
		} else if bytes.Equal(key, []byte("count")) {
			record.Count = endian.Uint32(value)
		} else if bytes.Equal(key, []byte("op")) {
			// explicit ignore
		} else {
			log.Printf("unknown %s. Ignoring...", string(key))
		}

		return true
	})

	return &record, err
}

func (record *RecordChunkInfo) String() string {
	return fmt.Sprintf(`
ver        : %d
chunk_pos  : %d
start_time : %s
end_time   : %s
count      : %d
`, record.Ver, record.ChunkPos, record.StartTime, record.EndTime, record.Count)
}
