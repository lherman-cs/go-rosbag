package rosbag

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

const (
	versionFormat = "#ROSBAG V%d.%d\n"
)

var (
	errInvalidOp                = errors.New("invalid op")
	errNotFoundConnectionHeader = errors.New("failed to find connection header")
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
	Op() (Op, error)
	Header() []byte
	Data() []byte
}

type RecordBase struct {
	// Raw contains: <header_len><header><data_len><data>
	Raw                []byte
	HeaderLen, DataLen uint32
}

func iterateHeaderFields(header []byte, cb func(key, value []byte) bool) error {
	for len(header) > 0 {
		if len(header) < lenInBytes {
			return errors.New("missing header field length")
		}

		fieldLen := int(endian.Uint32(header))
		header = header[lenInBytes:]
		if len(header) < fieldLen {
			return fmt.Errorf("expected header field len to be %d, but got %d", fieldLen, len(header))
		}

		i := bytes.IndexByte(header, headerFieldDelimiter)
		if i == -1 {
			return fmt.Errorf("invalid header field format, expected the key and value is separated by a '%c'", headerFieldDelimiter)
		}

		if fieldLen < i+1 {
			return fmt.Errorf("invalid header field len, expected to be at least %d", i+1)
		}

		shouldContinue := cb(header[:i], header[i+1:fieldLen])
		if !shouldContinue {
			break
		}
		header = header[fieldLen:]
	}

	return nil
}

func (record *RecordBase) findField(key []byte) ([]byte, error) {
	var value []byte
	header := record.Header()
	err := iterateHeaderFields(header, func(currentKey, currentValue []byte) bool {
		if bytes.Equal(currentKey, key) {
			value = currentValue
			return false
		}

		return true
	})

	if err != nil {
		return nil, err
	}

	if value == nil {
		return nil, fmt.Errorf("%s field doesn't exist in the header fields", string(key))
	}

	return value, nil
}

func (record *RecordBase) findFieldUint32(key []byte) (uint32, error) {
	value, err := record.findField(key)
	if err != nil {
		return 0, err
	}
	return endian.Uint32(value), nil
}

func (record *RecordBase) findFieldUint64(key []byte) (uint64, error) {
	value, err := record.findField(key)
	if err != nil {
		return 0, err
	}
	return endian.Uint64(value), nil
}

func (record *RecordBase) findFieldTime(key []byte) (time.Time, error) {
	value, err := record.findField(key)
	if err != nil {
		return time.Time{}, err
	}
	return extractTime(value), nil
}

func (record *RecordBase) Op() (Op, error) {
	value, err := record.findField([]byte("op"))
	if err != nil {
		return OpInvalid, err
	}
	return Op(value[0]), nil
}

func (record *RecordBase) Header() []byte {
	return record.Raw[lenInBytes : lenInBytes+record.HeaderLen]
}

func (record *RecordBase) Data() []byte {
	off := 2*lenInBytes + record.HeaderLen
	return record.Raw[off : off+record.DataLen]
}

func (record *RecordBase) grow(requiredSize uint32) {
	if uint32(len(record.Raw)) < requiredSize {
		newRaw := make([]byte, 2*requiredSize)
		copy(newRaw, record.Raw)
		record.Raw = newRaw
	}
}

type RecordBagHeader struct {
	*RecordBase
}

func (record *RecordBagHeader) IndexPos() (uint64, error) {
	return record.findFieldUint64([]byte("index_pos"))
}

func (record *RecordBagHeader) ConnCount() (uint32, error) {
	return record.findFieldUint32([]byte("conn_count"))
}

func (record *RecordBagHeader) ChunkCount() (uint32, error) {
	return record.findFieldUint32([]byte("chunk_count"))
}

type RecordChunk struct {
	*RecordBase
}

func (record *RecordChunk) Compression() (Compression, error) {
	value, err := record.findField([]byte("compression"))
	if err != nil {
		return CompressionNone, err
	}
	return Compression(value), nil
}

func (record *RecordChunk) Size() (uint32, error) {
	return record.findFieldUint32([]byte("size"))
}

type RecordConnection struct {
	*RecordBase
}

func (record *RecordConnection) Conn() (uint32, error) {
	return record.findFieldUint32([]byte("conn"))
}

func (record *RecordConnection) Topic() (string, error) {
	value, err := record.findField([]byte("topic"))
	if err != nil {
		return "", err
	}
	return string(value), nil
}

// ConnectionHeader reads the underlying data and decode it to ConnectionHeader
func (record *RecordConnection) ConnectionHeader() (*ConnectionHeader, error) {
	var err error
	var connectionHeader ConnectionHeader
	err = iterateHeaderFields(record.Data(), func(key, value []byte) bool {
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

type RecordMessageData struct {
	*RecordBase
	connHdr *ConnectionHeader
}

func (record *RecordMessageData) Conn() (uint32, error) {
	return record.findFieldUint32([]byte("conn"))
}

func (record *RecordMessageData) Time() (time.Time, error) {
	return record.findFieldTime([]byte("time"))
}

func (record *RecordMessageData) ConnectionHeader() *ConnectionHeader {
	return record.connHdr
}

func (record *RecordMessageData) UnmarshallTo(v map[string]interface{}) error {
	_, err := decodeMessageData(&record.connHdr.MessageDefinition, record.Data(), v)
	return err
}

type RecordIndexData struct {
	*RecordBase
}

func (record *RecordIndexData) Conn() (uint32, error) {
	return record.findFieldUint32([]byte("conn"))
}

func (record *RecordIndexData) Ver() (uint32, error) {
	return record.findFieldUint32([]byte("ver"))
}

func (record *RecordIndexData) Count() (uint32, error) {
	return record.findFieldUint32([]byte("count"))
}

type RecordChunkInfo struct {
	*RecordBase
}

func (record *RecordChunkInfo) Ver() (uint32, error) {
	return record.findFieldUint32([]byte("ver"))
}

func (record *RecordChunkInfo) ChunkPos() (uint64, error) {
	return record.findFieldUint64([]byte("chunk_pos"))
}

func (record *RecordChunkInfo) StartTime() (time.Time, error) {
	return record.findFieldTime([]byte("start_time"))
}

func (record *RecordChunkInfo) EndTime() (time.Time, error) {
	return record.findFieldTime([]byte("end_time"))
}

func (record *RecordChunkInfo) Count() (uint32, error) {
	return record.findFieldUint32([]byte("count"))
}
