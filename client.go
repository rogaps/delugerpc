package delugerpc

import (
	"bytes"
	"compress/zlib"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/rpc"
	"reflect"
	"strings"

	"github.com/rogaps/delugerpc/rencode"
)

type rpcResponseTypeID int

const (
	rpcResponse rpcResponseTypeID = 1
	rpcError    rpcResponseTypeID = 2
	rpcEvent    rpcResponseTypeID = 3
)

type clientCodec struct {
	conn     *tls.Conn
	respBody interface{}
}

func (c *clientCodec) WriteRequest(r *rpc.Request, body interface{}) error {
	var b bytes.Buffer
	var msg []interface{}
	var req []interface{}

	zw := zlib.NewWriter(&b)
	e := rencode.NewEncoder(zw)

	args, kwargs := getArgs(body)
	msg = append(msg, r.Seq, r.ServiceMethod, args, kwargs)
	req = append(req, msg)

	if err := e.Encode(req); err != nil {
		return err
	}

	if err := zw.Close(); err != nil {
		return err
	}
	if _, err := c.conn.Write(b.Bytes()); err != nil {
		return err
	}

	return nil
}

func (c *clientCodec) ReadResponseHeader(r *rpc.Response) (err error) {
	zr, err := zlib.NewReader(c.conn)
	if err != nil {
		return
	}
	d := rencode.NewDecoder(zr)

	var resp []interface{}
	if err = d.Decode(&resp); err != nil {
		return
	}

	messageType := resp[0].(int64)
	r.Seq = uint64(resp[1].(int64))

	switch rpcResponseTypeID(messageType) {
	case rpcResponse:
		c.respBody = resp[2]
		return
	case rpcError:
		errMsg := resp[2].([]interface{})
		exceptionType := errMsg[0]
		exceptionMsg := errMsg[1]
		return fmt.Errorf("%v: %v", exceptionType, exceptionMsg)
	case rpcEvent:
		return errors.New("event is not supported")
	default:
		return errors.New("unknown message type")
	}
}

func (c *clientCodec) ReadResponseBody(body interface{}) (err error) {
	bv := reflect.ValueOf(body)
	if bv.Kind() != reflect.Ptr || bv.IsNil() {
		return errors.New("Unwritable type passed into decode")
	}
	bv = bv.Elem()
	if c.respBody != nil {
		bv.Set(reflect.ValueOf(c.respBody))
	}
	return nil
}

func (c *clientCodec) Close() error {
	return c.conn.Close()
}

func newDelugeCodec(conn *tls.Conn) rpc.ClientCodec {
	return &clientCodec{
		conn: conn,
	}
}

// Dial creates RPC client with rencode codec
func Dial(network, address string) (*rpc.Client, error) {
	conn, err := net.Dial(network, address)
	if err != nil {
		return nil, err
	}
	tlsConn := tls.Client(conn, &tls.Config{
		ServerName:         address,
		InsecureSkipVerify: true,
	})
	return rpc.NewClientWithCodec(newDelugeCodec(tlsConn)), err
}

func getArgs(body interface{}) (args []interface{}, kwargs map[string]interface{}) {
	bodyValue := reflect.ValueOf(body)
	switch bodyValue.Kind() {
	case reflect.Slice, reflect.Array:
		for i := 0; i < bodyValue.Len(); i++ {
			args = append(args, bodyValue.Index(i).Interface())
		}
		return
	case reflect.Map:
		for _, key := range bodyValue.MapKeys() {
			if strings.EqualFold("args", key.String()) {
				argsValue := bodyValue.MapIndex(key)
				if argsValue.Kind() == reflect.Interface {
					argsValue = argsValue.Elem()
				}
				if argsValue.Kind() == reflect.Slice ||
					argsValue.Kind() == reflect.Array {
					for i := 0; i < argsValue.Len(); i++ {
						args = append(args, argsValue.Index(i).Interface())
					}
				}
			} else if strings.EqualFold("kwargs", key.String()) {
				kwargsValue := bodyValue.MapIndex(key)
				if kwargsValue.Kind() == reflect.Interface {
					kwargsValue = kwargsValue.Elem()
				}
				if kwargsValue.Kind() == reflect.Map {
					kwargs = make(map[string]interface{})
					for _, key := range kwargsValue.MapKeys() {
						kwargs[key.String()] = kwargsValue.MapIndex(key).Interface()
					}
				}
			}
		}
		return
	default:
		return
	}
}
