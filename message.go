package rosbag

import (
	"bytes"
	"fmt"
	"strings"
)

type ConnectionHeader struct {
	Topic             string
	Type              string
	MD5Sum            string
	MessageDefinition MessageDefinition
}

func (header *ConnectionHeader) String() string {
	return fmt.Sprintf(`
================
topic              : %s
type               : %s
md5sum             : %s
message_definition : 

%s
`, header.Topic, header.Type, header.MD5Sum, &header.MessageDefinition)
}

// MessageDefinition is defined here, http://wiki.ros.org/msg
type MessageDefinition struct {
	Type        string
	Fields      []MessageFieldDefinition
	ComplexMsgs []MessageDefinition
}

func (def *MessageDefinition) unmarshall(b []byte) error {
	fmt.Println(string(b))
	lines := bytes.Split(b, []byte("\n"))

	fmt.Printf("READING %d LINES\n", len(lines))
	for i, line := range lines {
		fmt.Printf("READING LINE #%d: %s\n", i, string(line))
		// find comments
		idx := bytes.IndexByte(line, '#')
		if idx != -1 {
			line = line[:idx]
		}

		// remove whitespaces
		line = bytes.TrimSpace(line)

		// these are usually comment lines, ignore
		if len(line) == 0 {
			continue
		}

		// at this point, if there's a '=', it just means a separator, ignore
		if line[0] == '=' {
			continue
		}

		// detect if this is a complex message definition
		idx = bytes.IndexByte(line, ':')
		if idx != -1 {
			idx = bytes.LastIndexByte(line, ' ')
			msgType := line[idx+1:]
			def.ComplexMsgs = append(def.ComplexMsgs, MessageDefinition{Type: string(msgType)})
			continue
		}

		idx = bytes.IndexByte(line, ' ')
		fieldType := line[:idx]
		fieldName := bytes.TrimSpace(line[idx+1:])

		idx = bytes.IndexByte(fieldType, '[')
		var isArray bool
		if idx != -1 {
			fieldType = fieldType[:idx]
			isArray = true
		}

		// detect constant
		idx = bytes.IndexByte(fieldName, '=')
		var constantValue []byte
		if idx != -1 {
			// TODO: parse this constantValue
			constantValue = bytes.TrimSpace(fieldName[idx+1:])
			fieldName = bytes.TrimSpace(fieldName[:idx])
		}

		complexMsg := def
		if len(def.ComplexMsgs) > 0 {
			complexMsg = &def.ComplexMsgs[len(def.ComplexMsgs)-1]
		}

		fieldDef := MessageFieldDefinition{
			Type:    string(fieldType),
			Name:    string(fieldName),
			IsArray: isArray,
			Value:   constantValue,
		}
		complexMsg.Fields = append(complexMsg.Fields, fieldDef)
	}

	fmt.Println("FINISHED UNMARSHALLING")

	return nil
}

func (def *MessageDefinition) String() string {
	var sb strings.Builder

	if def.Type != "" {
		sb.WriteString(fmt.Sprintf("MSG: %s\n", def.Type))
	}

	for _, field := range def.Fields {
		sb.WriteString(field.String())
	}

	for _, msg := range def.ComplexMsgs {
		sb.WriteString(msg.String())
	}

	return sb.String()
}

type MessageFieldDefinition struct {
	Type    string
	Name    string
	IsArray bool
	// Value is an optional field. It's only being used for constants
	Value []byte
}

func (def *MessageFieldDefinition) String() string {
	fieldType := def.Type
	if def.IsArray {
		fieldType += "[]"
	}

	fieldValue := ""
	if len(def.Value) > 0 {
		fieldValue += "=" + string(def.Value)
	}
	return fmt.Sprintf("%s %s%s\n", fieldType, def.Name, fieldValue)
}
