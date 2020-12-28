package rosbag

import (
	"math"
	"reflect"
	"testing"
	"time"
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
	case time.Time:
		buf = make([]byte, 8)
		endian.PutUint32(buf, uint32(v.Second()))
		endian.PutUint32(buf[4:], uint32(v.Nanosecond()))
	case time.Duration:
		buf = make([]byte, 8)
		sec := v / time.Second
		nsec := v % (time.Second / time.Nanosecond)
		endian.PutUint32(buf, uint32(sec))
		endian.PutUint32(buf[4:], uint32(nsec))
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
time time
duration duration

MSG: custom_msgs/Person
uint8 age
`)

	type Person struct {
		Age uint8 `rosbag:"age"`
	}

	type Data struct {
		Bool     bool          `rosbag:"bool"`
		Int8     int8          `rosbag:"int8"`
		Uint8    uint8         `rosbag:"uint8"`
		Int16    int16         `rosbag:"int16"`
		Uint16   uint16        `rosbag:"uint16"`
		Int32    int32         `rosbag:"int32"`
		Uint32   uint32        `rosbag:"uint32"`
		Int64    int64         `rosbag:"int64"`
		Uint64   uint64        `rosbag:"uint64"`
		Float32  float32       `rosbag:"float32"`
		Float64  float64       `rosbag:"float64"`
		Person   Person        `rosbag:"person"`
		Pixel    []uint8       `rosbag:"pixel"`
		Children []Person      `rosbag:"children"`
		String   string        `rosbag:"string"`
		Time     time.Time     `rosbag:"time"`
		Duration time.Duration `rosbag:"duration"`
	}

	structExpected := Data{
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
		String:   "lukas",
		Time:     time.Unix(1, 10),
		Duration: time.Second + time.Nanosecond,
	}

	mapExpected := map[string]interface{}{
		"bool":    true,
		"int8":    structExpected.Int8,
		"uint8":   structExpected.Uint8,
		"int16":   structExpected.Int16,
		"uint16":  structExpected.Uint16,
		"int32":   structExpected.Int32,
		"uint32":  structExpected.Uint32,
		"int64":   structExpected.Int64,
		"uint64":  structExpected.Uint64,
		"float32": structExpected.Float32,
		"float64": structExpected.Float64,
		"person": map[string]interface{}{
			"age": structExpected.Person.Age,
		},
		"pixel": []interface{}{structExpected.Pixel[0], structExpected.Pixel[1], structExpected.Pixel[2]},
		"children": []interface{}{
			map[string]interface{}{"age": structExpected.Children[0].Age},
			map[string]interface{}{"age": structExpected.Children[1].Age},
		},
		"string":   structExpected.String,
		"time":     structExpected.Time,
		"duration": structExpected.Duration,
	}

	var msgDataRaw []byte
	msgDataRaw = addData(msgDataRaw, structExpected.Bool)
	msgDataRaw = addData(msgDataRaw, structExpected.Int8)
	msgDataRaw = addData(msgDataRaw, structExpected.Uint8)
	msgDataRaw = addData(msgDataRaw, structExpected.Int16)
	msgDataRaw = addData(msgDataRaw, structExpected.Uint16)
	msgDataRaw = addData(msgDataRaw, structExpected.Int32)
	msgDataRaw = addData(msgDataRaw, structExpected.Uint32)
	msgDataRaw = addData(msgDataRaw, structExpected.Int64)
	msgDataRaw = addData(msgDataRaw, structExpected.Uint64)
	msgDataRaw = addData(msgDataRaw, structExpected.Float32)
	msgDataRaw = addData(msgDataRaw, structExpected.Float64)
	msgDataRaw = addData(msgDataRaw, structExpected.Person.Age)
	msgDataRaw = addData(msgDataRaw, structExpected.Pixel[0])
	msgDataRaw = addData(msgDataRaw, structExpected.Pixel[1])
	msgDataRaw = addData(msgDataRaw, structExpected.Pixel[2])
	msgDataRaw = addData(msgDataRaw, uint32(2))
	msgDataRaw = addData(msgDataRaw, structExpected.Children[0].Age)
	msgDataRaw = addData(msgDataRaw, structExpected.Children[1].Age)
	msgDataRaw = addData(msgDataRaw, structExpected.String)
	msgDataRaw = addData(msgDataRaw, structExpected.Time)
	msgDataRaw = addData(msgDataRaw, structExpected.Duration)

	var msgDef MessageDefinition
	err := msgDef.unmarshall(msgDefRaw)
	if err != nil {
		t.Fatal(err)
	}

	mapData := make(map[string]interface{})
	err = decodeMessageData(&msgDef, msgDataRaw, mapData)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(mapExpected, mapData) {
		t.Fatalf("invalid parsed data.\n\nExpected:\n\n%#v\n\nActual:\n\n%#v", mapExpected, mapData)
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
		data := make(map[string]interface{})
		err = decodeMessageData(&msgDef, msgDataRaw, data)
		if err != nil {
			b.Fatal(err)
		}
	}
}
