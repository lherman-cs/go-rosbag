package rosbag

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

type TestField struct {
	Name      string
	Value     interface{}
	ArraySize int
	Dynamic   bool
	Const     bool
}

func addData(b []byte, v interface{}) []byte {
	var buf []byte
	switch v := v.(type) {
	case bool:
		if v {
			buf = []byte{1}
		} else {
			buf = []byte{0}
		}
	case int8:
		buf = []byte{byte(v)}
	case uint8:
		buf = []byte{byte(v)}
	case int16:
		buf = make([]byte, 2)
		endian.PutUint16(buf, uint16(v))
	case uint16:
		buf = make([]byte, 2)
		endian.PutUint16(buf, v)
	case int32:
		buf = make([]byte, 4)
		endian.PutUint32(buf, uint32(v))
	case uint32:
		buf = make([]byte, 4)
		endian.PutUint32(buf, v)
	case int64:
		buf = make([]byte, 8)
		endian.PutUint64(buf, uint64(v))
	case uint64:
		buf = make([]byte, 8)
		endian.PutUint64(buf, v)
	case float32:
		buf = make([]byte, 4)
		endian.PutUint32(buf, math.Float32bits(v))
	case float64:
		buf = make([]byte, 8)
		endian.PutUint64(buf, math.Float64bits(v))
	case string:
		buf = make([]byte, 4+len(v))
		endian.PutUint32(buf, uint32(len(v)))
		copy(buf[4:], []byte(v))
	case time.Time:
		buf = make([]byte, 8)
		timestamp := v.UnixNano()
		sec := timestamp / int64(time.Second/time.Nanosecond)
		nsec := timestamp % int64(time.Second/time.Nanosecond)
		endian.PutUint32(buf, uint32(sec))
		endian.PutUint32(buf[4:], uint32(nsec))
	case time.Duration:
		buf = make([]byte, 8)
		sec := v / (time.Second / time.Nanosecond)
		nsec := v % (time.Second / time.Nanosecond)
		endian.PutUint32(buf, uint32(sec))
		endian.PutUint32(buf[4:], uint32(nsec))
	case []TestField:
		buf = convertFieldsToRaw(v)
	}

	return append(b, buf...)
}

func convertFieldsToRaw(fields []TestField) []byte {
	var raw []byte

	for _, field := range fields {
		if field.Const {
			continue
		}

		if field.ArraySize == 0 {
			raw = addData(raw, field.Value)
			continue
		}

		if field.Dynamic {
			raw = addData(raw, uint32(field.ArraySize))
		}

		reflectValue := reflect.ValueOf(field.Value)
		for i := 0; i < reflectValue.Len(); i++ {
			raw = addData(raw, reflectValue.Index(i).Interface())
		}
	}

	return raw
}

func convertFieldsToMap(fields []TestField) map[string]interface{} {
	m := make(map[string]interface{})

	for _, field := range fields {
		switch v := field.Value.(type) {
		case []TestField:
			m[field.Name] = convertFieldsToMap(v)
		case [][]TestField:
			arr := make([]map[string]interface{}, len(v))
			for i, item := range v {
				arr[i] = convertFieldsToMap(item)
			}
			m[field.Name] = arr
		default:
			m[field.Name] = field.Value
		}
	}

	return m
}

func TestDecodeMessageData(t *testing.T) {
	msgDefRaw := []byte(`
# This is an example comment
# Example can be anywhere

# Following is a list of singular types
bool bool# Comment can be next to the variable name
int8     int8 # Space should not matter in between the type and name
  uint8 uint8 # Initial space shouldn't matter either
int16 int16
uint16 uint16
int32 int32
uint32 uint32
int64 int64
uint64 uint64
float32 float32
float64 float64
string string
time time
duration duration
Person person

# Following is a list of dynamic slice types
bool[] boolSlice
int8[] int8Slice
uint8[] uint8Slice
int16[] int16Slice
uint16[] uint16Slice
int32[] int32Slice
uint32[] uint32Slice
int64[] int64Slice
uint64[] uint64Slice
float32[] float32Slice
float64[] float64Slice
string[] stringSlice
time[] timeSlice
duration[] durationSlice
Person[] personSlice

# Following is a list of fixed slice types
bool[2] boolArray
int8[2] int8Array
uint8[2] uint8Array
int16[2] int16Array
uint16[2] uint16Array
int32[2] int32Array
uint32[2] uint32Array
int64[2] int64Array
uint64[2] uint64Array
float32[3] float32Array
float64[3] float64Array
string[1] stringArray
time[2] timeArray
duration[2] durationArray
Person[2] personArray

Const const

  MSG: custom_msgs/Person # Message type should be parseable with a comment and a leading space
uint8 age


MSG: custom_msgs/Const
bool boolConst = 1
int8 int8Const = -1
uint8 uint8Const = 1
int16 int16Const = -1
uint16 uint16Const = 1
int32 int32Const = -1
uint32 uint32Const = 1
int64 int64Const = -1
uint64 uint64Const = 1
float32 float32Const = 0.123
float64 float64Const = 0.321
string stringConst  =  lukas herman# This comment should not be included in the string
`)

	/*


	 */

	expectedFields := []TestField{
		{Name: "bool", Value: true},
		{Name: "int8", Value: int8(math.MinInt8)},
		{Name: "uint8", Value: uint8(math.MaxUint8)},
		{Name: "int16", Value: int16(math.MinInt16)},
		{Name: "uint16", Value: uint16(math.MaxUint16)},
		{Name: "int32", Value: int32(math.MinInt32)},
		{Name: "uint32", Value: uint32(math.MaxUint32)},
		{Name: "int64", Value: int64(math.MinInt64)},
		{Name: "uint64", Value: uint64(math.MaxUint64)},
		{Name: "float32", Value: float32(math.MaxFloat32 / 10)},
		{Name: "float64", Value: float64(math.MaxFloat64 / 10)},
		{Name: "string", Value: "lukas"},
		{Name: "time", Value: time.Unix(1, 10)},
		{Name: "duration", Value: time.Second + time.Nanosecond},
		{Name: "person", Value: []TestField{
			{Name: "age", Value: uint8(24)},
		}},
		{Name: "boolSlice", Value: []bool{true, false}, ArraySize: 2, Dynamic: true},
		{Name: "int8Slice", Value: []int8{-1, 1}, ArraySize: 2, Dynamic: true},
		{Name: "uint8Slice", Value: []uint8{1, 2}, ArraySize: 2, Dynamic: true},
		{Name: "int16Slice", Value: []int16{-1, 1}, ArraySize: 2, Dynamic: true},
		{Name: "uint16Slice", Value: []uint16{1, 2}, ArraySize: 2, Dynamic: true},
		{Name: "int32Slice", Value: []int32{-1, 1}, ArraySize: 2, Dynamic: true},
		{Name: "uint32Slice", Value: []uint32{1, 2}, ArraySize: 2, Dynamic: true},
		{Name: "int64Slice", Value: []int64{-1, 1}, ArraySize: 2, Dynamic: true},
		{Name: "uint64Slice", Value: []uint64{1, 2}, ArraySize: 2, Dynamic: true},
		{Name: "float32Slice", Value: []float32{0.123, 0.3312, 0.111}, ArraySize: 3, Dynamic: true},
		{Name: "float64Slice", Value: []float64{-0.123, 0.3312, -0.111}, ArraySize: 3, Dynamic: true},
		{Name: "stringSlice", Value: []string{"lukas"}, ArraySize: 1, Dynamic: true},
		// 1,000,000,000,000
		{Name: "timeSlice", Value: []time.Time{time.Unix(10, 1000), time.Unix(1000, 2132131)}, ArraySize: 2, Dynamic: true},
		{Name: "durationSlice", Value: []time.Duration{time.Second, time.Microsecond}, ArraySize: 2, Dynamic: true},
		{Name: "personSlice", ArraySize: 2, Dynamic: true, Value: [][]TestField{
			{{Name: "age", Value: uint8(26)}},
			{{Name: "age", Value: uint8(100)}},
		}},
		{Name: "boolArray", Value: []bool{true, false}, ArraySize: 2},
		{Name: "int8Array", Value: []int8{-1, 1}, ArraySize: 2},
		{Name: "uint8Array", Value: []uint8{1, 2}, ArraySize: 2},
		{Name: "int16Array", Value: []int16{-1, 1}, ArraySize: 2},
		{Name: "uint16Array", Value: []uint16{1, 2}, ArraySize: 2},
		{Name: "int32Array", Value: []int32{-1, 1}, ArraySize: 2},
		{Name: "uint32Array", Value: []uint32{1, 2}, ArraySize: 2},
		{Name: "int64Array", Value: []int64{-1, 1}, ArraySize: 2},
		{Name: "uint64Array", Value: []uint64{1, 2}, ArraySize: 2},
		{Name: "float32Array", Value: []float32{0.123, 0.3312, 0.111}, ArraySize: 3},
		{Name: "float64Array", Value: []float64{-0.123, 0.3312, -0.111}, ArraySize: 3},
		{Name: "stringArray", Value: []string{"lukas"}, ArraySize: 1},
		// 1,000,000,000,000
		{Name: "timeArray", Value: []time.Time{time.Unix(10, 1000), time.Unix(1000, 2132131)}, ArraySize: 2},
		{Name: "durationArray", Value: []time.Duration{time.Second, time.Microsecond}, ArraySize: 2},
		{Name: "personArray", ArraySize: 2, Value: [][]TestField{
			{{Name: "age", Value: uint8(26)}},
			{{Name: "age", Value: uint8(100)}},
		}},
		{Name: "const", Value: []TestField{
			{Name: "boolConst", Value: true, Const: true},
			{Name: "int8Const", Value: int8(-1), Const: true},
			{Name: "uint8Const", Value: uint8(1), Const: true},
			{Name: "int16Const", Value: int16(-1), Const: true},
			{Name: "uint16Const", Value: uint16(1), Const: true},
			{Name: "int32Const", Value: int32(-1), Const: true},
			{Name: "uint32Const", Value: uint32(1), Const: true},
			{Name: "int64Const", Value: int64(-1), Const: true},
			{Name: "uint64Const", Value: uint64(1), Const: true},
			{Name: "float32Const", Value: float32(0.123), Const: true},
			{Name: "float64Const", Value: float64(0.321), Const: true},
			{Name: "stringConst", Value: "lukas herman", Const: true},
		}},
	}

	expectedMap := convertFieldsToMap(expectedFields)
	msgDataRaw := convertFieldsToRaw(expectedFields)

	var msgDef MessageDefinition
	err := msgDef.unmarshall(msgDefRaw)
	if err != nil {
		t.Fatal(err)
	}

	actualMap, _, err := decodeMessageData(&msgDef, msgDataRaw, getClearedComplexData)
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(expectedMap, actualMap); diff != "" {
		t.Fatal(diff)
	}
}

func BenchmarkDecodeMessageData(b *testing.B) {
	b.StopTimer()
	msgDefRaw := []byte(`
uint8[] pixels
`)

	res := 1920 * 1080
	var msgDataRaw []byte
	msgDataRaw = addData(msgDataRaw, uint32(res))
	for i := 0; i < res; i++ {
		msgDataRaw = addData(msgDataRaw, uint8(i))
	}

	var msgDef MessageDefinition
	err := msgDef.unmarshall(msgDefRaw)
	if err != nil {
		b.Fatal(err)
	}

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := decodeMessageData(&msgDef, msgDataRaw, getClearedComplexData)
		if err != nil {
			b.Fatal(err)
		}
	}
}
