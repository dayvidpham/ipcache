package msgs

import (
	"bufio"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"net"
	"time"
)

type ProtocolVersion uint8
type MessageType uint8

const (
	Version_1_0_0 ProtocolVersion = iota
)

const (
	T_Ok MessageType = iota
	T_Err
	T_String

	T_Ping
	T_Pong

	T_ClientRegister
	T_ClientGetIPs

	T_ClientGrantAuthorization
	T_ClientRevokeAuthorization
)

var messageTypeName = map[MessageType]string{
	T_Ok:     "Ok",
	T_Err:    "Err",
	T_String: "String",

	T_Ping: "Ping",
	T_Pong: "Pong",

	T_ClientRegister: "ClientRegister",
	T_ClientGetIPs:   "ClientGetIPs",

	T_ClientGrantAuthorization:  "ClientGrantAuthorization",
	T_ClientRevokeAuthorization: "ClientRevokeAuthorization",
}

func (mt MessageType) String() string {
	return messageTypeName[mt]
}

var DefaultVersion ProtocolVersion = Version_1_0_0

type Message struct {
	Type             MessageType
	Version          ProtocolVersion
	UnixTimestampUtc int64
	Payload          []byte
}

var emptyMsg Message
var msgStaticSize int = binary.Size(emptyMsg.Type) +
	binary.Size(emptyMsg.Version) +
	binary.Size(emptyMsg.UnixTimestampUtc)

func (m *Message) Size() int {
	return msgStaticSize + binary.Size(m.Payload)
}

func NewMessage(msgT MessageType) Message {
	return Message{
		Type:             msgT,
		Version:          DefaultVersion,
		UnixTimestampUtc: time.Now().UTC().Unix(),
		Payload:          nil,
	}
}

func SetDefaultVersion(version ProtocolVersion) {
	DefaultVersion = version
}

func Ok() Message {
	return NewMessage(T_Ok)
}

func Err() Message {
	return NewMessage(T_Err)
}

func Ping() Message {
	return NewMessage(T_Ping)
}

func Pong() Message {
	return NewMessage(T_Pong)
}

func ClientRegister() Message {
	return NewMessage(T_ClientRegister)
}

func String(data string) Message {
	msg := NewMessage(T_String)
	msg.Payload = []byte(data)
	return msg
}

func Encode(enc *gob.Encoder, msg Message) (err error) {
	err = enc.Encode(&msg)
	if err != nil {
		return fmt.Errorf("[ERROR] Failed to decode Message\n\t%+w\n", err)
	}
	return err
}

func Decode(dec *gob.Decoder) (msg Message, err error) {
	err = dec.Decode(&msg)
	if err != nil {
		return msg, fmt.Errorf("[ERROR] Failed to decode Message\n\t%+w\n", err)
	}
	return msg, err
}

type Client struct {
	// Hmmmm not sure what fields are needed yet
	Id string
	IP net.IP
}

func ConnIP(conn *tls.Conn) (ip net.IP, err error) {
	switch x := conn.RemoteAddr().(type) {
	case *net.TCPAddr:
		return x.IP, err
	case *net.UDPAddr:
		return x.IP, err
	case *net.IPAddr:
		return x.IP, err
	default:
		return nil, fmt.Errorf("The TLS connection using the address <%s> wasn't using a net.Addr that has an IP", x.String())
	}
}

func ConnId(conn *tls.Conn) (id string, err error) {
	certs := conn.ConnectionState().PeerCertificates
	if len(certs) < 1 {
		return id, errors.New("PeerCertificates is empty, none were given by client")
	}

	cert := certs[0]
	skid := base64.StdEncoding.EncodeToString(cert.SubjectKeyId)
	//log.Printf("[INFO] The subject key id from conn's cert: %+v\n", skid)

	return skid, err
}

func NewClient(conn *tls.Conn) (client Client, err error) {
	skid, err := ConnId(conn)
	if err != nil {
		log.Println(err)
		return client, err
	}

	ip, err := ConnIP(conn)
	if err != nil {
		log.Println(err)
		return client, err
	}
	return Client{Id: skid, IP: ip}, err
}

type Messenger interface {
	Send(msg Message) (err error)
	SendN(msg Message) (n int, err error)
	Receive() (msg Message, err error)

	SetDeadline(deadline time.Time) (err error)
	SetReadDeadline(deadline time.Time) (err error)
	SetWriteDeadline(deadline time.Time) (err error)

	SetTimeout(timeout time.Duration) (err error)
	SetReadTimeout(timeout time.Duration) (err error)
	SetWriteTimeout(timeout time.Duration) (err error)
}

type blockingMessenger struct {
	conn   *tls.Conn
	connrw *bufio.ReadWriter
	enc    *gob.Encoder
	dec    *gob.Decoder
}

func (bm *blockingMessenger) Send(msg Message) (err error) {
	if err = Encode(bm.enc, msg); err != nil {
		return fmt.Errorf("[ERROR] Messenger failed to Encode the message\n\t%w\n", err)
	}
	if err = bm.connrw.Flush(); err != nil {
		return fmt.Errorf("[ERROR] Messenger failed to Flush the buffered connection\n\t%w\n", err)
	}
	return err
}

func (bm *blockingMessenger) SendN(msg Message) (n int, err error) {
	n = 0
	if err = Encode(bm.enc, msg); err != nil {
		return n, fmt.Errorf("[ERROR] Messenger failed to Encode the message\n\t%w\n", err)
	}

	err = bm.connrw.Flush()
	n = bm.connrw.Writer.Buffered()
	if err != nil {
		return n, fmt.Errorf("[ERROR] Messenger failed to Flush the buffered connection\n\t%w\n", err)
	}
	return
}

func (bm *blockingMessenger) Receive() (msg Message, err error) {
	if msg, err = Decode(bm.dec); err != nil {
		return msg, fmt.Errorf("[ERROR] Messenger failed during Receive\n\t%w\n", err)
	}
	return
}

func (bm *blockingMessenger) SetReadTimeout(timeout time.Duration) (err error) {
	err = bm.conn.SetReadDeadline(time.Now().Add(timeout))
	if err != nil {
		return fmt.Errorf("[ERROR] Failed to set read timeout of %s\n\t%w\n", timeout.String(), err)
	}
	return err
}
func (bm *blockingMessenger) SetWriteTimeout(timeout time.Duration) (err error) {
	err = bm.conn.SetWriteDeadline(time.Now().Add(timeout))
	if err != nil {
		return fmt.Errorf("[ERROR] Failed to set write timeout of %s\n\t%w\n", timeout.String(), err)
	}
	return err
}
func (bm *blockingMessenger) SetTimeout(timeout time.Duration) (err error) {
	err = bm.conn.SetDeadline(time.Now().Add(timeout))
	if err != nil {
		return fmt.Errorf("[ERROR] Failed to set timeout of %s\n\t%w\n", timeout.String(), err)
	}
	return err
}

func (bm *blockingMessenger) SetReadDeadline(deadline time.Time) (err error) {
	err = bm.conn.SetReadDeadline(deadline)
	if err != nil {
		return fmt.Errorf("[ERROR] Failed to set read deadline of %s\n\t%w\n", deadline.String(), err)
	}
	return err
}
func (bm *blockingMessenger) SetWriteDeadline(deadline time.Time) (err error) {
	err = bm.conn.SetWriteDeadline(deadline)
	if err != nil {
		return fmt.Errorf("[ERROR] Failed to set write deadline of %s\n\t%w\n", deadline.String(), err)
	}
	return err
}
func (bm *blockingMessenger) SetDeadline(deadline time.Time) (err error) {
	err = bm.conn.SetDeadline(deadline)
	if err != nil {
		return fmt.Errorf("[ERROR] Failed to set deadline of %s\n\t%w\n", deadline.String(), err)
	}
	return err
}

func NewMessenger(conn *tls.Conn) Messenger {
	r, w := bufio.NewReader(conn), bufio.NewWriter(conn)
	connrw := bufio.NewReadWriter(r, w)

	return &blockingMessenger{
		conn:   conn,
		connrw: connrw,
		enc:    gob.NewEncoder(connrw),
		dec:    gob.NewDecoder(connrw),
	}
}
