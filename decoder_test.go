package rosbag

import (
	"bytes"
	"io"
	"reflect"
	"testing"
)

func bytesToLimitedReader(b []byte) *io.LimitedReader {
	return &io.LimitedReader{R: bytes.NewReader(b), N: int64(len(b))}
}

func TestDecoderCheckVersion(t *testing.T) {
	testCases := []struct {
		Name string
		Raw  []byte
		Fail bool
	}{
		{
			Name: "Missing Newline character",
			Raw:  []byte("#ROSBAG V2.0"),
			Fail: false,
		},
		{
			Name: "Unsupported Version",
			Raw:  []byte("#ROSBAG V1.2\n"),
			Fail: true,
		},
		{
			Name: "Expected Version Format",
			Raw:  []byte("#ROSBAG V2.0\n"),
			Fail: false,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.Name, func(t *testing.T) {
			in := bytes.NewReader(testCase.Raw)
			err := NewDecoder(in).checkVersion()

			if testCase.Fail && err == nil {
				t.Fatal("expected to fail")
			} else if !testCase.Fail && err != nil {
				t.Fatal("expected to succeed")
			}
		})
	}
}

func TestDecodeRecord(t *testing.T) {
	testCases := []struct {
		Name   string
		Raw    func() []byte
		Fail   bool
		Expect func([]byte) Record
	}{
		{
			Name: "Not enough data for header len",
			Raw: func() []byte {
				return make([]byte, 2)
			},
			Fail: true,
		},
		{
			Name: "Not enough data for header",
			Raw: func() []byte {
				raw := make([]byte, 4)
				endian.PutUint32(raw, 4)
				return raw
			},
			Fail: true,
		},
		{
			Name: "Not enough data for data len",
			Raw: func() []byte {
				raw := make([]byte, 12)
				endian.PutUint32(raw, 8)
				endian.PutUint32(raw[4:], 4)
				copy(raw[8:], []byte("op="))
				raw[11] = 0x03
				return raw
			},
			Fail: true,
		},
		{
			Name: "Exactly 1 record",
			Raw: func() []byte {
				raw := make([]byte, 17)
				endian.PutUint32(raw, 8)
				endian.PutUint32(raw[4:], 4)
				copy(raw[8:], []byte("op="))
				raw[11] = 0x03
				endian.PutUint32(raw[12:], 1)
				return raw
			},
			Expect: func(b []byte) Record {
				record := &RecordBagHeader{
					RecordBase: &RecordBase{
						HeaderLen: 8,
						DataLen:   1,
						Raw:       b,
					},
				}

				return record
			},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.Name, func(t *testing.T) {
			raw := testCase.Raw()
			var actualBase RecordBase

			decoder := NewDecoder(bytes.NewReader(raw))
			decoder.checkedVersion = true
			actual, err := decoder.decodeRecord(decoder.reader, &actualBase)

			if testCase.Fail && err == nil {
				t.Fatal("expected to fail")
			} else if !testCase.Fail && err != nil {
				t.Fatal("expected to succeed:", err)
			}

			if err == nil {
				expected := testCase.Expect(raw)
				if !reflect.DeepEqual(actual.Header(), expected.Header()) {
					t.Fatalf("expected record header to be\n\n%v\n\nbut got\n\n%v\n\n", expected.Header(), actual.Header())
				}

				if !reflect.DeepEqual(actual.Data(), expected.Data()) {
					t.Fatalf("expected record data to be\n\n%v\n\nbut got\n\n%v\n\n", expected.Data(), actual.Data())
				}
			}
		})
	}
}

func TestDecoderNext(t *testing.T) {
	/* TODO: fix this unit test after all decoding stuff is done
	version := []byte("#ROSBAG V2.0\n")
	records := make([]byte, 22)
	rand.Read(records)
	endian.PutUint32(records, 1)
	endian.PutUint32(records[5:], 1)
	endian.PutUint32(records[10:], 2)
	endian.PutUint32(records[16:], 2)
	raw := append(version, records...)

	expected := []*RecordBase{
		{header: records[4:5], data: records[9:10]},
		{header: records[14:16], data: records[20:]},
	}

	in := bytes.NewReader(raw)
	decoder := NewDecoder(in)

	for i := 0; i < 2; i++ {
		actual, err := decoder.Next()
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(actual, expected[i]) {
			t.Fatalf("expected record to be\n\n%v\n\nbut got\n\n%v\n\n", expected[i], actual)
		}
	}
	*/
}

func TestIterateHeaderFields(t *testing.T) {
	add := func(header, key, value []byte) []byte {
		fieldLen := uint32(len(key) + 1 + len(value))
		endian.PutUint32(header, fieldLen)
		header = header[lenInBytes:]
		copy(header, key)
		header = header[len(key):]
		header[0] = headerFieldDelimiter
		header = header[1:]
		copy(header, value)
		header = header[len(value):]
		return header
	}

	testCases := []struct {
		Name           string
		ExpectedFields [][2][]byte
		Header         func() []byte
		Fail           bool
	}{
		{
			Name: "Single Field",
			ExpectedFields: [][2][]byte{
				{[]byte("op"), []byte{0x03}},
			},
			Header: func() []byte {
				header := make([]byte, 8)
				return header
			},
		},
		{
			Name: "Multiple Fields",
			ExpectedFields: [][2][]byte{
				{[]byte("op"), []byte{0x03}},
				{[]byte("key1"), []byte("value1")},
			},
			Header: func() []byte {
				header := make([]byte, 23)
				return header
			},
		},
		{
			Name: "Invalid Field Length",
			Header: func() []byte {
				header := make([]byte, 8)
				endian.PutUint32(header, 1)
				return header
			},
			Fail: true,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase

		t.Run(testCase.Name, func(t *testing.T) {
			i := 0
			expectedFields := testCase.ExpectedFields

			header := testCase.Header()
			cur := header
			for _, field := range testCase.ExpectedFields {
				key, value := field[0], field[1]
				cur = add(cur, key, value)
			}

			err := iterateHeaderFields(header, func(key, value []byte) bool {
				expectedKey, expectedValue := expectedFields[i][0], expectedFields[i][1]

				if !bytes.Equal(expectedKey, key) {
					t.Fatalf("expect header field to be %v, but got %v", expectedKey, key)
				}

				if !bytes.Equal(expectedValue, value) {
					t.Fatalf("expect value to be %v, but got %v", expectedValue, value)
				}

				i++
				return true
			})

			if testCase.Fail && err == nil {
				t.Fatal("expected to fail")
			} else if !testCase.Fail && err != nil {
				t.Fatal("expected to succeed:", err)
			}

			if err == nil && len(testCase.ExpectedFields) != i {
				t.Fatalf("expected the number of fields to be %d, but got %d", len(testCase.ExpectedFields), i)
			}
		})
	}
}
