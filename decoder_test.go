package rosbag

import (
	"bytes"
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
