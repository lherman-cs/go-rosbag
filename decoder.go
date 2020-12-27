package rosbag

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
)

const (
	lenInBytes            = 4
	headerLenInBytes      = 4
	dataLenInBytes        = 4
	headerFieldLenInBytes = 4
	headerFieldDelimiter  = '='
	maxTokenSize          = 8589934598
)

var (
	opFieldKey = []byte("op")
)

type Decoder struct {
	reader         io.Reader
	checkedVersion bool
	err            error
	lastRecord     Record
}

func newDecoder(r io.Reader, hasVersionLine bool, withBuffer bool) *Decoder {
	if withBuffer {
		r = bufio.NewReader(r)
	}
	decoder := Decoder{reader: r, checkedVersion: !hasVersionLine}
	return &decoder
}

func NewDecoder(r io.Reader) *Decoder {
	return newDecoder(r, true, true)
}

// Next returns the next record in the rosbag. Next might will return nil record and error
// at the beginning to mark that the rosbag format version is supported. When, it reaches EOF,
// Next returns io.EOF error.
func (decoder *Decoder) Next() (Record, error) {
	var err error

	if decoder.err != nil {
		return nil, decoder.err
	}

	if !decoder.checkedVersion {
		if err = decoder.checkVersion(); err != nil {
			decoder.err = err
			return nil, err
		}

		decoder.checkedVersion = true
	}

	if decoder.lastRecord != nil {
		// We need to drain the reader until the end of the last record. Otherwise, the next record will
		// start reading from anywhere in the last record.
		lastRecordData := decoder.lastRecord.Data()
		_, err := io.CopyN(ioutil.Discard, lastRecordData, lastRecordData.N)
		if err != nil {
			return nil, err
		}
	}

	record, err := decodeRecord(decoder.reader)
	if err != nil {
		decoder.err = err
		return nil, err
	}

	decoder.lastRecord = record
	return record, nil
}

func (decoder *Decoder) checkVersion() error {
	var version Version

	_, err := fmt.Fscanf(decoder.reader, versionFormat, &version.Major, &version.Minor)
	if err != nil {
		return err
	}

	if version.Major != supportedVersion.Major || version.Minor != supportedVersion.Minor {
		return fmt.Errorf("%s is not supported. %s is the current supported version", &version, &supportedVersion)
	}

	return nil
}

func decodeRecord(r io.Reader) (Record, error) {
	header, err := decodeHeader(r)
	if err != nil {
		return nil, err
	}

	opFieldValue, err := findField(header, opFieldKey)
	if err != nil {
		return nil, err
	}

	if len(opFieldValue) == 0 {
		return nil, errors.New("empty header field op value")
	}

	lenBuf := make([]byte, lenInBytes)
	_, err = io.ReadFull(r, lenBuf)
	if err != nil {
		return nil, err
	}
	dataLen := endian.Uint32(lenBuf)
	dataReader := io.LimitedReader{
		N: int64(dataLen),
		R: r,
	}

	base := RecordBase{
		header: header,
		data:   &dataReader,
	}

	op := Op(opFieldValue[0])
	switch op {
	case OpBagHeader:
		return NewRecordBagHeader(&base)
	case OpChunk:
		return NewRecordChunk(&base)
	case OpConnection:
		return NewRecordConnection(&base)
	case OpMessageData:
		return NewRecordMessageData(&base)
	case OpIndexData:
		return NewRecordIndexData(&base)
	case OpChunkInfo:
		return NewRecordChunkInfo(&base)
	default:
		return nil, errors.New("invalid op value")
	}
}

func decodeHeader(r io.Reader) ([]byte, error) {
	lenBuf := make([]byte, lenInBytes)
	_, err := io.ReadFull(r, lenBuf)
	if err != nil {
		return nil, err
	}

	headerLen := endian.Uint32(lenBuf)
	header := make([]byte, headerLen)
	_, err = io.ReadFull(r, header)
	if err != nil {
		return nil, err
	}

	return header, nil
}

func findField(header []byte, key []byte) (value []byte, err error) {
	err = iterateHeaderFields(header, func(currentKey, currentValue []byte) bool {
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

func iterateHeaderFields(header []byte, cb func(key, value []byte) bool) error {
	for len(header) > 0 {
		if len(header) < headerFieldLenInBytes {
			return errors.New("missing header field length")
		}

		fieldLen := int(endian.Uint32(header))
		header = header[headerFieldLenInBytes:]
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
