package rosbag

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unsafe"
)

const (
	rosbagStructTag = "rosbag"
)

var (
	errInvalidFormat     = errors.New("invalid message format")
	errUnresolvedMsgType = errors.New("failed to resolve a complex message type")
	errInvalidConstType  = errors.New("invalid const type")
)

var hostEndian binary.ByteOrder

func init() {
	switch v := *(*uint16)(unsafe.Pointer(&([]byte{0x12, 0x34}[0]))); v {
	case 0x1234:
		hostEndian = binary.BigEndian
	case 0x3412:
		hostEndian = binary.LittleEndian
	default:
		panic(fmt.Sprintf("failed to determine host endianness: %x", v))
	}
}

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
	MessageFieldTypeComplex
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
	Type   string
	Fields []*MessageFieldDefinition
}

// decodeConstValue decodes raw to concrete type. Raw is expected to be in ASCII.
// Constant types can be any builtin types except Time and Duration.
// Reference: http://wiki.ros.org/msg#Constants
func decodeConstValue(fieldType MessageFieldType, raw []byte) (interface{}, error) {
	rawStr := string(raw)

	switch fieldType {
	case MessageFieldTypeBool:
		v, err := strconv.ParseUint(rawStr, 10, 8)
		if err != nil {
			return nil, err
		}

		return v != 0, nil
	case MessageFieldTypeInt8:
		v, err := strconv.ParseInt(rawStr, 10, 8)
		return int8(v), err
	case MessageFieldTypeUint8:
		v, err := strconv.ParseUint(rawStr, 10, 8)
		return uint8(v), err
	case MessageFieldTypeInt16:
		v, err := strconv.ParseInt(rawStr, 10, 16)
		return int16(v), err
	case MessageFieldTypeUint16:
		v, err := strconv.ParseUint(rawStr, 10, 16)
		return uint16(v), err
	case MessageFieldTypeInt32:
		v, err := strconv.ParseInt(rawStr, 10, 32)
		return int32(v), err
	case MessageFieldTypeUint32:
		v, err := strconv.ParseUint(rawStr, 10, 32)
		return uint32(v), err
	case MessageFieldTypeInt64:
		return strconv.ParseInt(rawStr, 10, 64)
	case MessageFieldTypeUint64:
		return strconv.ParseUint(rawStr, 10, 64)
	case MessageFieldTypeFloat32:
		v, err := strconv.ParseFloat(rawStr, 32)
		return float32(v), err
	case MessageFieldTypeFloat64:
		return strconv.ParseFloat(rawStr, 64)
	case MessageFieldTypeString:
		return rawStr, nil
	default:
		return nil, errInvalidConstType
	}
}

func (def *MessageDefinition) unmarshall(b []byte) error {
	var err error
	lines := bytes.Split(b, []byte("\n"))
	unresolvedFields := make(map[*MessageFieldDefinition][]byte)
	complexMsgs := []*MessageDefinition{def}

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
			complexMsgs = append(complexMsgs, &MessageDefinition{Type: string(msgType)})
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
		msgFieldType := newMessageFieldType(fieldType)
		var constantValue interface{}
		if idx != -1 {
			// TODO: parse this constantValue
			constantValue, err = decodeConstValue(msgFieldType, bytes.TrimSpace(fieldName[idx+1:]))
			fieldName = bytes.TrimSpace(fieldName[:idx])

		}

		complexMsg := complexMsgs[len(complexMsgs)-1]
		fieldDef := MessageFieldDefinition{
			Type:      msgFieldType,
			Name:      string(fieldName),
			IsArray:   isArray,
			ArraySize: arraySize,
			Value:     constantValue,
		}

		if fieldDef.Type == MessageFieldTypeComplex {
			unresolvedFields[&fieldDef] = fieldType
		}
		complexMsg.Fields = append(complexMsg.Fields, &fieldDef)
	}

	for field, msgType := range unresolvedFields {
		msgDef := findComplexMsg(complexMsgs, string(msgType))
		if msgDef == nil {
			return errUnresolvedMsgType
		}

		field.MsgType = msgDef
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

	return sb.String()
}

type MessageFieldDefinition struct {
	Type    MessageFieldType
	Name    string
	IsArray bool
	// ArraySize is only used when the field is a fixed-size array
	ArraySize int
	// Value is an optional field. It's only being used for constants
	Value interface{}
	// MsgType is only being used when type is complex. This defines the custom
	// message type.
	MsgType *MessageDefinition
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

	return fmt.Sprintf("%s %s\n", fieldType, def.Name)
}

// findComplexMsg iterates complexMsgs, and find for msgType. msgType can have an optional
// package name as prefix.
func findComplexMsg(complexMsgs []*MessageDefinition, msgType string) *MessageDefinition {
	for _, cur := range complexMsgs {
		if strings.HasSuffix(cur.Type, msgType) {
			return cur
		}
	}
	return nil
}

func decodeMessageData(def *MessageDefinition, raw []byte, data map[string]interface{}) ([]byte, error) {
	var err error
	for _, field := range def.Fields {
		// Const value, no need to parse, simply fill in the data
		if field.Value != nil {
			data[field.Name] = field.Value
		} else if field.Type == MessageFieldTypeComplex {
			data[field.Name], raw, err = decodeFieldComplex(field, raw)
		} else {
			data[field.Name], raw, err = decodeFieldBasic(field, raw)
		}

		if err != nil {
			return nil, err
		}
	}

	return raw, nil
}

func decodeFieldBasic(field *MessageFieldDefinition, raw []byte) (interface{}, []byte, error) {
	var decodeFuncs map[MessageFieldType]fieldDecodeFunc
	var length int
	if field.IsArray {
		decodeFuncs = fieldDecodeSliceHelper
		length = field.ArraySize
	} else {
		decodeFuncs = fieldDecodeBasicHelper
	}

	v, off, ok := decodeFuncs[field.Type](raw, length)
	if !ok {
		return nil, raw, errInvalidFormat
	}

	return v, raw[off:], nil
}

func decodeFieldComplex(field *MessageFieldDefinition, raw []byte) (interface{}, []byte, error) {
	var length int = 1
	if field.IsArray {
		var off int
		var ok bool
		length, off, ok = fieldDecodeLength(raw, field.ArraySize)
		if !ok {
			return nil, raw, errInvalidFormat
		}
		raw = raw[off:]
	}

	var err error
	vs := make([]map[string]interface{}, length)
	for i := range vs {
		vs[i] = make(map[string]interface{})
		raw, err = decodeMessageData(field.MsgType, raw, vs[i])
		if err != nil {
			return nil, raw, err
		}
	}

	if field.IsArray {
		return vs, raw, nil
	}

	return vs[0], raw, nil
}
