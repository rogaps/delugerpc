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

type ArgsKwargs struct {
	Args   []interface{}
	Kwargs map[string]interface{}
}

func (c *clientCodec) WriteRequest(r *rpc.Request, body interface{}) error {
	var b bytes.Buffer
	zw := zlib.NewWriter(&b)
	e := rencode.NewEncoder(zw)

	argsKwrgs, ok := body.(ArgsKwargs)
	if !ok {
		argsKwrgs = ArgsKwargs{}
	}
	var msg []interface{}
	var reqMsg []interface{}
	reqMsg = append(reqMsg, r.Seq, r.ServiceMethod, argsKwrgs.Args, argsKwrgs.Kwargs)
	msg = append(msg, reqMsg)

	if err := e.Encode(msg); err != nil {
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

func Dial(network, address string) (*rpc.Client, error) {
	conn, err := net.Dial(network, address)
	if err != nil {
		return nil, err
	}
	tlsConn := tls.Client(conn, &tls.Config{
		ServerName:         address,
		InsecureSkipVerify: true, // x509: cannot verify signature: algorithm unimplemented
	})
	return rpc.NewClientWithCodec(newDelugeCodec(tlsConn)), err
}
