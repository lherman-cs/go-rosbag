package rosbag

import (
	"time"
)

// extractTime extracts raw data to Go time.Time. raw MUST contain at least
// 8 bytes of data
func extractTime(raw []byte) time.Time {
	sec := endian.Uint32(raw)
	nsec := endian.Uint32(raw[4:])
	return time.Unix(int64(sec), int64(nsec))
}

// extractDuration is similar to extractTime, but it outputs duration
func extractDuration(raw []byte) time.Duration {
	sec := endian.Uint32(raw)
	nsec := endian.Uint32(raw[4:])
	return time.Duration(sec)*time.Second + time.Duration(nsec)*time.Nanosecond
}
