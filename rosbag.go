package rosbag

import "fmt"

const (
	versionFormat = "#ROSBAG V%d.%d"
)

var (
	supportedVersion = Version{
		Major: 2,
		Minor: 0,
	}
)

type Version struct {
	Major uint
	Minor uint
}

func (version *Version) String() string {
	return fmt.Sprintf("%d.%d", version.Major, version.Minor)
}

type Rosbag struct {
	Version Version
	Record  []Record
}

type Record interface {
	Header() []byte
	Data() []byte
}

type RecordBase struct {
	Header []byte
	Data   []byte
}
