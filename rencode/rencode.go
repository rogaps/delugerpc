package rencode

import (
	"bufio"
	"io"
)

const (
	maxIntLength     byte = 64
	chrList          byte = 59
	chrDict          byte = 60
	chrInt           byte = 61
	chrInt1          byte = 62
	chrInt2          byte = 63
	chrInt4          byte = 64
	chrInt8          byte = 65
	chrFloat32       byte = 66
	chrFloat64       byte = 44
	chrTrue          byte = 67
	chrFalse         byte = 68
	chrNone          byte = 69
	chrTerm          byte = 127
	intPosFixedStart byte = 0
	intPosFixedCount byte = 44
	dictFixedStart   byte = 102
	dictFixedCount   byte = 25
	intNegFixedStart byte = 70
	intNegFixedCount byte = 32
	strFixedStart    byte = 128
	strFixedCount    byte = 64
	listFixedStart   byte = strFixedStart + strFixedCount
	listFixedCount   byte = 64
)

// NewDecoder returns a new rencode decoder
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: bufio.NewReader(r)}
}

// NewEncoder returns a new rencode encoder
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w}
}
