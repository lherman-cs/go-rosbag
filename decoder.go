package rosbag

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
)

const (
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
	scanner        *bufio.Scanner
	checkedVersion bool
	err            error
	splitFunc      bufio.SplitFunc
}

func NewDecoder(r io.Reader) *Decoder {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(nil, maxTokenSize)

	decoder := Decoder{scanner: scanner}
	scanner.Split(decoder.split)
	return &decoder
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

	record, err := decoder.decodeRecord()
	if err != nil {
		decoder.err = err
		return nil, err
	}
	return record, nil
}

func (decoder *Decoder) split(data []byte, atEOF bool) (advance int, token []byte, err error) {
	return decoder.splitFunc(data, atEOF)
}

func (decoder *Decoder) checkVersion() error {
	var version Version

	decoder.splitFunc = scanStrictLines
	found := decoder.scanner.Scan()
	if !found {
		err := decoder.scanner.Err()
		if err == nil {
			err = errors.New("failed to find version new line character delimiter")
		}
		return err
	}

	versionLine := decoder.scanner.Text()
	_, err := fmt.Sscanf(versionLine, versionFormat, &version.Major, &version.Minor)
	if err != nil {
		return err
	}

	if version.Major != supportedVersion.Major || version.Minor != supportedVersion.Minor {
		return fmt.Errorf("%s is not supported. %s is the current supported version", &version, &supportedVersion)
	}

	return nil
}

func (decoder *Decoder) decodeRecord() (Record, error) {
	var recordBase *RecordBase
	decoder.splitFunc = newScanRecords(func(record *RecordBase) {
		recordBase = record
	})

	found := decoder.scanner.Scan()
	if !found {
		err := decoder.scanner.Err()
		if err == nil {
			err = io.EOF
		}

		return nil, err
	}

	op := OpInvalid
	var err error
	iterateHeaderFields(recordBase.header, func(key, value []byte) bool {
		if bytes.Equal(key, opFieldKey) {
			if len(value) == 0 {
				err = errors.New("empty header field op value")
			} else {
				op = Op(value[0])
			}

			return false
		}

		return true
	})

	if op == OpInvalid {
		if err != nil {
			return nil, err
		}

		return nil, errors.New("required op field doesn't exist in the header fields")
	}

	var record Record
	switch op {
	case OpBagHeader:
		record = &RecordBagHeader{RecordBase: recordBase}
	case OpChunk:
		record = &RecordChunk{RecordBase: recordBase}
	default:
		// return nil, fmt.Errorf("%v is unsupported op code", op)
		return nil, nil
	}

	err = record.unmarshall()
	return record, err
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

// scanStrictLines is similar to bufio.ScanLines but it's more strict, it requires a new line
// character to always exist at the end. And, it doesn't accept CR, only '\n' character.
func scanStrictLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		// We have a full newline-terminated line.
		return i + 1, data[:i], nil
	}
	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), nil, nil
	}
	// Request more data.
	return 0, nil, nil
}

func newScanRecords(cb func(record *RecordBase)) bufio.SplitFunc {
	return bufio.SplitFunc(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		var record RecordBase
		var headerLen, dataLen uint32
		var recordLen int

		defer func() {
			// Since we can't get more data but the data is still not enough to create a record,
			// we need to error out since there's nothing we can do.
			if advance == 0 && atEOF && len(data) > 0 {
				err = errors.New("corrupted record data")
			}
		}()

		if len(data) < headerLenInBytes {
			return
		}
		headerLen = endian.Uint32(data[recordLen : recordLen+headerLenInBytes])
		recordLen += headerLenInBytes

		if uint32(len(data[recordLen:])) < headerLen {
			return
		}
		record.header = data[recordLen : recordLen+int(headerLen)]
		recordLen += int(headerLen)

		if len(data[recordLen:]) < dataLenInBytes {
			return
		}
		dataLen = endian.Uint32(data[recordLen : recordLen+dataLenInBytes])
		recordLen += dataLenInBytes

		if uint32(len(data[recordLen:])) < dataLen {
			return
		}
		record.data = data[recordLen : recordLen+int(dataLen)]
		recordLen += int(dataLen)

		cb(&record)
		advance, token = recordLen, data[:recordLen]
		return
	})
}
