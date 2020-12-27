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
		var err error
		if curValue.Kind() == reflect.Ptr {
			curValue = reflect.Indirect(curValue)
		}

		lookupMap := make(map[string]reflect.Value)
		curType := curValue.Type()
		if curType.Kind() == reflect.Struct {
			for i := 0; i < curType.NumField(); i++ {
				field := curType.Field(i)
				tag, ok := field.Tag.Lookup(rosbagStructTag)
				if !ok {
					tag = field.Name
				}
				lookupMap[tag] = curValue.Field(i)
			}
		}

		fmt.Println(curDef)
		fmt.Println(curDef.Fields)
		for _, field := range curDef.Fields {
			// TODO: this is const, need to parse this
			if len(field.Value) != 0 {
				continue
			}

			fieldValue, ok := lookupMap[field.Name]
			if !ok && curType.Kind() == reflect.Struct {
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

			for i := 0; i < length; i++ {
				var newValue reflect.Value

				switch field.Type {
				case "bool":
					var isTrue bool
					if curRaw[0] != 0 {
						isTrue = true
					}
					newValue = reflect.ValueOf(isTrue)
					curRaw = curRaw[1:]
				case "byte":
					fallthrough
				case "int8":
					newValue = reflect.ValueOf(int8(curRaw[0]))
					curRaw = curRaw[1:]
				case "char":
					fallthrough
				case "uint8":
					newValue = reflect.ValueOf(uint8(curRaw[0]))
					curRaw = curRaw[1:]
				case "int16":
					newValue = reflect.ValueOf(int16(endian.Uint16(curRaw)))
					curRaw = curRaw[2:]
				case "uint16":
					newValue = reflect.ValueOf(endian.Uint16(curRaw))
					curRaw = curRaw[2:]
				case "int32":
					newValue = reflect.ValueOf(int32(endian.Uint32(curRaw)))
					curRaw = curRaw[4:]
				case "uint32":
					newValue = reflect.ValueOf(endian.Uint32(curRaw))
					curRaw = curRaw[4:]
				case "int64":
					newValue = reflect.ValueOf(int64(endian.Uint64(curRaw)))
					curRaw = curRaw[8:]
				case "uint64":
					newValue = reflect.ValueOf(endian.Uint64(curRaw))
					curRaw = curRaw[8:]
				case "float32":
					newValue = reflect.ValueOf(math.Float32frombits(endian.Uint32(curRaw)))
					curRaw = curRaw[4:]
				case "float64":
					newValue = reflect.ValueOf(math.Float64frombits(endian.Uint64(curRaw)))
					curRaw = curRaw[8:]
				case "string":
					length := endian.Uint32(curRaw)
					curRaw = curRaw[4:]
					newValue = reflect.ValueOf(string(curRaw[:length]))
					curRaw = curRaw[length:]
				case "time":
					newValue = reflect.ValueOf(extractTime(curRaw))
					curRaw = curRaw[8:]
				case "duration":
					newValue = reflect.ValueOf(extractDuration(curRaw))
					curRaw = curRaw[8:]
				default:
					if curType.Kind() == reflect.Struct {
						if field.IsArray {
							newValue = reflect.New(reflect.TypeOf(fieldValue.Interface()).Elem())
						} else {
							newValue = reflect.New(fieldValue.Type())
						}
						newValue = reflect.Indirect(newValue)
					} else {
						newValue = reflect.ValueOf(make(map[string]interface{}))
					}

					curRaw, err = visit(findComplexMsg(def, field.Type), newValue, curRaw)
					if err != nil {
						return nil, err
					}
				}

				if ok {
					if field.IsArray {
						fieldValue.Set(reflect.Append(fieldValue, newValue))
					} else {
						fieldValue.Set(newValue)
					}
				} else {
					curValue.SetMapIndex(reflect.ValueOf(field.Name), newValue)
				}
			}
		}
		return curRaw, nil
	}

	_, err := visit(def, reflect.ValueOf(data), raw)
	return err
}
