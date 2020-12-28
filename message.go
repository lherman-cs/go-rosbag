package rosbag

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"strings"
)

const (
	rosbagStructTag = "rosbag"
)

type MessageFieldType uint8

const (
	MessageFieldTypeComplex MessageFieldType = iota
	MessageFieldTypeBool
	MessageFieldTypeInt8
	MessageFieldTypeUint8
	MessageFieldTypeInt16
	MessageFieldTypeUint16
	MessageFieldTypeInt32
	MessageFieldTypeUint32
	MessageFieldTypeInt64
	MessageFieldTypeUint64
	MessageFieldTypeFloat32
	MessageFieldTypeFloat64
	MessageFieldTypeString
	MessageFieldTypeTime
	MessageFieldTypeDuration
)

func newMessageFieldType(b []byte) MessageFieldType {
	if bytes.Equal(b, []byte("byte")) {
		return MessageFieldTypeInt8
	}

	if bytes.Equal(b, []byte("char")) {
		return MessageFieldTypeUint8
	}

	for t := MessageFieldTypeBool; t <= MessageFieldTypeDuration; t++ {
		if bytes.Equal(b, []byte(t.String())) {
			return t
		}
	}

	return MessageFieldTypeComplex
}

func (t MessageFieldType) String() string {
	switch t {
	case MessageFieldTypeBool:
		return "bool"
	case MessageFieldTypeInt8:
		return "int8"
	case MessageFieldTypeUint8:
		return "uint8"
	case MessageFieldTypeInt16:
		return "int16"
	case MessageFieldTypeUint16:
		return "uint16"
	case MessageFieldTypeInt32:
		return "int32"
	case MessageFieldTypeUint32:
		return "uint32"
	case MessageFieldTypeInt64:
		return "int64"
	case MessageFieldTypeUint64:
		return "uint64"
	case MessageFieldTypeFloat32:
		return "float32"
	case MessageFieldTypeFloat64:
		return "float64"
	case MessageFieldTypeString:
		return "string"
	case MessageFieldTypeTime:
		return "time"
	case MessageFieldTypeDuration:
		return "duration"
	default:
		return "invalid"
	}
}

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
	Type        []byte
	Fields      []MessageFieldDefinition
	ComplexMsgs []MessageDefinition
}

func (def *MessageDefinition) unmarshall(b []byte) error {
	var err error
	lines := bytes.Split(b, []byte("\n"))

	for _, line := range lines {
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
			def.ComplexMsgs = append(def.ComplexMsgs, MessageDefinition{Type: msgType})
			continue
		}

		idx = bytes.IndexByte(line, ' ')
		fieldType := line[:idx]
		fieldName := bytes.TrimSpace(line[idx+1:])

		idx = bytes.IndexByte(fieldType, '[')
		var isArray bool
		var arraySize int
		if idx != -1 {
			off := bytes.IndexByte(fieldType[idx:], ']')
			if off > 1 {
				arraySizeRaw := fieldType[idx+1 : idx+off]
				arraySize, err = strconv.Atoi(string(arraySizeRaw))
				if err != nil {
					return err
				}
			}

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
			Type:      newMessageFieldType(fieldType),
			Name:      fieldName,
			IsArray:   isArray,
			ArraySize: arraySize,
			Value:     constantValue,
		}

		if fieldDef.Type == MessageFieldTypeComplex {
			fieldDef.MsgType = fieldType
		}
		complexMsg.Fields = append(complexMsg.Fields, fieldDef)
	}

	return nil
}

func (def *MessageDefinition) String() string {
	var sb strings.Builder

	if len(def.Type) > 0 {
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
	Type    MessageFieldType
	Name    []byte
	IsArray bool
	// ArraySize is only used when the field is a fixed-size array
	ArraySize int
	// Value is an optional field. It's only being used for constants
	Value []byte
	// MsgType is only being used when type is complex. This defines the custom
	// message type.
	MsgType []byte
}

func (def *MessageFieldDefinition) String() string {
	fieldType := def.Type.String()
	if def.IsArray {
		if def.ArraySize > 0 {
			fieldType += fmt.Sprintf("[%d]", def.ArraySize)
		} else {
			fieldType += "[]"
		}
	}

	fieldValue := ""
	if len(def.Value) > 0 {
		fieldValue += "=" + string(def.Value)
	}
	return fmt.Sprintf("%s %s%s\n", fieldType, def.Name, fieldValue)
}

// findComplexMsg iterates ComplexMsgs inside def, and find for msgType. msgType can have an optional
// package name as prefix.
func findComplexMsg(def *MessageDefinition, msgType []byte) *MessageDefinition {
	for _, cur := range def.ComplexMsgs {
		if bytes.HasSuffix(cur.Type, msgType) {
			return &cur
		}
	}
	return nil
}

func decodeMessageData(def *MessageDefinition, raw []byte, data map[string]interface{}) error {
	var visit func(*MessageDefinition, map[string]interface{}, []byte) ([]byte, error)
	visit = func(curDef *MessageDefinition, curValue map[string]interface{}, curRaw []byte) ([]byte, error) {
		var err error
		for _, field := range curDef.Fields {
			var newValue interface{}
			// TODO: this is const, need to parse this
			if len(field.Value) != 0 {
				continue
			}

			length := 1
			if field.IsArray {
				if field.ArraySize != 0 {
					length = field.ArraySize
				} else {
					// If the array is dynamic, the array length is defined as 4 bytes integer
					length = int(endian.Uint16(curRaw))
					curRaw = curRaw[4:]
				}
			}

			values := make([]interface{}, length)

			for i := 0; i < length; i++ {
				switch field.Type {
				case MessageFieldTypeBool:
					var isTrue bool
					if curRaw[0] != 0 {
						isTrue = true
					}
					newValue = isTrue
					curRaw = curRaw[1:]
				case MessageFieldTypeInt8:
					newValue = int8(curRaw[0])
					curRaw = curRaw[1:]
				case MessageFieldTypeUint8:
					newValue = uint8(curRaw[0])
					curRaw = curRaw[1:]
				case MessageFieldTypeInt16:
					newValue = int16(endian.Uint16(curRaw))
					curRaw = curRaw[2:]
				case MessageFieldTypeUint16:
					newValue = endian.Uint16(curRaw)
					curRaw = curRaw[2:]
				case MessageFieldTypeInt32:
					newValue = int32(endian.Uint32(curRaw))
					curRaw = curRaw[4:]
				case MessageFieldTypeUint32:
					newValue = endian.Uint32(curRaw)
					curRaw = curRaw[4:]
				case MessageFieldTypeInt64:
					newValue = int64(endian.Uint64(curRaw))
					curRaw = curRaw[8:]
				case MessageFieldTypeUint64:
					newValue = endian.Uint64(curRaw)
					curRaw = curRaw[8:]
				case MessageFieldTypeFloat32:
					newValue = math.Float32frombits(endian.Uint32(curRaw))
					curRaw = curRaw[4:]
				case MessageFieldTypeFloat64:
					newValue = math.Float64frombits(endian.Uint64(curRaw))
					curRaw = curRaw[8:]
				case MessageFieldTypeString:
					length := endian.Uint32(curRaw)
					curRaw = curRaw[4:]
					newValue = string(curRaw[:length])
					curRaw = curRaw[length:]
				case MessageFieldTypeTime:
					newValue = extractTime(curRaw)
					curRaw = curRaw[8:]
				case MessageFieldTypeDuration:
					newValue = extractDuration(curRaw)
					curRaw = curRaw[8:]
				case MessageFieldTypeComplex:
					newValueReal := make(map[string]interface{})
					newValue = newValueReal
					curRaw, err = visit(findComplexMsg(def, field.MsgType), newValueReal, curRaw)
					if err != nil {
						return nil, err
					}
				}

				values[i] = newValue
			}

			if field.IsArray {
				curValue[string(field.Name)] = values
			} else {
				curValue[string(field.Name)] = values[0]
			}
		}
		return curRaw, nil
	}

	_, err := visit(def, data, raw)
	return err
}
