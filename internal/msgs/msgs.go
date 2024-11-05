package msgs

import (
	"crypto/tls"
	"encoding/base64"
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
	MsgT_Ok MessageType = iota
	MsgT_Err

	MsgT_Ping
	MsgT_Pong

	MsgT_ClientRegister
	MsgT_ClientGetIPs

	MsgT_ClientGrantAuthorization
	MsgT_ClientRevokeAuthorization
)

var DefaultVersion ProtocolVersion = Version_1_0_0

type Message interface {
	Type() MessageType
	Timestamp() time.Time
	Version() ProtocolVersion
}

type message struct {
	MsgType   MessageType
	CreatedAt time.Time
	VersionAt ProtocolVersion
}

func (m *message) Type() MessageType {
	return m.MsgType
}
func (m *message) Timestamp() time.Time {
	return m.CreatedAt
}
func (m *message) Version() ProtocolVersion {
	return m.VersionAt
}

func NewMessage(msgT MessageType) Message {
	return &message{
		MsgType:   msgT,
		CreatedAt: time.Now(),
		VersionAt: DefaultVersion,
	}
}

func SetDefaultVersion(version ProtocolVersion) {
	DefaultVersion = version
}

func Ok() Message {
	return NewMessage(MsgT_Ok)
}

func Err() Message {
	return NewMessage(MsgT_Err)
}

func Ping() Message {
	return NewMessage(MsgT_Ping)
}

func Pong() Message {
	return NewMessage(MsgT_Pong)
}

func ClientRegister() Message {
	return NewMessage(MsgT_ClientRegister)
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
