package rosbag

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	headerLenInBytes = 4
	dataLenInBytes   = 4
)

type Decoder struct {
	scanner        *bufio.Scanner
	checkedVersion bool
	err            error
	splitFunc      bufio.SplitFunc
}

func NewDecoder(r io.Reader) *Decoder {
	scanner := bufio.NewScanner(r)
	decoder := Decoder{scanner: scanner}
	scanner.Split(decoder.split)
	return &decoder
}

// Next returns the next record in the rosbag. Next might will return nil record and error
// at the beginning to mark that the rosbag format version is supported. When, it reaches EOF,
// Next returns io.EOF error.
func (decoder *Decoder) Next() (*Record, error) {
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
	return nil, nil
}

func (decoder *Decoder) split(data []byte, atEOF bool) (advance int, token []byte, err error) {
	return decoder.splitFunc(data, atEOF)
}

func (decoder *Decoder) checkVersion() error {
	var version Version

	decoder.scanner.Split(scanStrictLines)
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
			if advance == 0 && atEOF {
				// Since we can't get more data but the data is still not enough to create a record,
				// we need to error out since there's nothing we can do.
				err = errors.New("corrupted record data")
			}
		}()

		if len(data) < headerLenInBytes {
			return 0, nil, nil
		}
		headerLen = binary.LittleEndian.Uint32(data[recordLen : recordLen+headerLenInBytes])
		recordLen += headerLenInBytes

		if uint32(len(data[recordLen:])) < headerLen {
			return 0, nil, nil
		}
		record.Header = data[recordLen : recordLen+int(headerLen)]
		recordLen += int(headerLen)

		if len(data[recordLen:]) < dataLenInBytes {
			return 0, nil, nil
		}
		dataLen = binary.LittleEndian.Uint32(data[recordLen : recordLen+dataLenInBytes])
		recordLen += dataLenInBytes

		if uint32(len(data[recordLen:])) < dataLen {
			return 0, nil, nil
		}
		record.Data = data[recordLen : recordLen+int(dataLen)]
		recordLen += int(dataLen)

		cb(&record)
		return recordLen, data[:recordLen], nil
	})
}
