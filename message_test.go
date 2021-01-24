package rosbag

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	fuzz "github.com/google/gofuzz"
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

	type Const struct {
		BoolConst    bool    `rosbag:"boolConst"`
		Int8Const    int8    `rosbag:"int8Const"`
		Uint8Const   uint8   `rosbag:"uint8Const"`
		Int16Const   int16   `rosbag:"int16Const"`
		Uint16Const  uint16  `rosbag:"uint16Const"`
		Int32Const   int32   `rosbag:"int32Const"`
		Uint32Const  uint32  `rosbag:"uint32Const"`
		Int64Const   int64   `rosbag:"int64Const"`
		Uint64Const  uint64  `rosbag:"uint64Const"`
		Float32Const float32 `rosbag:"float32Const"`
		Float64Const float64 `rosbag:"float64Const"`
		StringConst  string  `rosbag:"stringConst"`
	}

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
		String   string        `rosbag:"string"`
		Time     time.Time     `rosbag:"time"`
		Duration time.Duration `rosbag:"duration"`
		Person   Person        `rosbag:"person"`

		BoolSlice     []bool          `rosbag:"boolSlice"`
		Int8Slice     []int8          `rosbag:"int8Slice"`
		Uint8Slice    []uint8         `rosbag:"uint8Slice"`
		Int16Slice    []int16         `rosbag:"int16Slice"`
		Uint16Slice   []uint16        `rosbag:"uint16Slice"`
		Int32Slice    []int32         `rosbag:"int32Slice"`
		Uint32Slice   []uint32        `rosbag:"uint32Slice"`
		Int64Slice    []int64         `rosbag:"int64Slice"`
		Uint64Slice   []uint64        `rosbag:"uint64Slice"`
		Float32Slice  []float32       `rosbag:"float32Slice"`
		Float64Slice  []float64       `rosbag:"float64Slice"`
		StringSlice   []string        `rosbag:"stringSlice"`
		TimeSlice     []time.Time     `rosbag:"timeSlice"`
		DurationSlice []time.Duration `rosbag:"durationSlice"`
		PersonSlice   []Person        `rosbag:"personSlice"`

		BoolArray     []bool          `rosbag:"boolArray"`
		Int8Array     []int8          `rosbag:"int8Array"`
		Uint8Array    []uint8         `rosbag:"uint8Array"`
		Int16Array    []int16         `rosbag:"int16Array"`
		Uint16Array   []uint16        `rosbag:"uint16Array"`
		Int32Array    []int32         `rosbag:"int32Array"`
		Uint32Array   []uint32        `rosbag:"uint32Array"`
		Int64Array    []int64         `rosbag:"int64Array"`
		Uint64Array   []uint64        `rosbag:"uint64Array"`
		Float32Array  []float32       `rosbag:"float32Array"`
		Float64Array  []float64       `rosbag:"float64Array"`
		StringArray   []string        `rosbag:"stringArray"`
		TimeArray     []time.Time     `rosbag:"timeArray"`
		DurationArray []time.Duration `rosbag:"durationArray"`
		PersonArray   []Person        `rosbag:"personArray"`

		Const Const `rosbag:"const"`
	}

	expectedStruct := Data{
		Bool:     true,
		Int8:     math.MinInt8,
		Uint8:    math.MaxUint8,
		Int16:    math.MinInt16,
		Uint16:   math.MaxUint16,
		Int32:    math.MinInt32,
		Uint32:   math.MaxInt32,
		Int64:    math.MinInt64,
		Uint64:   math.MaxUint64,
		Float32:  math.MaxFloat32 / 10,
		Float64:  math.MaxFloat64 / 10,
		String:   "lukas",
		Time:     time.Unix(1, 10),
		Duration: time.Second + time.Nanosecond,
		Person: Person{
			Age: 24,
		},
		BoolSlice:     []bool{true, false},
		Int8Slice:     []int8{-1, 1},
		Uint8Slice:    []uint8{1, 2},
		Int16Slice:    []int16{-1, 1},
		Uint16Slice:   []uint16{1, 2},
		Int32Slice:    []int32{-1, 1},
		Uint32Slice:   []uint32{1, 2},
		Int64Slice:    []int64{-1, 1},
		Uint64Slice:   []uint64{1, 2},
		Float32Slice:  []float32{0.123, 0.3312, 0.111},
		Float64Slice:  []float64{-0.123, 0.3312, -0.111},
		StringSlice:   []string{"lukas"},
		TimeSlice:     []time.Time{time.Unix(10, 1000), time.Unix(1000, 2132131)},
		DurationSlice: []time.Duration{time.Second - time.Millisecond, time.Microsecond + time.Nanosecond},
		PersonSlice: []Person{
			{Age: 26},
			{Age: 100},
		},
		BoolArray:     []bool{true, false},
		Int8Array:     []int8{-1, 1},
		Uint8Array:    []uint8{1, 2},
		Int16Array:    []int16{-1, 1},
		Uint16Array:   []uint16{1, 2},
		Int32Array:    []int32{-1, 1},
		Uint32Array:   []uint32{1, 2},
		Int64Array:    []int64{-1, 1},
		Uint64Array:   []uint64{1, 2},
		Float32Array:  []float32{0.123, 0.3312, 0.111},
		Float64Array:  []float64{-0.123, 0.3312, -0.111},
		StringArray:   []string{"lukas"},
		TimeArray:     []time.Time{time.Unix(10, 1000), time.Unix(1000, 2132131)},
		DurationArray: []time.Duration{time.Second - time.Millisecond, time.Microsecond + time.Nanosecond},
		PersonArray: []Person{
			{Age: 26},
			{Age: 100},
		},
		Const: Const{
			BoolConst:    true,
			Int8Const:    int8(-1),
			Uint8Const:   uint8(1),
			Int16Const:   int16(-1),
			Uint16Const:  uint16(1),
			Int32Const:   int32(-1),
			Uint32Const:  uint32(1),
			Int64Const:   int64(-1),
			Uint64Const:  uint64(1),
			Float32Const: float32(0.123),
			Float64Const: float64(0.321),
			StringConst:  "lukas herman",
		},
	}

	expectedFields := []TestField{
		{Name: "bool", Value: expectedStruct.Bool},
		{Name: "int8", Value: expectedStruct.Int8},
		{Name: "uint8", Value: expectedStruct.Uint8},
		{Name: "int16", Value: expectedStruct.Int16},
		{Name: "uint16", Value: expectedStruct.Uint16},
		{Name: "int32", Value: expectedStruct.Int32},
		{Name: "uint32", Value: expectedStruct.Uint32},
		{Name: "int64", Value: expectedStruct.Int64},
		{Name: "uint64", Value: expectedStruct.Uint64},
		{Name: "float32", Value: expectedStruct.Float32},
		{Name: "float64", Value: expectedStruct.Float64},
		{Name: "string", Value: expectedStruct.String},
		{Name: "time", Value: expectedStruct.Time},
		{Name: "duration", Value: expectedStruct.Duration},
		{Name: "person", Value: []TestField{
			{Name: "age", Value: expectedStruct.Person.Age},
		}},
		{Name: "boolSlice", Value: expectedStruct.BoolSlice, ArraySize: len(expectedStruct.BoolSlice), Dynamic: true},
		{Name: "int8Slice", Value: expectedStruct.Int8Slice, ArraySize: len(expectedStruct.Int8Slice), Dynamic: true},
		{Name: "uint8Slice", Value: expectedStruct.Uint8Slice, ArraySize: len(expectedStruct.Uint8Slice), Dynamic: true},
		{Name: "int16Slice", Value: expectedStruct.Int16Slice, ArraySize: len(expectedStruct.Int16Slice), Dynamic: true},
		{Name: "uint16Slice", Value: expectedStruct.Uint16Slice, ArraySize: len(expectedStruct.Uint16Slice), Dynamic: true},
		{Name: "int32Slice", Value: expectedStruct.Int32Slice, ArraySize: len(expectedStruct.Int32Slice), Dynamic: true},
		{Name: "uint32Slice", Value: expectedStruct.Uint32Slice, ArraySize: len(expectedStruct.Int32Slice), Dynamic: true},
		{Name: "int64Slice", Value: expectedStruct.Int64Slice, ArraySize: len(expectedStruct.Int64Slice), Dynamic: true},
		{Name: "uint64Slice", Value: expectedStruct.Uint64Slice, ArraySize: len(expectedStruct.Uint64Slice), Dynamic: true},
		{Name: "float32Slice", Value: expectedStruct.Float32Slice, ArraySize: len(expectedStruct.Float32Slice), Dynamic: true},
		{Name: "float64Slice", Value: expectedStruct.Float64Slice, ArraySize: len(expectedStruct.Float64Slice), Dynamic: true},
		{Name: "stringSlice", Value: expectedStruct.StringSlice, ArraySize: len(expectedStruct.StringSlice), Dynamic: true},
		{Name: "timeSlice", Value: expectedStruct.TimeSlice, ArraySize: len(expectedStruct.TimeSlice), Dynamic: true},
		{Name: "durationSlice", Value: expectedStruct.DurationSlice, ArraySize: len(expectedStruct.DurationSlice), Dynamic: true},
		{Name: "personSlice", ArraySize: len(expectedStruct.PersonSlice), Dynamic: true, Value: [][]TestField{
			{{Name: "age", Value: expectedStruct.PersonSlice[0].Age}},
			{{Name: "age", Value: expectedStruct.PersonSlice[1].Age}},
		}},
		{Name: "boolArray", Value: expectedStruct.BoolArray, ArraySize: len(expectedStruct.BoolArray), Dynamic: false},
		{Name: "int8Array", Value: expectedStruct.Int8Array, ArraySize: len(expectedStruct.Int8Array), Dynamic: false},
		{Name: "uint8Array", Value: expectedStruct.Uint8Array, ArraySize: len(expectedStruct.Uint8Array), Dynamic: false},
		{Name: "int16Array", Value: expectedStruct.Int16Array, ArraySize: len(expectedStruct.Int16Array), Dynamic: false},
		{Name: "uint16Array", Value: expectedStruct.Uint16Array, ArraySize: len(expectedStruct.Uint16Array), Dynamic: false},
		{Name: "int32Array", Value: expectedStruct.Int32Array, ArraySize: len(expectedStruct.Int32Array), Dynamic: false},
		{Name: "uint32Array", Value: expectedStruct.Uint32Array, ArraySize: len(expectedStruct.Int32Array), Dynamic: false},
		{Name: "int64Array", Value: expectedStruct.Int64Array, ArraySize: len(expectedStruct.Int64Array), Dynamic: false},
		{Name: "uint64Array", Value: expectedStruct.Uint64Array, ArraySize: len(expectedStruct.Uint64Array), Dynamic: false},
		{Name: "float32Array", Value: expectedStruct.Float32Array, ArraySize: len(expectedStruct.Float32Array), Dynamic: false},
		{Name: "float64Array", Value: expectedStruct.Float64Array, ArraySize: len(expectedStruct.Float64Array), Dynamic: false},
		{Name: "stringArray", Value: expectedStruct.StringArray, ArraySize: len(expectedStruct.StringArray), Dynamic: false},
		{Name: "timeArray", Value: expectedStruct.TimeArray, ArraySize: len(expectedStruct.TimeArray), Dynamic: false},
		{Name: "durationArray", Value: expectedStruct.DurationArray, ArraySize: len(expectedStruct.DurationArray), Dynamic: false},
		{Name: "personArray", ArraySize: len(expectedStruct.PersonArray), Dynamic: false, Value: [][]TestField{
			{{Name: "age", Value: expectedStruct.PersonArray[0].Age}},
			{{Name: "age", Value: expectedStruct.PersonArray[1].Age}},
		}},
		{Name: "const", Value: []TestField{
			{Name: "boolConst", Value: expectedStruct.Const.BoolConst, Const: true},
			{Name: "int8Const", Value: expectedStruct.Const.Int8Const, Const: true},
			{Name: "uint8Const", Value: expectedStruct.Const.Uint8Const, Const: true},
			{Name: "int16Const", Value: expectedStruct.Const.Int16Const, Const: true},
			{Name: "uint16Const", Value: expectedStruct.Const.Uint16Const, Const: true},
			{Name: "int32Const", Value: expectedStruct.Const.Int32Const, Const: true},
			{Name: "uint32Const", Value: expectedStruct.Const.Uint32Const, Const: true},
			{Name: "int64Const", Value: expectedStruct.Const.Int64Const, Const: true},
			{Name: "uint64Const", Value: expectedStruct.Const.Uint64Const, Const: true},
			{Name: "float32Const", Value: expectedStruct.Const.Float32Const, Const: true},
			{Name: "float64Const", Value: expectedStruct.Const.Float64Const, Const: true},
			{Name: "stringConst", Value: expectedStruct.Const.StringConst, Const: true},
		}},
	}

	expectedMap := convertFieldsToMap(expectedFields)
	msgDataRaw := convertFieldsToRaw(expectedFields)

	var msgDef MessageDefinition
	err := msgDef.unmarshall(msgDefRaw)
	if err != nil {
		t.Fatal(err)
	}

	actualMap := make(map[string]interface{})
	_, err = decodeMessageData(&msgDef, msgDataRaw, actualMap)
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(expectedMap, actualMap); diff != "" {
		t.Fatal(diff)
	}

	var actualStruct Data
	_, err = decodeMessageData(&msgDef, msgDataRaw, &actualStruct)
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(expectedStruct, actualStruct); diff != "" {
		t.Fatal(diff)
	}
}

func TestDecodeMessageDataNew(t *testing.T) {
	type Expected struct {
		Struct interface{}
		Map    map[string]interface{}
	}

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

				if diff := cmp.Diff(expected.Struct, actualStruct); diff != "" {
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

				if diff := cmp.Diff(expected.Struct, actualStruct); diff != "" {
					t.Fatalf("[Map] Decoded value is not matched:\n\n%s", diff)
				}
			}
		})
	}
}
