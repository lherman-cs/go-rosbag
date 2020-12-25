package rosbag

import (
	"bufio"
	"bytes"
	"encoding/binary"
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
				return nil
			},
			Fail: true,
		},
		{
			Name: "Not enough data for header",
			Raw: func() []byte {
				raw := make([]byte, 4)
				binary.LittleEndian.PutUint32(raw, 4)
				return raw
			},
			Fail: true,
		},
		{
			Name: "Not enough data for data len",
			Raw: func() []byte {
				raw := make([]byte, 5)
				binary.LittleEndian.PutUint32(raw, 1)
				return raw
			},
			Fail: true,
		},
		{
			Name: "Not enough data for data",
			Raw: func() []byte {
				raw := make([]byte, 9)
				binary.LittleEndian.PutUint32(raw, 1)
				binary.LittleEndian.PutUint16(raw[5:], 4)
				return raw
			},
			Fail: true,
		},
		{
			Name: "Exactly 1 record",
			Raw: func() []byte {
				raw := make([]byte, 11)
				rand.Read(raw)
				binary.LittleEndian.PutUint32(raw, 1)
				binary.LittleEndian.PutUint32(raw[5:9], 2)
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
	binary.LittleEndian.PutUint32(raw, 1)
	binary.LittleEndian.PutUint32(raw[5:], 1)
	binary.LittleEndian.PutUint32(raw[10:], 2)
	binary.LittleEndian.PutUint32(raw[16:], 2)

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
}

func TestDecoderNext(t *testing.T) {
	version := []byte("#ROSBAG V2.0\n")
	records := make([]byte, 22)
	rand.Read(records)
	binary.LittleEndian.PutUint32(records, 1)
	binary.LittleEndian.PutUint32(records[5:], 1)
	binary.LittleEndian.PutUint32(records[10:], 2)
	binary.LittleEndian.PutUint32(records[16:], 2)
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
}
