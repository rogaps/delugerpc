package rencode

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"math/big"
	"reflect"
	"sort"
)

// Encoder represents rencode encoder
type Encoder struct {
	w io.Writer
}

// Encode encodes value
func (e *Encoder) Encode(v interface{}) error {
	switch v.(type) {
	case nil:
		return e.encodeNil()
	default:
		return e.encodeValue(reflect.ValueOf(v))
	}
}

type stringValues []reflect.Value

func (sv stringValues) Len() int           { return len(sv) }
func (sv stringValues) Swap(i, j int)      { sv[i], sv[j] = sv[j], sv[i] }
func (sv stringValues) Less(i, j int) bool { return sv.get(i) < sv.get(j) }
func (sv stringValues) get(i int) string   { return sv[i].String() }

func (e *Encoder) encodeValue(v reflect.Value) error {
	switch v.Kind() {
	case reflect.Bool:
		return e.encodeBool(v)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return e.encodeInt(v)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return e.encodeUint(v)
	case reflect.Float32:
		return e.encodeFloat32(v)
	case reflect.Float64:
		return e.encodeFloat64(v)
	case reflect.Struct:
		if v.Type() == reflect.TypeOf(big.Int{}) {
			return e.encodeBigInt(v)
		}
	case reflect.String:
		return e.encodeBytes([]byte(v.String()))
	case reflect.Slice, reflect.Array:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			return e.encodeBytes(v.Bytes())
		}
		return e.encodeSlice(v)
	case reflect.Map:
		return e.encodeMap(v)
	case reflect.Ptr:
		if v.IsNil() {
			return e.encodeNil()
		}
		v = v.Elem()
		return e.encodeValue(v)
	}

	return nil
}

func (e *Encoder) encodeNil() error {
	return e.write([]byte{chrNone})
}

func (e *Encoder) encodeMap(v reflect.Value) error {
	var err error
	vLen := v.Len()
	fixedCount := byte(vLen) < dictFixedCount

	if fixedCount {
		err = e.write([]byte{dictFixedStart + byte(vLen)})
	} else {
		err = e.write([]byte{chrDict})
	}
	if err != nil {
		return err
	}

	keys := stringValues(v.MapKeys())
	sort.Sort(keys)
	for i := range keys {
		val := v.MapIndex(keys[i])
		if err := e.Encode(keys[i].Interface()); err != nil {
			return err
		}
		if err := e.Encode(val.Interface()); err != nil {
			return err
		}
	}

	if !fixedCount {
		err = e.write([]byte{byte(chrTerm)})
	}
	return err
}

func (e *Encoder) encodeSlice(v reflect.Value) error {
	var err error
	vLen := v.Len()
	fixedCount := byte(vLen) < listFixedCount

	if fixedCount {
		err = e.write([]byte{listFixedStart + byte(vLen)})
	} else {
		err = e.write([]byte{chrList})
	}
	if err != nil {
		return err
	}

	for i := 0; i < vLen; i++ {
		if err := e.Encode(v.Index(i).Interface()); err != nil {
			return err
		}
	}

	if !fixedCount {
		err = e.write([]byte{byte(chrTerm)})
	}
	return err
}

func (e *Encoder) encodeBool(v reflect.Value) error {
	if v.Bool() {
		return e.write([]byte{chrTrue})
	}
	return e.write([]byte{chrFalse})
}

func (e *Encoder) encodeInt(v reflect.Value) error {
	i := v.Int()
	if 0 <= i && i < int64(intPosFixedCount) {
		return e.write([]byte{intPosFixedStart + byte(i)})
	}
	if -int64(intNegFixedCount) <= i && i < 0 {
		return e.write([]byte{intNegFixedStart - 1 - byte(i)})
	}
	if math.MinInt8 <= i && i <= math.MaxInt8 {
		return e.write([]byte{chrInt1, byte(i)})
	}
	if math.MinInt16 <= i && i <= math.MaxInt16 {
		if err := e.write([]byte{chrInt2}); err != nil {
			return err
		}
		return binary.Write(e.w, binary.BigEndian, int16(i))
	}
	if math.MinInt32 <= i && i <= math.MaxInt32 {
		if err := e.write([]byte{chrInt4}); err != nil {
			return err
		}
		return binary.Write(e.w, binary.BigEndian, int32(i))
	}
	if err := e.write([]byte{chrInt8}); err != nil {
		return err
	}
	return binary.Write(e.w, binary.BigEndian, int64(i))
}

func (e *Encoder) encodeUint(v reflect.Value) error {
	i := v.Uint()
	if i < uint64(intPosFixedCount) {
		return e.write([]byte{intPosFixedStart + byte(i)})
	}
	if i <= math.MaxInt8 {
		return e.write([]byte{chrInt1, byte(i)})
	}
	if i <= math.MaxInt16 {
		if err := e.write([]byte{chrInt2}); err != nil {
			return err
		}
		return binary.Write(e.w, binary.BigEndian, int16(i))
	}
	if i <= math.MaxInt32 {
		if err := e.write([]byte{chrInt4}); err != nil {
			return err
		}
		return binary.Write(e.w, binary.BigEndian, int32(i))
	}
	if i <= math.MaxInt64 {
		if err := e.write([]byte{chrInt8}); err != nil {
			return err
		}
		return binary.Write(e.w, binary.BigEndian, int64(i))
	}
	bi := new(big.Int).SetUint64(i)
	return e.encodeBigInt(reflect.ValueOf(bi).Elem())
}

func (e *Encoder) encodeBigInt(v reflect.Value) error {
	bi := v.Interface().(big.Int)
	s := bi.String()
	if len(s) > int(maxIntLength) {
		return fmt.Errorf("rencode: Number is longer than %d characters", maxIntLength)
	}
	if err := e.write([]byte{chrInt}); err != nil {
		return err
	}
	if err := e.write([]byte(s)); err != nil {
		return err
	}
	return e.write([]byte{chrTerm})
}

func (e *Encoder) encodeFloat32(v reflect.Value) error {
	if err := e.write([]byte{chrFloat32}); err != nil {
		return err
	}
	return binary.Write(e.w, binary.BigEndian, float32(v.Float()))
}

func (e *Encoder) encodeFloat64(v reflect.Value) error {
	if err := e.write([]byte{chrFloat64}); err != nil {
		return err
	}
	return binary.Write(e.w, binary.BigEndian, v.Float())
}

func (e *Encoder) encodeBytes(v []byte) error {
	lv := len(v)
	if byte(lv) < strFixedCount {
		if err := e.write([]byte{strFixedStart + byte(lv)}); err != nil {
			return err
		}
		return e.write(v)
	}

	prefix := []byte(fmt.Sprintf("%d:", lv))

	if err := e.write(prefix); err != nil {
		return err
	}

	return e.write(v)
}

func (e *Encoder) write(b []byte) error {
	_, err := e.w.Write(b)
	return err
}
