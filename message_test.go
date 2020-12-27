package rosbag

import (
	"math"
	"reflect"
	"testing"
)

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
	}

	return append(b, buf...)
}

func TestDecodeMessageData(t *testing.T) {
	msgDefRaw := []byte(`
bool bool 
int8 int8
uint8 uint8
int16 int16
uint16 uint16
int32 int32
uint32 uint32
int64 int64
uint64 uint64
float32 float32
float64 float64
Person person
uint8[3] pixel
Person[] children
string string

MSG: custom_msgs/Person
uint8 age
`)

	type Person struct {
		Age uint8 `rosbag:"age"`
	}

	type Data struct {
		Bool     bool     `rosbag:"bool"`
		Int8     int8     `rosbag:"int8"`
		Uint8    uint8    `rosbag:"uint8"`
		Int16    int16    `rosbag:"int16"`
		Uint16   uint16   `rosbag:"uint16"`
		Int32    int32    `rosbag:"int32"`
		Uint32   uint32   `rosbag:"uint32"`
		Int64    int64    `rosbag:"int64"`
		Uint64   uint64   `rosbag:"uint64"`
		Float32  float32  `rosbag:"float32"`
		Float64  float64  `rosbag:"float64"`
		Person   Person   `rosbag:"person"`
		Pixel    []uint8  `rosbag:"pixel"`
		Children []Person `rosbag:"children"`
		String   string   `rosbag:"string"`
	}

	expected := Data{
		Bool:    true,
		Int8:    math.MinInt8,
		Uint8:   math.MaxUint8,
		Int16:   math.MinInt16,
		Uint16:  math.MaxUint16,
		Int32:   math.MinInt32,
		Uint32:  math.MaxUint32,
		Int64:   math.MinInt64,
		Uint64:  math.MaxUint64,
		Float32: math.MaxFloat32 / 10,
		Float64: math.MaxFloat64 / 10,
		Person: Person{
			Age: 24,
		},
		Pixel: []uint8{1, 2, 3},
		Children: []Person{
			{Age: 20},
			{Age: 15},
		},
		String: "lukas",
	}

	var msgDataRaw []byte
	msgDataRaw = addData(msgDataRaw, expected.Bool)
	msgDataRaw = addData(msgDataRaw, expected.Int8)
	msgDataRaw = addData(msgDataRaw, expected.Uint8)
	msgDataRaw = addData(msgDataRaw, expected.Int16)
	msgDataRaw = addData(msgDataRaw, expected.Uint16)
	msgDataRaw = addData(msgDataRaw, expected.Int32)
	msgDataRaw = addData(msgDataRaw, expected.Uint32)
	msgDataRaw = addData(msgDataRaw, expected.Int64)
	msgDataRaw = addData(msgDataRaw, expected.Uint64)
	msgDataRaw = addData(msgDataRaw, expected.Float32)
	msgDataRaw = addData(msgDataRaw, expected.Float64)
	msgDataRaw = addData(msgDataRaw, expected.Person.Age)
	msgDataRaw = addData(msgDataRaw, expected.Pixel[0])
	msgDataRaw = addData(msgDataRaw, expected.Pixel[1])
	msgDataRaw = addData(msgDataRaw, expected.Pixel[2])
	msgDataRaw = addData(msgDataRaw, uint32(2))
	msgDataRaw = addData(msgDataRaw, expected.Children[0].Age)
	msgDataRaw = addData(msgDataRaw, expected.Children[1].Age)
	msgDataRaw = addData(msgDataRaw, expected.String)

	var msgDef MessageDefinition
	err := msgDef.unmarshall(msgDefRaw)
	if err != nil {
		t.Fatal(err)
	}

	var data Data
	err = decodeMessageData(&msgDef, msgDataRaw, &data)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(&expected, &data) {
		t.Fatalf("invalid parsed data.\n\nExpected:\n\n%+v\n\nActual:\n\n%+v", expected, data)
	}
}
