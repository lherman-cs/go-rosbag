package rosbag

import "fmt"

type ConnectionHeader struct {
	Topic             string
	Type              string
	MD5Sum            string
	MessageDefinition MessageDefinition
}

func (header *ConnectionHeader) String() string {
	return fmt.Sprintf(`
topic  : %s
type   : %s
md5sum : %s
`, header.Topic, header.Type, header.MD5Sum)
}

type MessageDefinition struct {
}
