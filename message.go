package rosbag

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

const (
	rosbagStructTag = "rosbag"
)

var (
	errInvalidFormat     = errors.New("invalid message format")
	errUnresolvedMsgType = errors.New("failed to resolve a complex message type")
	errInvalidConstType  = errors.New("invalid const type")
	errInvalidDataType   = errors.New("data must be a map[string]interface{} or a pointer to a struct")
)

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

var (
	messageFieldTypeMap = map[string]MessageFieldType{
		"bool":     MessageFieldTypeBool,
		"int8":     MessageFieldTypeInt8,
		"byte":     MessageFieldTypeInt8,
		"uint8":    MessageFieldTypeUint8,
		"char":     MessageFieldTypeUint8,
		"int16":    MessageFieldTypeInt16,
		"uint16":   MessageFieldTypeUint16,
		"int32":    MessageFieldTypeInt32,
		"uint32":   MessageFieldTypeUint32,
		"int64":    MessageFieldTypeInt64,
		"uint64":   MessageFieldTypeUint64,
		"float32":  MessageFieldTypeFloat32,
		"float64":  MessageFieldTypeFloat64,
		"string":   MessageFieldTypeString,
		"time":     MessageFieldTypeTime,
		"duration": MessageFieldTypeDuration,
	}
)

type ConnectionHeader struct {
	Topic             string
	Type              string
	MD5Sum            string
	MessageDefinition MessageDefinition
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
		v, err := strconv.ParseBool(rawStr)
		return v, err
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
		var arraySize int = -1
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
		msgFieldType, ok := messageFieldTypeMap[string(fieldType)]
		if !ok {
			msgFieldType = MessageFieldTypeComplex
		}

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

type MessageFieldDefinition struct {
	Type    MessageFieldType
	Name    string
	IsArray bool
	// ArraySize is only used when the field is a fixed-size array. If it's a slice, ArraySize is -1
	ArraySize int
	// Value is an optional field. It's only being used for constants
	Value interface{}
	// MsgType is only being used when type is complex. This defines the custom
	// message type.
	MsgType *MessageDefinition
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

func createFieldMapper(structValue reflect.Value, mapper map[string]reflect.Value) {
	structType := structValue.Type()
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		fieldName, ok := field.Tag.Lookup(rosbagStructTag)
		if !ok {
			fieldName = field.Name
		}

		mapper[fieldName] = structValue.Field(i)
	}
}

func decodeMessageData(def *MessageDefinition, raw []byte, data interface{}) ([]byte, error) {
	var err error

	value := reflect.ValueOf(data)

	if value.Kind() == reflect.Ptr {
		value = reflect.Indirect(value)
	}

	var getFn func(string) reflect.Value
	var getFieldTypeFn func(string) reflect.Type
	var setFn func(string, interface{}) error
	switch value.Kind() {
	case reflect.Map:
		m := data.(map[string]interface{})
		setFn = func(k string, v interface{}) error {
			m[k] = v
			return nil
		}
		getFn = func(k string) reflect.Value {
			return reflect.ValueOf(make(map[string]interface{}))
		}
		getFieldTypeFn = func(k string) reflect.Type {
			var m map[string]interface{}
			return reflect.SliceOf(reflect.TypeOf(m))
		}
	case reflect.Struct:
		mapper := make(map[string]reflect.Value)
		createFieldMapper(value, mapper)
		setFn = func(k string, v interface{}) error {
			fieldValue, ok := mapper[k]
			if !ok {
				return nil
			}

			reflectValue := reflect.ValueOf(v)
			if reflectValue.Kind() != fieldValue.Kind() {
				return fmt.Errorf("message field %s is %s, but the struct field is %s", k, reflectValue.Kind(), fieldValue.Kind())
			}

			fieldValue.Set(reflectValue)
			return nil
		}
		getFn = func(k string) reflect.Value {
			fieldValue, ok := mapper[k]
			if !ok {
				// TODO: To keep the decoder keeps reading, we need to create this dummy map
				return reflect.ValueOf(make(map[string]interface{}))
			}

			return fieldValue
		}
		getFieldTypeFn = func(k string) reflect.Type {
			fieldValue, ok := mapper[k]
			if !ok {
				var m map[string]interface{}
				return reflect.SliceOf(reflect.TypeOf(m))
			}

			// TODO: Add slice type check
			return fieldValue.Type()
		}
	default:
		return nil, errInvalidDataType
	}

	var v interface{}
	for _, field := range def.Fields {
		// Const value, no need to parse, simply fill in the data
		if field.Value != nil {
			v = field.Value
		} else if field.Type != MessageFieldTypeComplex {
			v, raw, err = decodeFieldBasic(field, raw)
		} else if field.IsArray {
			t := getFieldTypeFn(field.Name)
			v, raw, err = decodeFieldComplexSlice(field, raw, t)
		} else {
			reflectValue := getFn(field.Name)
			if reflectValue.CanAddr() {
				// No need to set the field value since the change happens in place
				reflectValue = reflectValue.Addr()
				raw, err = decodeMessageData(field.MsgType, raw, reflectValue.Interface())

				// TODO: Probably should be flatenned this or refactor out
				if err != nil {
					return nil, err
				}
				continue
			}

			v = reflectValue.Interface()
			raw, err = decodeMessageData(field.MsgType, raw, v)
		}

		if err != nil {
			return nil, err
		}

		err = setFn(field.Name, v)
		if err != nil {
			return nil, err
		}
	}

	return raw, nil
}

func decodeFieldBasic(field *MessageFieldDefinition, raw []byte) (interface{}, []byte, error) {
	var decodeFuncs map[MessageFieldType]fieldDecodeFunc
	if field.IsArray {
		decodeFuncs = fieldDecodeSliceHelper
	} else {
		decodeFuncs = fieldDecodeBasicHelper
	}

	v, off, ok := decodeFuncs[field.Type](raw, field.ArraySize)
	if !ok {
		return nil, raw, errInvalidFormat
	}

	return v, raw[off:], nil
}

func decodeFieldComplexSlice(field *MessageFieldDefinition, raw []byte, fieldType reflect.Type) (interface{}, []byte, error) {
	var length int
	var off int
	var ok bool
	length, off, ok = fieldDecodeLength(raw, field.ArraySize)
	if !ok {
		return nil, raw, errInvalidFormat
	}
	raw = raw[off:]

	var err error
	vs := reflect.MakeSlice(fieldType, length, length)
	for i := 0; i < length; i++ {
		v := vs.Index(i)
		if v.Kind() == reflect.Map {
			v.Set(reflect.ValueOf(make(map[string]interface{})))
		} else if v.CanAddr() { // struct value
			v = v.Addr()
		} else if v.IsNil() { // struct pointer
			v.Set(reflect.New(v.Elem().Type()))
		}

		// No need to check types as it'll be checked by decodeMessageData
		raw, err = decodeMessageData(field.MsgType, raw, v.Interface())
		if err != nil {
			return nil, raw, err
		}
	}

	return vs.Interface(), raw, nil
}
