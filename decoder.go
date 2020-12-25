package rosbag

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
)

type Decoder struct {
	reader  io.Reader
	scanner *bufio.Scanner
}

func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{
		reader:  r,
		scanner: bufio.NewScanner(r),
	}
}

func (decoder *Decoder) Decode(rosbag *Rosbag) error {
	var err error
	if err = decoder.decodeVersion(rosbag); err != nil {
		return err
	}
	return nil
}

func (decoder *Decoder) decodeVersion(rosbag *Rosbag) error {
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
	_, err := fmt.Sscanf(versionLine, versionFormat, &rosbag.Version.Major, &rosbag.Version.Minor)
	if err != nil {
		return err
	}

	if rosbag.Version.Major != supportedVersion.Major || rosbag.Version.Minor != supportedVersion.Minor {
		return fmt.Errorf("%s is not supported. %s is the current supported version", &rosbag.Version, &supportedVersion)
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
