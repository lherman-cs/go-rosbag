package rosbag

import (
	"bufio"
	"bytes"
	"math/rand"
	"reflect"
	"testing"
)

func TestDecoderCheckVersion(t *testing.T) {
	testCases := []struct {
		Name string
		Raw  []byte
		Fail bool
	}{
		{
			Name: "Missing Newline character",
			Raw:  []byte("#ROSBAG V2.0"),
			Fail: true,
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

func TestDecoderScanRecordsSingleRecord(t *testing.T) {
	testCases := []struct {
		Name   string
		Raw    func() []byte
		Fail   bool
		Expect func([]byte) *RecordBase
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
				raw := make([]byte, 5)
				endian.PutUint32(raw, 1)
				return raw
			},
			Fail: true,
		},
		{
			Name: "Not enough data for data",
			Raw: func() []byte {
				raw := make([]byte, 9)
				endian.PutUint32(raw, 1)
				endian.PutUint16(raw[5:], 4)
				return raw
			},
			Fail: true,
		},
		{
			Name: "Exactly 1 record",
			Raw: func() []byte {
				raw := make([]byte, 11)
				rand.Read(raw)
				endian.PutUint32(raw, 1)
				endian.PutUint32(raw[5:9], 2)
				return raw
			},
			Expect: func(b []byte) *RecordBase {
				return &RecordBase{
					header: b[4:5],
					data:   b[9:11],
				}
			},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.Name, func(t *testing.T) {
			var actual *RecordBase
			raw := testCase.Raw()
			in := bytes.NewReader(raw)
			scanner := bufio.NewScanner(in)
			scanner.Split(newScanRecords(func(record *RecordBase) {
				actual = record
			}))
			found := scanner.Scan()
			err := scanner.Err()

			if testCase.Fail && (err == nil || found) {
				t.Fatal("expected to fail")
			} else if !testCase.Fail && (err != nil || !found) {
				t.Fatal("expected to succeed:", err)
			}

			if found {
				expected := testCase.Expect(raw)
				if !reflect.DeepEqual(actual, expected) {
					t.Fatalf("expected record to be\n\n%v\n\nbut got\n\n%v\n\n", expected, actual)
				}
			}
		})
	}
}

func TestDecoderScanRecordsMultipleRecords(t *testing.T) {
	raw := make([]byte, 22)
	rand.Read(raw)
	endian.PutUint32(raw, 1)
	endian.PutUint32(raw[5:], 1)
	endian.PutUint32(raw[10:], 2)
	endian.PutUint32(raw[16:], 2)

	expected := []*RecordBase{
		{header: raw[4:5], data: raw[9:10]},
		{header: raw[14:16], data: raw[20:]},
	}

	var actual *RecordBase
	in := bytes.NewReader(raw)
	scanner := bufio.NewScanner(in)
	scanner.Split(newScanRecords(func(record *RecordBase) {
		actual = record
	}))

	for i := 0; i < 2; i++ {
		found := scanner.Scan()
		if !found || scanner.Err() != nil {
			t.Fatal("expected to get 2 records")
		}

		if !reflect.DeepEqual(actual, expected[i]) {
			t.Fatalf("expected record to be\n\n%v\n\nbut got\n\n%v\n\n", expected[i], actual)
		}
	}

	found := scanner.Scan()
	if found {
		t.Fatal("expect no more tokens")
	}

	err := scanner.Err()
	if err != nil {
		t.Fatalf("expect EOF, so there shouldn't be any error, but got \"%v\"", err)
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
		header = header[headerLenInBytes:]
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
