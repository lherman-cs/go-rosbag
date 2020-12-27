package rosbag

import (
	"math"
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
`)

	type Data struct {
		Bool    bool    `rosbag:"bool"`
		Int8    int8    `rosbag:"int8"`
		Uint8   uint8   `rosbag:"uint8"`
		Int16   int16   `rosbag:"int16"`
		Uint16  uint16  `rosbag:"uint16"`
		Int32   int32   `rosbag:"int32"`
		Uint32  uint32  `rosbag:"uint32"`
		Int64   int64   `rosbag:"int64"`
		Uint64  uint64  `rosbag:"uint64"`
		Float32 float32 `rosbag:"float32"`
		Float64 float64 `rosbag:"float64"`
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
	msgDataRaw = addData(msgDataRaw, expected.Float64)
	msgDataRaw = addData(msgDataRaw, expected.Float64)

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

	t.Log(data)
}