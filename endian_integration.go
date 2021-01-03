// +build integration

package rosbag

import (
	"encoding/binary"
)

// Simulate big endian for testing big endian machines. This integration test MUST run
// from a little endian machine
var (
	endian     binary.ByteOrder = binary.BigEndian
	hostEndian binary.ByteOrder = binary.BigEndian
)

func init() {
	initFieldSliceDecoder(false)
}
