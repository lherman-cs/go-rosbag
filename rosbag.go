package rosbag

import (
	"bytes"
	"compress/bzip2"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"time"

	"github.com/pierrec/lz4/v4"
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
	CompressionLZ4  Compression = "lz4"
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
	Compression      Compression
	Size             uint32
	chunkDataDecoder *Decoder
}

func NewRecordChunk(base *RecordBase, conns map[uint32]*RecordConnection) (*RecordChunk, error) {
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

	var uncompressedReader io.Reader
	switch record.Compression {
	case CompressionNone:
		uncompressedReader = record.data
	case CompressionBZ2:
		uncompressedReader = bzip2.NewReader(record.data)
	case CompressionLZ4:
		uncompressedReader = lz4.NewReader(record.data)
	default:
		return nil, errors.New("unsupported compression algorithm. Available algortihms: [none, bz2, lz4]")
	}

	record.chunkDataDecoder = newDecoder(uncompressedReader, false, false, conns)
	return &record, err
}

func (record *RecordChunk) String() string {
	return fmt.Sprintf(`
compression : %s
size        : %d bytes
`, record.Compression, record.Size)
}

func (record *RecordChunk) Next() (Record, error) {
	return record.chunkDataDecoder.Next()
}

type RecordConnection struct {
	*RecordBase
	Conn             uint32
	Topic            string
	connectionHeader *ConnectionHeader
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

	if err != nil {
		return nil, err
	}

	record.connectionHeader, err = record.readConnectionHeader()
	return &record, err
}

// readConnectionHeader reads the underlying data and decode it to ConnectionHeader
func (record *RecordConnection) readConnectionHeader() (*ConnectionHeader, error) {
	header := make([]byte, record.data.N)
	_, err := io.ReadFull(record.data, header)
	if err != nil {
		return nil, err
	}

	var connectionHeader ConnectionHeader
	err = iterateHeaderFields(header, func(key, value []byte) bool {
		if bytes.Equal(key, []byte("topic")) {
			connectionHeader.Topic = string(value)
		} else if bytes.Equal(key, []byte("type")) {
			connectionHeader.Type = string(value)
		} else if bytes.Equal(key, []byte("md5sum")) {
			connectionHeader.MD5Sum = string(value)
		} else if bytes.Equal(key, []byte("message_definition")) {
			err = connectionHeader.MessageDefinition.unmarshall(value)
		}
		return true
	})
	return &connectionHeader, err
}

func (record *RecordConnection) ConnectionHeader() *ConnectionHeader {
	return record.connectionHeader
}

func (record *RecordConnection) String() string {
	return fmt.Sprintf(`
conn  : %d
topic : %s
`, record.Conn, record.Topic)
}

type RecordMessageData struct {
	*RecordBase
	Conn *RecordConnection
	Time time.Time
}

func NewRecordMessageData(base *RecordBase, conns map[uint32]*RecordConnection) (*RecordMessageData, error) {
	record := RecordMessageData{
		RecordBase: base,
	}
	err := iterateHeaderFields(base.header, func(key, value []byte) bool {
		if bytes.Equal(key, []byte("conn")) {
			record.Conn = conns[endian.Uint32(value)]
		} else if bytes.Equal(key, []byte("time")) {
			record.Time = extractTime(value)
		} else if bytes.Equal(key, []byte("op")) {
			// explicit ignore
		} else {
			log.Printf("unknown %s. Ignoring...", string(key))
		}

		return true
	})

	return &record, err
}

func (record *RecordMessageData) UnmarshallTo(v map[string]interface{}) error {
	hdr := record.Conn.connectionHeader
	raw, err := ioutil.ReadAll(record.data)
	if err != nil {
		return err
	}
	return decodeMessageData(&hdr.MessageDefinition, raw, v)
}

func (record *RecordMessageData) String() string {
	return fmt.Sprintf(`
conn : %d
time : %s
`, record.Conn.Conn, record.Time)
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
			record.StartTime = extractTime(value)
		} else if bytes.Equal(key, []byte("end_time")) {
			record.EndTime = extractTime(value)
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
