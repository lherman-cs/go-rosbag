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
	header []byte
	data   []byte
}

func (record *RecordBase) Header() []byte {
	return record.header
}

func (record *RecordBase) Data() []byte {
	return record.data
}
