package rosbag

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	fuzz "github.com/google/gofuzz"
)

func fuzzDuration(fuzzer *fuzz.Fuzzer) time.Duration {
	// ROS uses uint32 to represent sec and nsec, so we need to tell fuzzer these boundaries
	var sec, nsec uint32
	fuzzer.Fuzz(&sec)
	fuzzer.Fuzz(&nsec)
	return time.Second*time.Duration(sec) + time.Nanosecond*time.Duration(nsec)
}

func fuzzDurationSlice(fuzzer *fuzz.Fuzzer) []time.Duration {
	var length uint32
	fuzzer.Fuzz(&length)
	length %= 100 // make sure we don't blow up the length

	s := make([]time.Duration, length)
	for i := range s {
		s[i] = fuzzDuration(fuzzer)
	}
	return s
}

func fuzzTime(fuzzer *fuzz.Fuzzer) time.Time {
	// ROS uses uint32 to represent sec and nsec, so we need to tell fuzzer these boundaries.
	// This also means that there's no time before UNIX epoch in ROS.
	var sec, nsec uint32
	fuzzer.Fuzz(&sec)
	fuzzer.Fuzz(&nsec)
	return time.Unix(int64(sec), int64(nsec))
}

func fuzzTimeSlice(fuzzer *fuzz.Fuzzer) []time.Time {
	var length uint32
	fuzzer.Fuzz(&length)
	length %= 100 // make sure we don't blow up the length

	s := make([]time.Time, length)
	for i := range s {
		s[i] = fuzzTime(fuzzer)
	}
	return s
}

type Marshallable interface {
	Marshall() []byte
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
	case Marshallable:
		buf = v.Marshall()
	}

	return append(b, buf...)
}

func addDataMulti(b []byte, v interface{}, isSlice bool) []byte {
	value := reflect.ValueOf(v)
	length := value.Len()

	if isSlice {
		b = addData(b, uint32(length))
	}

	for i := 0; i < length; i++ {
		b = addData(b, value.Index(i).Interface())
	}

	return b
}

type Object struct {
	Name string `rosbag:"name"`
	Age  uint32 `rosbag:"age"`
}

func (o *Object) Marshall() []byte {
	raw := addData(nil, o.Name)
	raw = addData(raw, o.Age)
	return raw
}

func (o *Object) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"name": o.Name,
		"age":  o.Age,
	}
}

type Expected struct {
	Struct interface{}
	Map    map[string]interface{}
}

func TestDecodeMessageData(t *testing.T) {
	type TestCase struct {
		Name     string
		MsgDef   string
		Expected func(fuzzer *fuzz.Fuzzer) ([]byte, interface{}, Expected)
	}

	testCases := []TestCase{
		{
			Name:   "SingleBool",
			MsgDef: "bool bool",
			Expected: func(fuzzer *fuzz.Fuzzer) ([]byte, interface{}, Expected) {
				s := struct {
					Bool bool `rosbag:"bool"`
				}{}
				fuzzer.Fuzz(&s)

				m := map[string]interface{}{
					"bool": s.Bool,
				}
				a := s
				return addData(nil, s.Bool), &a, Expected{
					Struct: &s,
					Map:    m,
				}
			},
		},
		{
			Name:   "SingleInt8",
			MsgDef: "int8 int8",
			Expected: func(fuzzer *fuzz.Fuzzer) ([]byte, interface{}, Expected) {
				s := struct {
					Int8 int8 `rosbag:"int8"`
				}{}
				fuzzer.Fuzz(&s)

				m := map[string]interface{}{
					"int8": s.Int8,
				}
				a := s
				return addData(nil, s.Int8), &a, Expected{
					Struct: &s,
					Map:    m,
				}
			},
		},
		{
			Name:   "SingleUint8",
			MsgDef: "uint8 uint8",
			Expected: func(fuzzer *fuzz.Fuzzer) ([]byte, interface{}, Expected) {
				s := struct {
					Uint8 uint8 `rosbag:"uint8"`
				}{}
				fuzzer.Fuzz(&s)

				m := map[string]interface{}{
					"uint8": s.Uint8,
				}
				a := s
				return addData(nil, s.Uint8), &a, Expected{
					Struct: &s,
					Map:    m,
				}
			},
		},
		{
			Name:   "SingleInt16",
			MsgDef: "int16 int16",
			Expected: func(fuzzer *fuzz.Fuzzer) ([]byte, interface{}, Expected) {
				s := struct {
					Int16 int16 `rosbag:"int16"`
				}{}
				fuzzer.Fuzz(&s)

				m := map[string]interface{}{
					"int16": s.Int16,
				}
				a := s
				return addData(nil, s.Int16), &a, Expected{
					Struct: &s,
					Map:    m,
				}
			},
		},
		{
			Name:   "SingleUint16",
			MsgDef: "uint16 uint16",
			Expected: func(fuzzer *fuzz.Fuzzer) ([]byte, interface{}, Expected) {
				s := struct {
					Uint16 uint16 `rosbag:"uint16"`
				}{}
				fuzzer.Fuzz(&s)

				m := map[string]interface{}{
					"uint16": s.Uint16,
				}
				a := s
				return addData(nil, s.Uint16), &a, Expected{
					Struct: &s,
					Map:    m,
				}
			},
		},
		{
			Name:   "SingleInt32",
			MsgDef: "int32 int32",
			Expected: func(fuzzer *fuzz.Fuzzer) ([]byte, interface{}, Expected) {
				s := struct {
					Int32 int32 `rosbag:"int32"`
				}{}
				fuzzer.Fuzz(&s)

				m := map[string]interface{}{
					"int32": s.Int32,
				}
				a := s
				return addData(nil, s.Int32), &a, Expected{
					Struct: &s,
					Map:    m,
				}
			},
		},
		{
			Name:   "SingleUint32",
			MsgDef: "uint32 uint32",
			Expected: func(fuzzer *fuzz.Fuzzer) ([]byte, interface{}, Expected) {
				s := struct {
					Uint32 uint32 `rosbag:"uint32"`
				}{}
				fuzzer.Fuzz(&s)

				m := map[string]interface{}{
					"uint32": s.Uint32,
				}
				a := s
				return addData(nil, s.Uint32), &a, Expected{
					Struct: &s,
					Map:    m,
				}
			},
		},
		{
			Name:   "SingleInt64",
			MsgDef: "int64 int64",
			Expected: func(fuzzer *fuzz.Fuzzer) ([]byte, interface{}, Expected) {
				s := struct {
					Int64 int64 `rosbag:"int64"`
				}{}
				fuzzer.Fuzz(&s)

				m := map[string]interface{}{
					"int64": s.Int64,
				}
				a := s
				return addData(nil, s.Int64), &a, Expected{
					Struct: &s,
					Map:    m,
				}
			},
		},
		{
			Name:   "SingleUint64",
			MsgDef: "uint64 uint64",
			Expected: func(fuzzer *fuzz.Fuzzer) ([]byte, interface{}, Expected) {
				s := struct {
					Uint64 uint64 `rosbag:"uint64"`
				}{}
				fuzzer.Fuzz(&s)

				m := map[string]interface{}{
					"uint64": s.Uint64,
				}
				a := s
				return addData(nil, s.Uint64), &a, Expected{
					Struct: &s,
					Map:    m,
				}
			},
		},
		{
			Name:   "SingleFloat32",
			MsgDef: "float32 float32",
			Expected: func(fuzzer *fuzz.Fuzzer) ([]byte, interface{}, Expected) {
				s := struct {
					Float32 float32 `rosbag:"float32"`
				}{}
				fuzzer.Fuzz(&s)

				m := map[string]interface{}{
					"float32": s.Float32,
				}
				a := s
				return addData(nil, s.Float32), &a, Expected{
					Struct: &s,
					Map:    m,
				}
			},
		},
		{
			Name:   "SingleFloat64",
			MsgDef: "float64 float64",
			Expected: func(fuzzer *fuzz.Fuzzer) ([]byte, interface{}, Expected) {
				s := struct {
					Float64 float64 `rosbag:"float64"`
				}{}
				fuzzer.Fuzz(&s)

				m := map[string]interface{}{
					"float64": s.Float64,
				}
				a := s
				return addData(nil, s.Float64), &a, Expected{
					Struct: &s,
					Map:    m,
				}
			},
		},
		{
			Name:   "SingleString",
			MsgDef: "string string",
			Expected: func(fuzzer *fuzz.Fuzzer) ([]byte, interface{}, Expected) {
				s := struct {
					String string `rosbag:"string"`
				}{}
				fuzzer.Fuzz(&s)

				m := map[string]interface{}{
					"string": s.String,
				}
				a := s
				return addData(nil, s.String), &a, Expected{
					Struct: &s,
					Map:    m,
				}
			},
		},
		{
			Name:   "SingleTime",
			MsgDef: "time time",
			Expected: func(fuzzer *fuzz.Fuzzer) ([]byte, interface{}, Expected) {
				s := struct {
					Time time.Time `rosbag:"time"`
				}{}
				s.Time = fuzzTime(fuzzer)

				m := map[string]interface{}{
					"time": s.Time,
				}
				a := s
				return addData(nil, s.Time), &a, Expected{
					Struct: &s,
					Map:    m,
				}
			},
		},
		{
			Name:   "SingleDuration",
			MsgDef: "duration duration",
			Expected: func(fuzzer *fuzz.Fuzzer) ([]byte, interface{}, Expected) {
				s := struct {
					Duration time.Duration `rosbag:"duration"`
				}{}
				s.Duration = fuzzDuration(fuzzer)

				m := map[string]interface{}{
					"duration": s.Duration,
				}
				a := s
				return addData(nil, s.Duration), &a, Expected{
					Struct: &s,
					Map:    m,
				}
			},
		},
		{
			Name: "SingleObject",
			MsgDef: `
			object object

			MSG: custom_msgs/object
			string name
			uint32 age
			`,
			Expected: func(fuzzer *fuzz.Fuzzer) ([]byte, interface{}, Expected) {
				s := struct {
					Object Object `rosbag:"object"`
				}{}
				fuzzer.Fuzz(&s)

				m := map[string]interface{}{
					"object": s.Object.ToMap(),
				}
				a := s
				return addData(nil, &s.Object), &a, Expected{
					Struct: &s,
					Map:    m,
				}
			},
		},
		{
			Name:   "SliceBool",
			MsgDef: "bool[] bool",
			Expected: func(fuzzer *fuzz.Fuzzer) ([]byte, interface{}, Expected) {
				s := struct {
					Bool []bool `rosbag:"bool"`
				}{}
				fuzzer.Fuzz(&s)

				m := map[string]interface{}{
					"bool": s.Bool,
				}
				a := s
				return addDataMulti(nil, s.Bool, true), &a, Expected{
					Struct: &s,
					Map:    m,
				}
			},
		},
		{
			Name:   "SliceInt8",
			MsgDef: "int8[] int8",
			Expected: func(fuzzer *fuzz.Fuzzer) ([]byte, interface{}, Expected) {
				s := struct {
					Int8 []int8 `rosbag:"int8"`
				}{}
				fuzzer.Fuzz(&s)

				m := map[string]interface{}{
					"int8": s.Int8,
				}
				a := s
				return addDataMulti(nil, s.Int8, true), &a, Expected{
					Struct: &s,
					Map:    m,
				}
			},
		},
		{
			Name:   "SliceUint8",
			MsgDef: "uint8[] uint8",
			Expected: func(fuzzer *fuzz.Fuzzer) ([]byte, interface{}, Expected) {
				s := struct {
					Uint8 []uint8 `rosbag:"uint8"`
				}{}
				fuzzer.Fuzz(&s)

				m := map[string]interface{}{
					"uint8": s.Uint8,
				}
				a := s
				return addDataMulti(nil, s.Uint8, true), &a, Expected{
					Struct: &s,
					Map:    m,
				}
			},
		},
		{
			Name:   "SliceInt16",
			MsgDef: "int16[] int16",
			Expected: func(fuzzer *fuzz.Fuzzer) ([]byte, interface{}, Expected) {
				s := struct {
					Int16 []int16 `rosbag:"int16"`
				}{}
				fuzzer.Fuzz(&s)

				m := map[string]interface{}{
					"int16": s.Int16,
				}
				a := s
				return addDataMulti(nil, s.Int16, true), &a, Expected{
					Struct: &s,
					Map:    m,
				}
			},
		},
		{
			Name:   "SliceUint16",
			MsgDef: "uint16[] uint16",
			Expected: func(fuzzer *fuzz.Fuzzer) ([]byte, interface{}, Expected) {
				s := struct {
					Uint16 []uint16 `rosbag:"uint16"`
				}{}
				fuzzer.Fuzz(&s)

				m := map[string]interface{}{
					"uint16": s.Uint16,
				}
				a := s
				return addDataMulti(nil, s.Uint16, true), &a, Expected{
					Struct: &s,
					Map:    m,
				}
			},
		},
		{
			Name:   "SliceInt32",
			MsgDef: "int32[] int32",
			Expected: func(fuzzer *fuzz.Fuzzer) ([]byte, interface{}, Expected) {
				s := struct {
					Int32 []int32 `rosbag:"int32"`
				}{}
				fuzzer.Fuzz(&s)

				m := map[string]interface{}{
					"int32": s.Int32,
				}
				a := s
				return addDataMulti(nil, s.Int32, true), &a, Expected{
					Struct: &s,
					Map:    m,
				}
			},
		},
		{
			Name:   "SliceUint32",
			MsgDef: "uint32[] uint32",
			Expected: func(fuzzer *fuzz.Fuzzer) ([]byte, interface{}, Expected) {
				s := struct {
					Uint32 []uint32 `rosbag:"uint32"`
				}{}
				fuzzer.Fuzz(&s)

				m := map[string]interface{}{
					"uint32": s.Uint32,
				}
				a := s
				return addDataMulti(nil, s.Uint32, true), &a, Expected{
					Struct: &s,
					Map:    m,
				}
			},
		},
		{
			Name:   "SliceInt64",
			MsgDef: "int64[] int64",
			Expected: func(fuzzer *fuzz.Fuzzer) ([]byte, interface{}, Expected) {
				s := struct {
					Int64 []int64 `rosbag:"int64"`
				}{}
				fuzzer.Fuzz(&s)

				m := map[string]interface{}{
					"int64": s.Int64,
				}
				a := s
				return addDataMulti(nil, s.Int64, true), &a, Expected{
					Struct: &s,
					Map:    m,
				}
			},
		},
		{
			Name:   "SliceUint64",
			MsgDef: "uint64[] uint64",
			Expected: func(fuzzer *fuzz.Fuzzer) ([]byte, interface{}, Expected) {
				s := struct {
					Uint64 []uint64 `rosbag:"uint64"`
				}{}
				fuzzer.Fuzz(&s)

				m := map[string]interface{}{
					"uint64": s.Uint64,
				}
				a := s
				return addDataMulti(nil, s.Uint64, true), &a, Expected{
					Struct: &s,
					Map:    m,
				}
			},
		},
		{
			Name:   "SliceFloat32",
			MsgDef: "float32[] float32",
			Expected: func(fuzzer *fuzz.Fuzzer) ([]byte, interface{}, Expected) {
				s := struct {
					Float32 []float32 `rosbag:"float32"`
				}{}
				fuzzer.Fuzz(&s)

				m := map[string]interface{}{
					"float32": s.Float32,
				}
				a := s
				return addDataMulti(nil, s.Float32, true), &a, Expected{
					Struct: &s,
					Map:    m,
				}
			},
		},
		{
			Name:   "SliceFloat64",
			MsgDef: "float64[] float64",
			Expected: func(fuzzer *fuzz.Fuzzer) ([]byte, interface{}, Expected) {
				s := struct {
					Float64 []float64 `rosbag:"float64"`
				}{}
				fuzzer.Fuzz(&s)

				m := map[string]interface{}{
					"float64": s.Float64,
				}
				a := s
				return addDataMulti(nil, s.Float64, true), &a, Expected{
					Struct: &s,
					Map:    m,
				}
			},
		},
		{
			Name:   "SliceString",
			MsgDef: "string[] string",
			Expected: func(fuzzer *fuzz.Fuzzer) ([]byte, interface{}, Expected) {
				s := struct {
					String []string `rosbag:"string"`
				}{}
				fuzzer.Fuzz(&s)

				m := map[string]interface{}{
					"string": s.String,
				}
				a := s
				return addDataMulti(nil, s.String, true), &a, Expected{
					Struct: &s,
					Map:    m,
				}
			},
		},
		{
			Name:   "SliceTime",
			MsgDef: "time[] time",
			Expected: func(fuzzer *fuzz.Fuzzer) ([]byte, interface{}, Expected) {
				s := struct {
					Time []time.Time `rosbag:"time"`
				}{}
				s.Time = fuzzTimeSlice(fuzzer)

				m := map[string]interface{}{
					"time": s.Time,
				}
				a := s
				return addDataMulti(nil, s.Time, true), &a, Expected{
					Struct: &s,
					Map:    m,
				}
			},
		},
		{
			Name:   "SliceDuration",
			MsgDef: "duration[] duration",
			Expected: func(fuzzer *fuzz.Fuzzer) ([]byte, interface{}, Expected) {
				s := struct {
					Duration []time.Duration `rosbag:"duration"`
				}{}
				s.Duration = fuzzDurationSlice(fuzzer)

				m := map[string]interface{}{
					"duration": s.Duration,
				}
				a := s
				return addDataMulti(nil, s.Duration, true), &a, Expected{
					Struct: &s,
					Map:    m,
				}
			},
		},
		/*{
			Name: "SliceObject",
			MsgDef: `
			object[] object

			MSG: custom_msgs/object
			string name
			uint32 age
			`,
			Expected: func(fuzzer *fuzz.Fuzzer) ([]byte, interface{}, Expected) {
				s := struct {
					Object []Object `rosbag:"object"`
				}{}
				fuzzer.Fuzz(&s)

				slice := make([]map[string]interface{}, len(s.Object))
				for i, v := range s.Object {
					slice[i] = v.ToMap()
				}
				m := map[string]interface{}{
					"object": slice,
				}
				a := s
				return addDataMulti(nil, s.Object, true), &a, Expected{
					Struct: &s,
					Map:    m,
				}
			},
		},*/
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			fuzzer := fuzz.New()
			for i := 0; i < 1000; i++ {
				var msgDef MessageDefinition
				err := msgDef.unmarshall([]byte(testCase.MsgDef))
				if err != nil {
					t.Fatal(err)
				}

				raw, actualStruct, expected := testCase.Expected(fuzzer)
				rawAfter, err := decodeMessageData(&msgDef, raw, actualStruct)
				if err != nil {
					t.Fatal(err)
				}

				if len(rawAfter) != 0 {
					t.Fatalf("[Struct] Expected no buffer left after decoding the whole message, but got %v", rawAfter)
				}

				if diff := cmp.Diff(expected.Struct, actualStruct, cmpopts.EquateEmpty()); diff != "" {
					t.Fatalf("[Struct] Decoded value is not matched:\n\n%s", diff)
				}

				actualMap := make(map[string]interface{})
				rawAfter, err = decodeMessageData(&msgDef, raw, actualMap)
				if err != nil {
					t.Fatal(err)
				}

				if len(rawAfter) != 0 {
					t.Fatalf("[Map] Expected no buffer left after decoding the whole message, but got %v", rawAfter)
				}

				if diff := cmp.Diff(expected.Struct, actualStruct, cmpopts.EquateEmpty()); diff != "" {
					t.Fatalf("[Map] Decoded value is not matched:\n\n%s", diff)
				}
			}
		})
	}
}
