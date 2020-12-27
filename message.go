package rosbag

import (
	"bytes"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
)

const (
	rosbagStructTag = "rosbag"
)

/* TODO: Add this later to speed up a bit message decoder
type MessageFieldType uint8

const (
	MessageFieldTypeBool MessageFieldType = iota + 1
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

func (t MessageFieldType) String() string {
	switch t {
	case MessageFieldTypeBool:
		return "bool"
	case MessageFieldTypeInt8:
		return "int8"
	case MessageFieldTypeInt16:
		return "int16"
	case MessageFieldTypeInt32:
		return "int32"
	default:
		return "invalid"
	}
}
*/

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
	var err error
	lines := bytes.Split(b, []byte("\n"))
	fmt.Println(string(b))

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
			def.ComplexMsgs = append(def.ComplexMsgs, MessageDefinition{Type: string(msgType)})
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
			Type:      string(fieldType),
			Name:      string(fieldName),
			IsArray:   isArray,
			ArraySize: arraySize,
			Value:     constantValue,
		}
		complexMsg.Fields = append(complexMsg.Fields, fieldDef)
	}

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
	// ArraySize is only used when the field is a fixed-size array
	ArraySize int
	// Value is an optional field. It's only being used for constants
	Value []byte
}

func (def *MessageFieldDefinition) String() string {
	fieldType := def.Type
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
func findComplexMsg(def *MessageDefinition, msgType string) *MessageDefinition {
	for _, cur := range def.ComplexMsgs {
		if strings.HasSuffix(cur.Type, msgType) {
			return &cur
		}
	}
	return nil
}

func decodeMessageData(def *MessageDefinition, raw []byte, data interface{}) error {
	var visit func(*MessageDefinition, reflect.Value, []byte) ([]byte, error)
	visit = func(curDef *MessageDefinition, curValue reflect.Value, curRaw []byte) ([]byte, error) {
		if curValue.Kind() == reflect.Ptr {
			curValue = reflect.Indirect(curValue)
		}

		lookupMap := make(map[string]reflect.Value)
		curType := curValue.Type()
		for i := 0; i < curType.NumField(); i++ {
			field := curType.Field(i)
			tag, ok := field.Tag.Lookup(rosbagStructTag)
			if !ok {
				tag = field.Name
			}
			lookupMap[tag] = curValue.Field(i)
		}

		for _, field := range curDef.Fields {
			fieldValue, ok := lookupMap[field.Name]
			if !ok {
				continue
			}

			switch field.Type {
			case "bool":
				var isTrue bool
				if curRaw[0] != 0 {
					isTrue = true
				}
				fieldValue.SetBool(isTrue)
				curRaw = curRaw[1:]
			case "int8":
				fieldValue.SetInt(int64(curRaw[0]))
				curRaw = curRaw[1:]
			case "uint8":
				fieldValue.SetUint(uint64(curRaw[0]))
				curRaw = curRaw[1:]
			case "int16":
				fieldValue.SetInt(int64(endian.Uint16(curRaw)))
				curRaw = curRaw[2:]
			case "uint16":
				fieldValue.SetUint(uint64(endian.Uint16(curRaw)))
				curRaw = curRaw[2:]
			case "int32":
				fieldValue.SetInt(int64(endian.Uint32(curRaw)))
				curRaw = curRaw[4:]
			case "uint32":
				fieldValue.SetUint(uint64(endian.Uint32(curRaw)))
				curRaw = curRaw[4:]
			case "int64":
				fieldValue.SetInt(int64(endian.Uint64(curRaw)))
				curRaw = curRaw[8:]
			case "uint64":
				fieldValue.SetUint(endian.Uint64(curRaw))
				curRaw = curRaw[8:]
			case "float32":
				fieldValue.SetFloat(float64(math.Float32frombits(endian.Uint32(curRaw))))
				curRaw = curRaw[4:]
			case "float64":
				fieldValue.SetFloat(math.Float64frombits(endian.Uint64(curRaw)))
				curRaw = curRaw[8:]
			}
		}
		return nil, nil
	}

	_, err := visit(def, reflect.ValueOf(data), raw)
	return err
}
