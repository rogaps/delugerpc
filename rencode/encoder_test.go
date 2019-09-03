package rencode

import (
	"bytes"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"strings"
	"testing"
)

type encodeTestCase struct {
	value    interface{}
	expected string
}

var encodeTestCases = []encodeTestCase{
	{nil, "E"},
	{(*int)(nil), "E"},
	{false, "D"},
	{true, "C"},
	{43, "\x2b"},
	{uint(43), "\x2b"},
	{-32, "\x65"},
	{int8(127), ">\x7f"},
	{uint8(127), ">\x7f"},
	{int16(32767), "?\x7f\xff"},
	{uint16(32767), "?\x7f\xff"},
	{int32(2147483647), "@\x7f\xff\xff\xff"},
	{uint32(2147483647), "@\x7f\xff\xff\xff"},
	{int64(9223372036854775807), "A\x7f\xff\xff\xff\xff\xff\xff\xff"},
	{uint64(9223372036854775807), "A\x7f\xff\xff\xff\xff\xff\xff\xff"},
	{uint64(18446744073709551615), "=18446744073709551615\x7f"},
	{bigIntFromString("9223372036854775808"), "=9223372036854775808\x7f"},
	{*bigIntFromString("9223372036854775808"), "=9223372036854775808\x7f"},
	{float32(math.MaxFloat32), "B\x7f\x7f\xff\xff"},
	{float64(math.MaxFloat64), ",\x7f\xef\xff\xff\xff\xff\xff\xff"},
	{"fööbar", "\x88fööbar"},
	{[]byte("fööbar"), "\x88fööbar"},
	{strings.Repeat("o", 65), "65:ooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooo"},
	{[]interface{}{int8(127), "fööbar", strings.Repeat("o", 65)},
		"\xc3>\u007f\x88fööbar65:ooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooo"},
	{sliceWithLength(65),
		";\x83ö0\x83ö1\x83ö2\x83ö3\x83ö4\x83ö5\x83ö6\x83ö7\x83ö8\x83ö9\x84ö10\x84ö11\x84ö12\x84ö13\x84ö14\x84ö15\x84ö16\x84ö17\x84ö18\x84ö19\x84ö20\x84ö21\x84ö22\x84ö23\x84ö24\x84ö25\x84ö26\x84ö27\x84ö28\x84ö29\x84ö30\x84ö31\x84ö32\x84ö33\x84ö34\x84ö35\x84ö36\x84ö37\x84ö38\x84ö39\x84ö40\x84ö41\x84ö42\x84ö43\x84ö44\x84ö45\x84ö46\x84ö47\x84ö48\x84ö49\x84ö50\x84ö51\x84ö52\x84ö53\x84ö54\x84ö55\x84ö56\x84ö57\x84ö58\x84ö59\x84ö60\x84ö61\x84ö62\x84ö63\x84ö64\u007f"},
	{map[string]interface{}{"fööbar": strings.Repeat("o", 65)},
		"\x67\x88fööbar65:ooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooo"},
	{map[string]interface{}{"fööbar": (*int)(nil)},
		"g\x88fööbarE"},
	{mapWithLength(25),
		"<\x83ö0\x00\x83ö1\x01\x84ö10\n\x84ö11\v\x84ö12\f\x84ö13\r\x84ö14\x0e\x84ö15\x0f\x84ö16\x10\x84ö17\x11\x84ö18\x12\x84ö19\x13\x83ö2\x02\x84ö20\x14\x84ö21\x15\x84ö22\x16\x84ö23\x17\x84ö24\x18\x83ö3\x03\x83ö4\x04\x83ö5\x05\x83ö6\x06\x83ö7\a\x83ö8\b\x83ö9\t\u007f"},
}

func bigIntFromString(s string) *big.Int {
	bi, ok := new(big.Int).SetString(s, 10)
	if !ok {
		panic(s)
	}
	return bi
}

func mapWithLength(n int) map[string]interface{} {
	m := make(map[string]interface{})
	for i := 0; i < n; i++ {
		m[fmt.Sprintf("ö%d", i)] = int64(i)
	}
	return m
}

func sliceWithLength(n int) (s []interface{}) {
	for i := 0; i < n; i++ {
		s = append(s, fmt.Sprintf("ö%d", i))
	}
	return
}

func TestEncode(t *testing.T) {
	for _, test := range encodeTestCases {
		var buf bytes.Buffer
		e := NewEncoder(&buf)
		if err := e.Encode(test.value); err != nil {
			t.Fatal(err)
		}
		actual := string(buf.Bytes())
		if !reflect.DeepEqual(test.expected, actual) {
			t.Fatalf("\n"+
				"For     : %v\n"+
				"expected: %+q\n"+
				"actual  : %+q", test.value, test.expected, actual)
		}
	}
}
