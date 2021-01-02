package rosbag

import (
	"bufio"
	"compress/bzip2"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/pierrec/lz4/v4"
)

const (
	lenInBytes           = 4
	headerFieldDelimiter = '='
	initialRecordSize    = 4096
)

var (
	errUnsupportedCompression = errors.New("unsupported compression algorithm. Available algortihms: [none, bz2, lz4]")
)

var (
	recordPool = sync.Pool{
		New: func() interface{} {
			return &RecordBase{
				Raw: make([]byte, initialRecordSize),
			}
		},
	}
)

type Decoder struct {
	reader         io.Reader
	chunkReader    io.Reader
	checkedVersion bool
	conns          map[uint32]*ConnectionHeader
}

func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{
		reader: bufio.NewReader(r),
		conns:  make(map[uint32]*ConnectionHeader),
	}
}

// Read returns the next record in the rosbag. Next might will return nil record and error
// at the beginning to mark that the rosbag format version is supported. When, it reaches EOF,
// Next returns io.EOF error.
func (decoder *Decoder) Read() (Record, error) {
	if !decoder.checkedVersion {
		if err := decoder.checkVersion(); err != nil {
			return nil, err
		}

		decoder.checkedVersion = true
	}

	record := recordPool.Get().(*RecordBase)
	record.closeFn = func() {
		recordPool.Put(record)
	}
	if decoder.chunkReader != nil {
		specializedRecord, err := decoder.decodeRecord(decoder.chunkReader, record)
		switch err {
		case nil:
			return specializedRecord, nil
		case io.EOF:
			/* explicit ignore */
		default:
			// the record is not usable, so recyle it
			record.Close()
			return nil, err
		}

		// at this point, the error must be EOF, need to reset chunkReader and read from the source
		// again
		decoder.chunkReader = nil
	}

	specializedRecord, err := decoder.decodeRecord(decoder.reader, record)
	if err != nil {
		// the record is not usable, so recyle it
		record.Close()
		return nil, err
	}

	return specializedRecord, nil
}

func (decoder *Decoder) handleChunk(record *RecordBase) (Record, error) {
	chunkRecord := RecordChunk{
		RecordBase: record,
	}

	compression, err := chunkRecord.Compression()
	if err != nil {
		return nil, err
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
		return nil, errUnsupportedCompression
	}

	return &chunkRecord, nil
}

func (decoder *Decoder) handleConnection(record *RecordBase) (Record, error) {
	connRecord := RecordConnection{
		RecordBase: record,
	}

	conn, err := connRecord.Conn()
	if err != nil {
		return nil, err
	}

	hdr, err := connRecord.ConnectionHeader()
	if err != nil {
		return nil, err
	}

	decoder.conns[conn] = hdr
	return &connRecord, nil
}

func (decoder *Decoder) handleMessageData(record *RecordBase) (Record, error) {
	connRecord := RecordMessageData{
		RecordBase: record,
	}

	conn, err := connRecord.Conn()
	if err != nil {
		return nil, err
	}

	connHdr, ok := decoder.conns[conn]
	if !ok {
		return nil, errNotFoundConnectionHeader
	}

	connRecord.connHdr = connHdr
	return &connRecord, nil
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

func (decoder *Decoder) decodeRecord(r io.Reader, record *RecordBase) (Record, error) {
	var off uint32
	var err error

	record.grow(off + lenInBytes)
	_, err = io.ReadFull(r, record.Raw[off:off+lenInBytes])
	if err != nil {
		return nil, err
	}
	record.HeaderLen = endian.Uint32(record.Raw[off : off+lenInBytes])
	off += lenInBytes

	record.grow(off + record.HeaderLen)
	_, err = io.ReadFull(r, record.Raw[off:off+record.HeaderLen])
	if err != nil {
		return nil, err
	}
	off += record.HeaderLen

	op, err := record.Op()
	if err != nil {
		return nil, err
	}

	record.grow(off + lenInBytes)
	_, err = io.ReadFull(r, record.Raw[off:off+lenInBytes])
	if err != nil {
		return nil, err
	}
	record.DataLen = endian.Uint32(record.Raw[off : off+lenInBytes])
	off += lenInBytes

	// Since RecordChunk contains a lot of messages and connections, we don't parse
	// the data part. We'll let the next iteration to parse this.
	if op == OpChunk {
		return decoder.handleChunk(record)
	}

	record.grow(off + record.DataLen)
	_, err = io.ReadFull(r, record.Raw[off:off+record.DataLen])
	if err != nil {
		return nil, err
	}

	switch op {
	case OpBagHeader:
		return &RecordBagHeader{RecordBase: record}, nil
	case OpConnection:
		return decoder.handleConnection(record)
	case OpMessageData:
		return decoder.handleMessageData(record)
	case OpIndexData:
		return &RecordIndexData{RecordBase: record}, nil
	case OpChunkInfo:
		return &RecordChunkInfo{RecordBase: record}, nil
	default:
		return nil, errInvalidOp
	}
}
