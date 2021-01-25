package rosbag

import (
	"math"
	"reflect"
	"time"
	"unsafe"
)

type fieldDecodeFunc func(raw []byte, length int) (v interface{}, off int, ok bool)

var fieldDecodeBasicHelper = map[MessageFieldType]fieldDecodeFunc{
	MessageFieldTypeBool:     fieldDecodeBool,
	MessageFieldTypeInt8:     fieldDecodeInt8,
	MessageFieldTypeUint8:    fieldDecodeUint8,
	MessageFieldTypeInt16:    fieldDecodeInt16,
	MessageFieldTypeUint16:   fieldDecodeUint16,
	MessageFieldTypeInt32:    fieldDecodeInt32,
	MessageFieldTypeUint32:   fieldDecodeUint32,
	MessageFieldTypeInt64:    fieldDecodeInt64,
	MessageFieldTypeUint64:   fieldDecodeUint64,
	MessageFieldTypeFloat32:  fieldDecodeFloat32,
	MessageFieldTypeFloat64:  fieldDecodeFloat64,
	MessageFieldTypeString:   fieldDecodeString,
	MessageFieldTypeTime:     fieldDecodeTime,
	MessageFieldTypeDuration: fieldDecodeDuration,
}

var fieldDecodeSliceHelper map[MessageFieldType]fieldDecodeFunc

func initFieldSliceDecoder(fastMode bool) {
	if fastMode {
		fieldDecodeSliceHelper = map[MessageFieldType]fieldDecodeFunc{
			MessageFieldTypeBool:     fieldDecodeBoolSlice,
			MessageFieldTypeInt8:     fieldDecodeInt8Slice,
			MessageFieldTypeUint8:    fieldDecodeUint8Slice,
			MessageFieldTypeInt16:    fieldDecodeInt16Slice,
			MessageFieldTypeUint16:   fieldDecodeUint16Slice,
			MessageFieldTypeInt32:    fieldDecodeInt32Slice,
			MessageFieldTypeUint32:   fieldDecodeUint32Slice,
			MessageFieldTypeInt64:    fieldDecodeInt64Slice,
			MessageFieldTypeUint64:   fieldDecodeUint64Slice,
			MessageFieldTypeFloat32:  fieldDecodeFloat32Slice,
			MessageFieldTypeFloat64:  fieldDecodeFloat64Slice,
			MessageFieldTypeString:   fieldDecodeStringSlice,
			MessageFieldTypeTime:     fieldDecodeTimeSlice,
			MessageFieldTypeDuration: fieldDecodeDurationSlice,
		}
	} else {
		fieldDecodeSliceHelper = map[MessageFieldType]fieldDecodeFunc{
			MessageFieldTypeBool:     fieldDecodeBoolSlice,
			MessageFieldTypeInt8:     fieldDecodeInt8Slice,
			MessageFieldTypeUint8:    fieldDecodeUint8Slice,
			MessageFieldTypeInt16:    fieldDecodeInt16SliceSlow,
			MessageFieldTypeUint16:   fieldDecodeUint16SliceSlow,
			MessageFieldTypeInt32:    fieldDecodeInt32SliceSlow,
			MessageFieldTypeUint32:   fieldDecodeUint32SliceSlow,
			MessageFieldTypeInt64:    fieldDecodeInt64SliceSlow,
			MessageFieldTypeUint64:   fieldDecodeUint64SliceSlow,
			MessageFieldTypeFloat32:  fieldDecodeFloat32SliceSlow,
			MessageFieldTypeFloat64:  fieldDecodeFloat64SliceSlow,
			MessageFieldTypeString:   fieldDecodeStringSlice,
			MessageFieldTypeTime:     fieldDecodeTimeSlice,
			MessageFieldTypeDuration: fieldDecodeDurationSlice,
		}
	}
}

func fieldDecodeLength(raw []byte, fixedLength int) (length int, off int, ok bool) {
	if fixedLength >= 0 {
		ok = true
		length = fixedLength
		return
	}

	if len(raw) < lenInBytes {
		return
	}

	length = int(endian.Uint32(raw))
	if len(raw) < lenInBytes+length {
		return
	}

	ok = true
	off = lenInBytes
	return
}

func fieldDecodeBool(raw []byte, length int) (v interface{}, off int, ok bool) {
	off = 1
	if len(raw) < off {
		return
	}

	v = raw[0] != 0
	ok = true
	return
}

func fieldDecodeInt8(raw []byte, length int) (v interface{}, off int, ok bool) {
	off = 1
	if len(raw) < off {
		return
	}

	v = int8(raw[0])
	ok = true
	return
}

func fieldDecodeUint8(raw []byte, length int) (v interface{}, off int, ok bool) {
	off = 1
	if len(raw) < off {
		return
	}

	v = uint8(raw[0])
	ok = true
	return
}

func fieldDecodeInt16(raw []byte, length int) (v interface{}, off int, ok bool) {
	off = 2
	if len(raw) < off {
		return
	}

	v = int16(endian.Uint16(raw))
	ok = true
	return
}

func fieldDecodeUint16(raw []byte, length int) (v interface{}, off int, ok bool) {
	off = 2
	if len(raw) < off {
		return
	}

	v = endian.Uint16(raw)
	ok = true
	return
}

func fieldDecodeInt32(raw []byte, length int) (v interface{}, off int, ok bool) {
	off = 4
	if len(raw) < off {
		return
	}

	v = int32(endian.Uint32(raw))
	ok = true
	return
}

func fieldDecodeUint32(raw []byte, length int) (v interface{}, off int, ok bool) {
	off = 4
	if len(raw) < off {
		return
	}

	v = endian.Uint32(raw)
	ok = true
	return
}

func fieldDecodeInt64(raw []byte, length int) (v interface{}, off int, ok bool) {
	off = 8
	if len(raw) < off {
		return
	}

	v = int64(endian.Uint64(raw))
	ok = true
	return
}

func fieldDecodeUint64(raw []byte, length int) (v interface{}, off int, ok bool) {
	off = 8
	if len(raw) < off {
		return
	}

	v = endian.Uint64(raw)
	ok = true
	return
}

func fieldDecodeFloat32(raw []byte, length int) (v interface{}, off int, ok bool) {
	off = 4
	if len(raw) < off {
		return
	}

	u := endian.Uint32(raw)
	v = math.Float32frombits(u)
	ok = true
	return
}

func fieldDecodeFloat64(raw []byte, length int) (v interface{}, off int, ok bool) {
	off = 8
	if len(raw) < off {
		return
	}

	u := endian.Uint64(raw)
	v = math.Float64frombits(u)
	ok = true
	return
}

func fieldDecodeString(raw []byte, length int) (v interface{}, off int, ok bool) {
	var s string

	length, off, ok = fieldDecodeLength(raw, length)
	if !ok {
		return
	}

	if length == 0 {
		v = ""
		ok = true
		return
	}

	raw = raw[off:]
	if len(raw) < length {
		ok = false
		return
	}

	sl := (*reflect.StringHeader)(unsafe.Pointer(&s))
	sl.Data = uintptr(unsafe.Pointer(&raw[0]))
	sl.Len = length
	off += length
	ok = true
	v = s
	return
}

func fieldDecodeTime(raw []byte, length int) (v interface{}, off int, ok bool) {
	off = 8
	if len(raw) < off {
		return
	}

	v = extractTime(raw)
	ok = true
	return
}

func fieldDecodeDuration(raw []byte, length int) (v interface{}, off int, ok bool) {
	off = 8
	if len(raw) < off {
		return
	}

	v = extractDuration(raw)
	ok = true
	return
}

func fieldDecodeBasicSlice(raw []byte, ptr unsafe.Pointer, length int, size int) (off int, ok bool) {
	length, off, ok = fieldDecodeLength(raw, length)
	if !ok {
		return
	}

	if length == 0 {
		ok = true
		return
	}

	raw = raw[off:]
	if len(raw) < length*size {
		ok = false
		return
	}

	s := (*reflect.SliceHeader)(ptr)
	s.Data = uintptr(unsafe.Pointer(&raw[0]))
	s.Len = length
	s.Cap = length
	off += length * size
	ok = true
	return
}

func fieldDecodeBoolSlice(raw []byte, length int) (v interface{}, off int, ok bool) {
	var b []bool

	off, ok = fieldDecodeBasicSlice(raw, unsafe.Pointer(&b), length, 1)
	v = b
	return
}

func fieldDecodeInt8Slice(raw []byte, length int) (v interface{}, off int, ok bool) {
	var b []int8

	off, ok = fieldDecodeBasicSlice(raw, unsafe.Pointer(&b), length, 1)
	v = b
	return
}

func fieldDecodeUint8Slice(raw []byte, length int) (v interface{}, off int, ok bool) {
	var b []uint8

	off, ok = fieldDecodeBasicSlice(raw, unsafe.Pointer(&b), length, 1)
	v = b
	return
}

func fieldDecodeInt16Slice(raw []byte, length int) (v interface{}, off int, ok bool) {
	var b []int16

	off, ok = fieldDecodeBasicSlice(raw, unsafe.Pointer(&b), length, 2)
	v = b
	return
}

func fieldDecodeInt16SliceSlow(raw []byte, length int) (v interface{}, off int, ok bool) {
	length, off, ok = fieldDecodeLength(raw, length)
	if !ok {
		return
	}

	arr := make([]int16, length)
	for i := range arr {
		arr[i] = int16(endian.Uint16(raw[off:]))
		off += 2
	}
	v = arr
	return
}

func fieldDecodeUint16Slice(raw []byte, length int) (v interface{}, off int, ok bool) {
	var b []uint16

	off, ok = fieldDecodeBasicSlice(raw, unsafe.Pointer(&b), length, 2)
	v = b
	return
}

func fieldDecodeUint16SliceSlow(raw []byte, length int) (v interface{}, off int, ok bool) {
	length, off, ok = fieldDecodeLength(raw, length)
	if !ok {
		return
	}

	arr := make([]uint16, length)
	for i := range arr {
		arr[i] = endian.Uint16(raw[off:])
		off += 2
	}
	v = arr
	return
}

func fieldDecodeInt32Slice(raw []byte, length int) (v interface{}, off int, ok bool) {
	var b []int32

	off, ok = fieldDecodeBasicSlice(raw, unsafe.Pointer(&b), length, 4)
	v = b
	return
}

func fieldDecodeInt32SliceSlow(raw []byte, length int) (v interface{}, off int, ok bool) {
	length, off, ok = fieldDecodeLength(raw, length)
	if !ok {
		return
	}

	arr := make([]int32, length)
	for i := range arr {
		arr[i] = int32(endian.Uint32(raw[off:]))
		off += 4
	}
	v = arr
	return
}

func fieldDecodeUint32Slice(raw []byte, length int) (v interface{}, off int, ok bool) {
	var b []uint32

	off, ok = fieldDecodeBasicSlice(raw, unsafe.Pointer(&b), length, 4)
	v = b
	return
}

func fieldDecodeUint32SliceSlow(raw []byte, length int) (v interface{}, off int, ok bool) {
	length, off, ok = fieldDecodeLength(raw, length)
	if !ok {
		return
	}

	arr := make([]uint32, length)
	for i := range arr {
		arr[i] = endian.Uint32(raw[off:])
		off += 4
	}
	v = arr
	return
}

func fieldDecodeInt64Slice(raw []byte, length int) (v interface{}, off int, ok bool) {
	var b []int64

	off, ok = fieldDecodeBasicSlice(raw, unsafe.Pointer(&b), length, 8)
	v = b
	return
}

func fieldDecodeInt64SliceSlow(raw []byte, length int) (v interface{}, off int, ok bool) {
	length, off, ok = fieldDecodeLength(raw, length)
	if !ok {
		return
	}

	arr := make([]int64, length)
	for i := range arr {
		arr[i] = int64(endian.Uint64(raw[off:]))
		off += 8
	}
	v = arr
	return
}

func fieldDecodeUint64Slice(raw []byte, length int) (v interface{}, off int, ok bool) {
	var b []uint64

	off, ok = fieldDecodeBasicSlice(raw, unsafe.Pointer(&b), length, 8)
	v = b
	return
}

func fieldDecodeUint64SliceSlow(raw []byte, length int) (v interface{}, off int, ok bool) {
	length, off, ok = fieldDecodeLength(raw, length)
	if !ok {
		return
	}

	arr := make([]uint64, length)
	for i := range arr {
		arr[i] = endian.Uint64(raw[off:])
		off += 8
	}
	v = arr
	return
}

func fieldDecodeFloat32Slice(raw []byte, length int) (v interface{}, off int, ok bool) {
	var b []float32

	off, ok = fieldDecodeBasicSlice(raw, unsafe.Pointer(&b), length, 4)
	v = b
	return
}

func fieldDecodeFloat32SliceSlow(raw []byte, length int) (v interface{}, off int, ok bool) {
	length, off, ok = fieldDecodeLength(raw, length)
	if !ok {
		return
	}

	arr := make([]float32, length)
	for i := range arr {
		arr[i] = math.Float32frombits(endian.Uint32(raw[off:]))
		off += 4
	}
	v = arr
	return
}

func fieldDecodeFloat64Slice(raw []byte, length int) (v interface{}, off int, ok bool) {
	var b []float64

	off, ok = fieldDecodeBasicSlice(raw, unsafe.Pointer(&b), length, 8)
	v = b
	return
}

func fieldDecodeFloat64SliceSlow(raw []byte, length int) (v interface{}, off int, ok bool) {
	length, off, ok = fieldDecodeLength(raw, length)
	if !ok {
		return
	}

	arr := make([]float64, length)
	for i := range arr {
		arr[i] = math.Float64frombits(endian.Uint64(raw[off:]))
		off += 8
	}
	v = arr
	return
}

func fieldDecodeStringSlice(raw []byte, length int) (v interface{}, off int, ok bool) {
	length, off, ok = fieldDecodeLength(raw, length)
	if !ok {
		return
	}

	if length == 0 {
		var s []string
		v = s
		ok = true
		return
	}

	s := make([]string, length)
	totalOff := off
	for i := 0; i < length; i++ {
		v, off, ok = fieldDecodeString(raw[totalOff:], -1)
		if !ok {
			off = 0
			return
		}

		s[i] = v.(string)
		totalOff += off
	}

	v = s
	off = totalOff
	ok = true
	return
}

func fieldDecodeTimeSlice(raw []byte, length int) (v interface{}, off int, ok bool) {
	length, off, ok = fieldDecodeLength(raw, length)
	if !ok {
		return
	}

	s := make([]time.Time, length)
	totalOff := off
	for i := 0; i < length; i++ {
		v, off, ok = fieldDecodeTime(raw[totalOff:], 0)
		if !ok {
			off = 0
			return
		}

		s[i] = v.(time.Time)
		totalOff += off
	}

	v = s
	off = totalOff
	ok = true
	return
}

func fieldDecodeDurationSlice(raw []byte, length int) (v interface{}, off int, ok bool) {
	length, off, ok = fieldDecodeLength(raw, length)
	if !ok {
		return
	}

	s := make([]time.Duration, length)
	totalOff := off
	for i := 0; i < length; i++ {
		v, off, ok = fieldDecodeDuration(raw[totalOff:], 0)
		if !ok {
			off = 0
			return
		}

		s[i] = v.(time.Duration)
		totalOff += off
	}

	v = s
	off = totalOff
	ok = true
	return
}
