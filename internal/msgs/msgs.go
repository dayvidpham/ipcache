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

type Message interface {
	Type() MessageType
	Version() ProtocolVersion
	UnixTimestampUTC() int64
	Size() int
	Payload() []byte
}

type message struct {
	MsgType          MessageType
	VersionAt        ProtocolVersion
	CreatedAtUnixUTC int64
	Data             []byte
}

func (m *message) Type() MessageType {
	return m.MsgType
}
func (m *message) UnixTimestampUTC() int64 {
	return m.CreatedAtUnixUTC
}
func (m *message) Version() ProtocolVersion {
	return m.VersionAt
}
func (m *message) Payload() []byte {
	return m.Data
}

var emptyMsg message
var msgStaticSize int = binary.Size(emptyMsg.MsgType) +
	binary.Size(emptyMsg.VersionAt) +
	binary.Size(emptyMsg.CreatedAtUnixUTC)

func (m *message) Size() int {
	return msgStaticSize + binary.Size(m.Data)
}

func NewMessage(msgT MessageType) Message {
	return &message{
		MsgType:          msgT,
		VersionAt:        DefaultVersion,
		CreatedAtUnixUTC: time.Now().UTC().Unix(),
		Data:             nil,
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
	msg := NewMessage(T_String).(*message)
	msg.Data = []byte(data)
	return msg
}

func Encode(enc *gob.Encoder, msg Message) (err error) {
	err = enc.Encode(&msg)
	if err != nil {
		log.Printf("[ERROR]: Failed to encode Message\n\t%+v\n", err)
	}
	return err
}

func Decode(dec *gob.Decoder) (msg Message, err error) {
	err = dec.Decode(&msg)
	if err != nil {
		log.Printf("[ERROR]: Failed to decode Message\n\t%+v\n", err)
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

func init() {
	gob.Register(&message{})
}

type Messenger interface {
	Send(msg Message) (err error)
	SendN(msg Message) (n int, err error)
	Receive() (msg Message, err error)
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
		return msg, fmt.Errorf("[ERROR] Messenger failed to Decode the message from the buffered connection\n\t%w\n", err)
	}
	return
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
