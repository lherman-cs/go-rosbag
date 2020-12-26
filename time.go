package rosbag

import (
	"fmt"
	"time"
)

func nanoToTime(nsec uint64) time.Time {
	fmt.Println(nsec)
	sec := nsec / 1e9
	nsec -= sec * 1e9
	fmt.Println(sec, nsec)
	return time.Unix(int64(sec), int64(nsec))
}
