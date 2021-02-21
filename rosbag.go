// rosbag implements Rosbag Format Version 2.0, http://wiki.ros.org/Bags/Format/2.0.
// Currently, this package only implements the decoder.
package rosbag

import (
	"bytes"
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
	// Op parses the header, and lookup for the op field
	Op() (Op, error)
	// Header returns raw header byte slice. It contains metadata for the record
	Header() []byte
	// Data returns raw data byte slice. It contains record specific data
	Data() []byte
	// Close closes the underlying memory allocation for this record so that
	// the allocated memory can be reused
	Close()
}

type RecordBase struct {
	// Raw contains: <header_len><header><data_len><data>
	Raw []byte
	// HeaderLen contains length in bytes of the header
	HeaderLen uint32
	// DataLen contains length in bytes of the data
	DataLen uint32
	closeFn func()
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

func (record *RecordBase) Close() {
	if record.closeFn != nil {
		record.closeFn()
	}
}

func (record *RecordBase) grow(requiredSize uint32) {
	if uint32(len(record.Raw)) < requiredSize {
		newRaw := make([]byte, 2*requiredSize)
		copy(newRaw, record.Raw)
		record.Raw = newRaw
	}
}

// RecordBagHeader occurs once in the file as the first record.
type RecordBagHeader struct {
	*RecordBase
}

// IndexPos parses Header to get an offset of first record after the chunk section
func (record *RecordBagHeader) IndexPos() (uint64, error) {
	return record.findFieldUint64([]byte("index_pos"))
}

// ConnCount parses Header to get a number of unique connections in the file
func (record *RecordBagHeader) ConnCount() (uint32, error) {
	return record.findFieldUint32([]byte("conn_count"))
}

// ChunkCount parses Header to get a number of chunk records in the file
func (record *RecordBagHeader) ChunkCount() (uint32, error) {
	return record.findFieldUint32([]byte("chunk_count"))
}

// RecordChunk is a record that contains one or more RecordConnection and/or RecordMessageData.
type RecordChunk struct {
	*RecordBase
}

// Compression parses Header to get the compression algorithm that's used for the underlying chunk data.
// The supported compression values are "none", "lz4", and "bz2".
func (record *RecordChunk) Compression() (Compression, error) {
	value, err := record.findField([]byte("compression"))
	if err != nil {
		return CompressionNone, err
	}
	return Compression(value), nil
}

// Size parses Header to get the size in bytes of the uncompressed chunk
func (record *RecordChunk) Size() (uint32, error) {
	return record.findFieldUint32([]byte("size"))
}

// RecordConnection is a record that contains metadata about message data. Some of the metadata are used
// to encode/decode message data.
type RecordConnection struct {
	*RecordBase
}

// Conn parses Header to get the unique connection ID within a bag
func (record *RecordConnection) Conn() (uint32, error) {
	return record.findFieldUint32([]byte("conn"))
}

// Topic parses Header to get the topic
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

// RecordMessageData contains the serialized message data in the ROS serialization format.
type RecordMessageData struct {
	*RecordBase
	connHdr *ConnectionHeader
}

// Conn parses Header to get the unique connection ID within a bag
func (record *RecordMessageData) Conn() (uint32, error) {
	return record.findFieldUint32([]byte("conn"))
}

// Time parses Header to get the timestamp when this message was recorded.
// Note that this timestamp is different from the message header timestamp from ROS. The
// main difference is that this timestamp is the retrieved time NOT sent time.
func (record *RecordMessageData) Time() (time.Time, error) {
	return record.findFieldTime([]byte("time"))
}

// ConnectionHeader returns the parsed ROS connection header.
func (record *RecordMessageData) ConnectionHeader() *ConnectionHeader {
	return record.connHdr
}

// ViewAs views the underlying raw data in the given v format. When possible, View
// will convert raw data without making a copy. With no copy, decoding large arrays become really
// fast! But, this also means that any data types that are reference based can't be used after this
// Record is closed.
//
// So, if the data is absolutely needed after reading this record, you MUST NOT CLOSE this record
// so that the underlying raw data is not overwritten by other records.
func (record *RecordMessageData) ViewAs(v interface{}) error {
	_, err := decodeMessageData(&record.connHdr.MessageDefinition, record.Data(), v)
	if err != nil {
		return err
	}

	return nil
}

type RecordIndexData struct {
	*RecordBase
}

// Conn parses Header to get the unique connection ID within a bag
func (record *RecordIndexData) Conn() (uint32, error) {
	return record.findFieldUint32([]byte("conn"))
}

// Ver parses Header to get the version of this Index record
func (record *RecordIndexData) Ver() (uint32, error) {
	return record.findFieldUint32([]byte("ver"))
}

// Count parses Header to get the number of messages on conn in the preceding chunk
func (record *RecordIndexData) Count() (uint32, error) {
	return record.findFieldUint32([]byte("count"))
}

// RecordChunkInfo contains metadata about Chunks
type RecordChunkInfo struct {
	*RecordBase
}

// Ver parses Header to get the version of this ChunkInfo record
func (record *RecordChunkInfo) Ver() (uint32, error) {
	return record.findFieldUint32([]byte("ver"))
}

// ChunkPos parses Header to get the offset of the chunk record
func (record *RecordChunkInfo) ChunkPos() (uint64, error) {
	return record.findFieldUint64([]byte("chunk_pos"))
}

// StartTime parses Header to get the timestamp of earliest message in the chunk
func (record *RecordChunkInfo) StartTime() (time.Time, error) {
	return record.findFieldTime([]byte("start_time"))
}

// EndTime parses Header to get the timestamp of latest message in the chunk
func (record *RecordChunkInfo) EndTime() (time.Time, error) {
	return record.findFieldTime([]byte("end_time"))
}

// Count parses Header to get the number of connections in the chunk
func (record *RecordChunkInfo) Count() (uint32, error) {
	return record.findFieldUint32([]byte("count"))
}
