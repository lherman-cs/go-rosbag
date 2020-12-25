package rosbag

import (
	"bytes"
	"reflect"
	"testing"
)

func TestDecoderVersion(t *testing.T) {
	testCases := []struct {
		Name     string
		Raw      []byte
		Expected Version
		Fail     bool
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
			Expected: Version{
				Major: 2,
				Minor: 0,
			},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.Name, func(t *testing.T) {
			var rosbag Rosbag
			in := bytes.NewReader(testCase.Raw)
			err := NewDecoder(in).Decode(&rosbag)

			if testCase.Fail && err == nil {
				t.Fatal("expected to fail")
			} else if !testCase.Fail && err != nil {
				t.Fatal("expected to succeed")
			} else if err == nil && !reflect.DeepEqual(rosbag.Version, testCase.Expected) {
				t.Fatalf("expected version to be\n\n%v\n\nbut got\n\n%v\n", testCase.Expected, rosbag.Version)
			}
		})
	}
}
