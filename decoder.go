package rosbag

import (
	"bufio"
	"compress/bzip2"
	"errors"
	"fmt"
	"io"

	"github.com/pierrec/lz4/v4"
)

const (
	lenInBytes           = 4
	headerFieldDelimiter = '='
)

var (
	errUnsupportedCompression = errors.New("unsupported compression algorithm. Available algortihms: [none, bz2, lz4]")
)

type Decoder struct {
	reader         io.Reader
	chunkReader    io.Reader
	checkedVersion bool
}

func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{
		reader: bufio.NewReader(r),
	}
}

// Read returns the next record in the rosbag. Next might will return nil record and error
// at the beginning to mark that the rosbag format version is supported. When, it reaches EOF,
// Next returns io.EOF error.
func (decoder *Decoder) Read(record *Record) (op Op, err error) {
	if !decoder.checkedVersion {
		if err = decoder.checkVersion(); err != nil {
			return
		}

		decoder.checkedVersion = true
	}

	if decoder.chunkReader != nil {
		op, err = decoder.decodeRecord(decoder.chunkReader, record)
		if err == nil || err != io.EOF {
			return
		}

		// at this point, the error must be EOF, need to reset chunkReader and read from the source
		// again
		decoder.chunkReader = nil
	}

	op, err = decoder.decodeRecord(decoder.reader, record)
	if err != nil {
		return
	}

	return
}

func (decoder *Decoder) handleChunk(record *Record) error {
	compression, err := record.Compression()
	if err != nil {
		return err
	}

	chunkReader := io.LimitReader(decoder.reader, int64(record.DataLen))
	switch compression {
	case CompressionNone:
		decoder.chunkReader = chunkReader
	case CompressionBZ2:
		decoder.chunkReader = bzip2.NewReader(chunkReader)
	case CompressionLZ4:
		decoder.chunkReader = lz4.NewReader(chunkReader)
	default:
		return errUnsupportedCompression
	}

	return nil
}

func (decoder *Decoder) handleConnection(record *Record) error {
	conn, err := record.Conn()
	if err != nil {
		return err
	}

	hdr, err := record.ConnectionHeader()
	if err != nil {
		return err
	}

	if record.Conns == nil {
		record.Conns = make(map[uint32]*ConnectionHeader)
	}

	record.Conns[conn] = hdr
	return nil
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

func (decoder *Decoder) decodeRecord(r io.Reader, record *Record) (op Op, err error) {
	var off uint32

	record.grow(off + lenInBytes)
	_, err = io.ReadFull(r, record.Raw[off:off+lenInBytes])
	if err != nil {
		return
	}
	record.HeaderLen = endian.Uint32(record.Raw[off : off+lenInBytes])
	off += lenInBytes

	record.grow(off + record.HeaderLen)
	_, err = io.ReadFull(r, record.Raw[off:off+record.HeaderLen])
	if err != nil {
		return
	}
	off += record.HeaderLen

	op, err = record.Op()
	if err != nil {
		return
	}

	record.grow(off + lenInBytes)
	_, err = io.ReadFull(r, record.Raw[off:off+lenInBytes])
	if err != nil {
		return
	}
	record.DataLen = endian.Uint32(record.Raw[off : off+lenInBytes])
	off += lenInBytes

	// Since RecordChunk contains a lot of messages and connections, we don't parse
	// the data part. We'll let the next iteration to parse this.
	if op == OpChunk {
		err = decoder.handleChunk(record)
		return
	}

	record.grow(off + record.DataLen)
	_, err = io.ReadFull(r, record.Raw[off:off+record.DataLen])
	if err != nil {
		return
	}

	if op == OpConnection {
		err = decoder.handleConnection(record)
		return
	}

	return
}
