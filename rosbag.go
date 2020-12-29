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

type Record struct {
	// Raw contains: <header_len><header><data_len><data>
	Raw                []byte
	HeaderLen, DataLen uint32
	conns              map[uint32]*ConnectionHeader
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

func (record *Record) findField(key []byte) ([]byte, error) {
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

func (record *Record) findFieldUint32(key []byte) (uint32, error) {
	value, err := record.findField(key)
	if err != nil {
		return 0, err
	}
	return endian.Uint32(value), nil
}

func (record *Record) findFieldUint64(key []byte) (uint64, error) {
	value, err := record.findField(key)
	if err != nil {
		return 0, err
	}
	return endian.Uint64(value), nil
}

func (record *Record) findFieldTime(key []byte) (time.Time, error) {
	value, err := record.findField(key)
	if err != nil {
		return time.Time{}, err
	}
	return extractTime(value), nil
}

func (record *Record) Op() (Op, error) {
	value, err := record.findField([]byte("op"))
	if err != nil {
		return OpInvalid, err
	}
	return Op(value[0]), nil
}

func (record *Record) Header() []byte {
	return record.Raw[lenInBytes : lenInBytes+record.HeaderLen]
}

func (record *Record) Data() []byte {
	off := 2*lenInBytes + record.HeaderLen
	return record.Raw[off : off+record.DataLen]
}

func (record *Record) grow(requiredSize uint32) {
	if uint32(len(record.Raw)) < requiredSize {
		newRaw := make([]byte, 2*requiredSize)
		copy(newRaw, record.Raw)
		record.Raw = newRaw
	}
}

func (record *Record) validateOp(to Op) *Record {
	op, err := record.Op()
	if err != nil {
		panic(err)
	}

	if op != to {
		panic(errInvalidOp)
	}
	return record
}

type RecordBagHeader interface {
	IndexPos() (uint64, error)
	ConnCount() (uint32, error)
	ChunkCount() (uint32, error)
}

func (record *Record) BagHeader() RecordBagHeader {
	return record.validateOp(OpBagHeader)
}

func (record *Record) IndexPos() (uint64, error) {
	return record.findFieldUint64([]byte("index_pos"))
}

func (record *Record) ConnCount() (uint32, error) {
	return record.findFieldUint32([]byte("conn_count"))
}

func (record *Record) ChunkCount() (uint32, error) {
	return record.findFieldUint32([]byte("chunk_count"))
}

type RecordChunk interface {
	Compression() (Compression, error)
	Size() (uint32, error)
}

func (record *Record) Chunk() RecordChunk {
	return record.validateOp(OpChunk)
}

func (record *Record) Compression() (Compression, error) {
	value, err := record.findField([]byte("compression"))
	if err != nil {
		return CompressionNone, err
	}
	return Compression(value), nil
}

func (record *Record) Size() (uint32, error) {
	return record.findFieldUint32([]byte("size"))
}

type RecordConnection interface {
	Conn() (uint32, error)
	Topic() (string, error)
	ConnectionHeader() (*ConnectionHeader, error)
}

func (record *Record) Connection() RecordConnection {
	return record.validateOp(OpConnection)
}

func (record *Record) Conn() (uint32, error) {
	return record.findFieldUint32([]byte("conn"))
}

func (record *Record) Topic() (string, error) {
	value, err := record.findField([]byte("topic"))
	if err != nil {
		return "", err
	}
	return string(value), nil
}

// ConnectionHeader reads the underlying data and decode it to ConnectionHeader
func (record *Record) ConnectionHeader() (*ConnectionHeader, error) {
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

type RecordMessageData interface {
	Conn() (uint32, error)
	Time() (time.Time, error)
	UnmarshallTo(map[string]interface{}) error
}

func (record *Record) MessageData() RecordMessageData {
	return record.validateOp(OpMessageData)
}

func (record *Record) Time() (time.Time, error) {
	return record.findFieldTime([]byte("time"))
}

func (record *Record) UnmarshallTo(v map[string]interface{}) error {
	conn, err := record.Conn()
	if err != nil {
		return err
	}

	hdr, ok := record.conns[conn]
	if !ok {
		return errNotFoundConnectionHeader
	}

	return decodeMessageData(&hdr.MessageDefinition, record.Data(), v)
}

type RecordIndexData interface {
	Ver() (uint32, error)
	Conn() (uint32, error)
	Count() (uint32, error)
}

func (record *Record) IndexData() RecordIndexData {
	return record.validateOp(OpIndexData)
}

func (record *Record) Ver() (uint32, error) {
	return record.findFieldUint32([]byte("ver"))
}

func (record *Record) Count() (uint32, error) {
	return record.findFieldUint32([]byte("count"))
}

type RecordChunkInfo interface {
	Ver() (uint32, error)
	ChunkPos() (uint64, error)
	StartTime() (time.Time, error)
	EndTime() (time.Time, error)
	Count() (uint32, error)
}

func (record *Record) ChunkInfo() RecordChunkInfo {
	return record.validateOp(OpChunkInfo)
}

func (record *Record) ChunkPos() (uint64, error) {
	return record.findFieldUint64([]byte("chunk_pos"))
}

func (record *Record) StartTime() (time.Time, error) {
	return record.findFieldTime([]byte("start_time"))
}

func (record *Record) EndTime() (time.Time, error) {
	return record.findFieldTime([]byte("end_time"))
}
