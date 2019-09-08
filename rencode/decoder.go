package rencode

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"math/big"
	"reflect"
	"runtime"
	"strconv"
	"unsafe"
)

// Decoder represents rencoder decoder
type Decoder struct {
	r *bufio.Reader
}

// Decode decodes stream
func (d *Decoder) Decode(v interface{}) error {
	vv := reflect.ValueOf(v)
	if vv.Kind() != reflect.Ptr || vv.IsNil() {
		return &DecodeInvalidArgError{Type: vv.Type()}
	}

	vv = vv.Elem()
	if !vv.IsValid() {
		return &DecodeInvalidArgError{Type: vv.Type()}
	}

	return d.decodeValue(vv)
}

func (d *Decoder) peekByte() (b byte, err error) {
	ch, err := d.r.Peek(1)
	if err != nil {
		return
	}
	b = ch[0]
	return
}

func (d *Decoder) decodeValue(v reflect.Value) error {
	c, err := d.r.ReadByte()
	if err != nil {
		return err
	}
	switch c {
	case chrNone:
		return nil
	case chrFalse:
		return d.decodeBool(v, false)
	case chrTrue:
		return d.decodeBool(v, true)
	case chrInt1, chrInt2, chrInt4, chrInt8, chrInt:
		return d.decodeInt(v, c)
	case chrFloat32, chrFloat64:
		return d.decodeFloat(v, c)
	case chrList:
		return d.decodeSlice(v, -1)
	case chrDict:
		return d.decodeMap(v, -1)
	default:
		if isFixedPosInt(c) {
			data := int64(c - intPosFixedStart)
			return setInt(strconv.FormatInt(data, 10), v)
		}
		if isFixedNegInt(c) {
			data := int64(c-intNegFixedStart+1) * -1
			return setInt(strconv.FormatInt(data, 10), v)
		}
		if isFixedString(c) {
			size := int64(c - strFixedStart)
			return d.decodeString(v, size)
		}
		if isString(c) {
			size, err := d.decodeStringSize(c)
			if err != nil {
				return err
			}
			return d.decodeString(v, size)
		}
		if isFixedSlice(c) {
			size := int(c - listFixedStart)
			return d.decodeSlice(v, size)
		}
		if isFixedMap(c) {
			size := int(c - dictFixedStart)
			return d.decodeMap(v, size)
		}
	}
	return fmt.Errorf("rencode: unsupported code %v", c)
}

func (d *Decoder) decodeStringSize(c byte) (int64, error) {
	size, err := d.r.ReadBytes(':')
	if err != nil {
		return 0, err
	}
	size = append([]byte{byte(c)}, size[:len(size)-1]...)

	return strconv.ParseInt(string(size), 10, 64)
}

func (d *Decoder) decodeString(v reflect.Value, size int64) error {
	data := make([]byte, size)
	n, err := io.ReadFull(d.r, data)
	if n != len(data) {
		return err
	}
	switch v.Kind() {
	case reflect.String:
		v.SetString(bytesAsString(data))
		return nil
	case reflect.Slice:
		if v.Type().Elem().Kind() != reflect.Uint8 {
			break
		}
		v.SetBytes(data)
		return nil
	case reflect.Array:
		if v.Type().Elem().Kind() != reflect.Uint8 {
			break
		}
		reflect.Copy(v, reflect.ValueOf(data))
		return nil
	case reflect.Interface:
		v.Set(reflect.ValueOf(bytesAsString(data)))
		return nil
	}

	return &DecodeTypeError{
		Value: "string",
		Type:  v.Type(),
	}
}

func (d *Decoder) decodeBool(v reflect.Value, b bool) error {
	if v.Kind() == reflect.Bool {
		v.SetBool(b)
		return nil
	}
	if v.Kind() == reflect.Interface || v.Type().ConvertibleTo(reflect.TypeOf(b)) {
		v.Set(reflect.ValueOf(b))
		return nil
	}
	return &DecodeTypeError{
		Value: "bool",
		Type:  v.Type(),
	}
}

func setInt(s string, v reflect.Value) error {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return err
		}
		if v.OverflowInt(i) {
			return &DecodeTypeError{
				Value: "integer " + s,
				Type:  v.Type(),
			}
		}
		v.SetInt(i)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		i, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return err
		}
		if v.OverflowUint(i) {
			return &DecodeTypeError{
				Value: "integer " + s,
				Type:  v.Type(),
			}
		}
		v.SetUint(i)
	case reflect.Bool:
		v.SetBool(s != "0")
	case reflect.Struct:
		if v.Type() == reflect.TypeOf(big.Int{}) {
			var bi big.Int
			if _, err := fmt.Sscan(s, &bi); err != nil {
				return err
			}
			v.Set(reflect.ValueOf(bi))
		}
	case reflect.Interface:
		n, err := strconv.ParseInt(s, 10, 64)
		if ne, ok := err.(*strconv.NumError); ok && ne.Err == strconv.ErrRange {
			var bi big.Int
			if _, err := fmt.Sscan(s, &bi); err != nil {
				return err
			}
			v.Set(reflect.ValueOf(bi))
		} else {
			if err != nil {
				return err
			}
			v.Set(reflect.ValueOf(n))
		}
	default:
		return &DecodeTypeError{
			Value: "integer " + s,
			Type:  v.Type(),
		}
	}
	return nil
}

func (d *Decoder) decodeInt(v reflect.Value, code byte) error {
	var s string

	switch code {
	case chrInt1:
		var data int8
		if err := binary.Read(d.r, binary.BigEndian, &data); err != nil {
			return err
		}
		s = strconv.FormatInt(int64(data), 10)
	case chrInt2:
		var data int16
		if err := binary.Read(d.r, binary.BigEndian, &data); err != nil {
			return err
		}
		s = strconv.FormatInt(int64(data), 10)
	case chrInt4:
		var data int32
		if err := binary.Read(d.r, binary.BigEndian, &data); err != nil {
			return err
		}
		s = strconv.FormatInt(int64(data), 10)
	case chrInt8:
		var data int64
		if err := binary.Read(d.r, binary.BigEndian, &data); err != nil {
			return err
		}
		s = strconv.FormatInt(int64(data), 10)
	case chrInt:
		var ibytes []byte
		ibytes, err := d.r.ReadBytes(chrTerm)
		if err != nil {
			return err
		}
		ibytes = ibytes[:len(ibytes)-1]
		s = string(ibytes)
	}
	return setInt(s, v)
}

func setFloat(f float64, v reflect.Value) error {
	defer func() error {
		r := recover()
		runtimeErr, ok := r.(runtime.Error)
		if ok {
			return runtimeErr
		}
		err, ok := r.(error)
		if !ok && r != nil {
			panic(r)
		}
		return err
	}()
	if v.Kind() == reflect.Interface {
		v.Set(reflect.ValueOf(f))
	} else {
		v.SetFloat(f)
	}
	return nil
}

func (d *Decoder) decodeFloat(v reflect.Value, code byte) error {
	switch code {
	case chrFloat32:
		var data float32
		if err := binary.Read(d.r, binary.BigEndian, &data); err != nil {
		}
		return setFloat(float64(data), v)
	case chrFloat64:
		var data float64
		if err := binary.Read(d.r, binary.BigEndian, &data); err != nil {
			return err
		}
		return setFloat(data, v)
	default:
		return fmt.Errorf("rencode: unsupported code %v for type float", code)
	}
}

func (d *Decoder) decodeSliceElem(i int, v reflect.Value) error {
	if i >= v.Cap() && v.IsValid() {
		newcap := v.Cap() + v.Cap()/2
		if newcap < 4 {
			newcap = 4
		}
		newv := reflect.MakeSlice(v.Type(), v.Len(), newcap)
		reflect.Copy(newv, v)
		v.Set(newv)
	}

	if i >= v.Len() && v.IsValid() {
		v.SetLen(i + 1)
	}
	return d.decodeValue(v.Index(i))
}

func (d *Decoder) decodeSlice(v reflect.Value, size int) error {
	if v.Kind() == reflect.Interface {
		var x []interface{}
		defer func(p reflect.Value) { p.Set(v) }(v)
		v = reflect.ValueOf(&x).Elem()
	}

	if v.Kind() != reflect.Array && v.Kind() != reflect.Slice {
		return &DecodeTypeError{
			Value: "slice",
			Type:  v.Type(),
		}
	}
	for i := 0; (0 <= size && i < size) || size < 0; i++ {
		if size < 0 {
			c, err := d.peekByte()
			if err != nil {
				return err
			}
			if c == chrTerm {
				_, err := d.r.ReadByte()
				return err
			}
		}
		if err := d.decodeSliceElem(i, v); err != nil {
			return err
		}
	}
	return nil
}

func (d *Decoder) decodeMap(v reflect.Value, size int) error {
	if v.Kind() == reflect.Interface {
		var x map[string]interface{}
		defer func(p reflect.Value) { p.Set(v) }(v)
		v = reflect.ValueOf(&x).Elem()
	}
	var (
		mapElem reflect.Value
		isMap   bool
		vals    map[string]reflect.Value
	)
	switch v.Kind() {
	case reflect.Map:
		t := v.Type()
		if t.Key() != reflect.TypeOf("") {
			return &DecodeTypeError{
				Value: "string ",
				Type:  t.Key(),
			}
		}
		if v.IsNil() {
			v.Set(reflect.MakeMap(t))
		}

		isMap = true
		mapElem = reflect.New(t.Elem()).Elem()
	case reflect.Struct:
		// TODO
	default:
		return &DecodeTypeError{
			Value: "map",
			Type:  v.Type(),
		}
	}

	for i := 0; (0 <= size && i < size) || size < 0; i++ {
		var subv reflect.Value

		// peek the next value type
		ch, err := d.peekByte()
		if err != nil {
			return err
		}
		if ch == chrTerm {
			_, err := d.r.ReadByte()
			return err
		}

		// peek the next value we're suppsed to read
		var key string
		if err := d.decodeValue(reflect.ValueOf(&key).Elem()); err != nil {
			return err
		}

		if isMap {
			mapElem.Set(reflect.Zero(v.Type().Elem()))
			subv = mapElem
		} else {
			subv = vals[key]
		}

		if !subv.IsValid() {
			// if it's invalid, grab but ignore the next value
			var x interface{}
			err := d.decodeValue(reflect.ValueOf(&x).Elem())
			if err != nil {
				return err
			}

			continue
		}

		// subv now contains what we load into
		if err := d.decodeValue(subv); err != nil {
			return err
		}

		if isMap {
			v.SetMapIndex(reflect.ValueOf(key), subv)
		}
	}
	return nil
}

func bytesAsString(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return *(*string)(unsafe.Pointer(&reflect.StringHeader{
		uintptr(unsafe.Pointer(&b[0])),
		len(b),
	}))
}

// DecodeTypeError represents decode type error
type DecodeTypeError struct {
	Value string
	Type  reflect.Type
}

func (e *DecodeTypeError) Error() string {
	return fmt.Sprintf("cannot decode a rencode %s into a %s", e.Value, e.Type)
}

// DecodeInvalidArgError represents decode invalid argument error
type DecodeInvalidArgError struct {
	Type reflect.Type
}

func (e *DecodeInvalidArgError) Error() string {
	if e.Type == nil {
		return "rencode: decode(nil)"
	}

	if e.Type.Kind() != reflect.Ptr {
		return "rencode: decode(non-pointer " + e.Type.String() + ")"
	}
	return "rencode: decode(nil " + e.Type.String() + ")"
}

func isFixedPosInt(code byte) bool {
	return intPosFixedStart <= code && code < intPosFixedStart+intPosFixedCount
}

func isFixedNegInt(code byte) bool {
	return intNegFixedStart <= code && code < intNegFixedStart+intNegFixedCount
}

func isFixedString(code byte) bool {
	return strFixedStart <= code && code < strFixedStart+strFixedCount
}

func isString(code byte) bool {
	return '0' <= code && code <= '9'
}

func isFixedSlice(code byte) bool {
	return listFixedStart <= code && code <= (listFixedStart-1+listFixedCount)
}

func isFixedMap(code byte) bool {
	return dictFixedStart <= code && code < dictFixedStart+dictFixedCount
}
