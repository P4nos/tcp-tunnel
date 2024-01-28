package common

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
)

// create enum for protocol messages
const (
	// client to server message indicating a new intent to connect
	Connect uint64 = iota + 1
	// server to client message indicating incoming traffic for the client
	Incoming
	// server|client message indicating successful parsing of a message
	Ok
	// server|client message indicating failed parsing of a message
	Error
)

// Accept message sends the port number to the server
type Message struct {
	MessageType uint64 // Matches the enum above
	PayloadSize uint64
	Payload     []byte
}

func (m *Message) Format() string {
	return fmt.Sprintf(`Message {
    messageType: %s,
    payloadSize: %s,
    payload: %s
  }`, fmt.Sprint(m.MessageType), fmt.Sprint(m.PayloadSize), string(m.Payload))
}

func ParseMessage(c net.Conn) (Message, error) {
	var messageType uint64
	err := binary.Read(c, binary.BigEndian, &messageType)
	if err != nil {
		return Message{}, err
	}

	var payloadSize uint64
	err = binary.Read(c, binary.BigEndian, &payloadSize)
	if err != nil {
		return Message{}, err
	}

	payload := make([]byte, payloadSize)
	_, err = c.Read(payload)
	if err != nil {
		return Message{}, err
	}
	return Message{
		MessageType: messageType,
		PayloadSize: payloadSize,
		Payload:     payload,
	}, nil
}

func SendMessage(c net.Conn, m Message) (uint64, error) {
	err := binary.Write(c, binary.BigEndian, m.MessageType)
	if err != nil {
		return 0, err
	}
	var n uint64 = 1
	err = binary.Write(c, binary.BigEndian, m.PayloadSize)
	if err != nil {
		return 0, err
	}
	n += uint64(m.PayloadSize)
	i, err := c.Write(m.Payload)
	if err != nil {
		return 0, err
	}
	return n + uint64(i), nil
}

func Pipe(destination net.Conn, source net.Conn) {
	// this buffer size is random
	const bufferSize = 4096

	// forward data to destination
	bufferDestination := make([]byte, bufferSize)
	_, err := source.Read(bufferDestination)

	if err != nil {
		log.Println("failed to read from server", err)
	}
	_, err = destination.Write(bytes.Trim(bufferDestination, "\x00"))
	if err != nil {
		log.Println("failed to write to destination", err)
	}

	// forward data to source
	bufferSource := make([]byte, bufferSize)
	_, err = destination.Read(bufferSource)

	if err != nil {
		log.Println("failed to read from destination", err)
	}

	_, err = source.Write(bytes.Trim(bufferSource, "\x00"))
	if err != nil {
		log.Println("failed to write to source", err)
	}
}

var ErrMaxPortNumber = errors.New("port number exceeds allowed value")
var ErrInvalidPort = errors.New("port number exceeds allowed value")

func ParsePortNumber(payload []byte) (string, error) {
	port, err := strconv.Atoi(string(payload))
	if err != nil {
		return "", errors.New("invalid port format")
	}

	controlPort, err := strconv.Atoi(os.Getenv("CONTROL_PORT"))
	if port == controlPort {
		return "", ErrInvalidPort
	}

	maxAllowedPortNumber, err := strconv.Atoi(os.Getenv("MAX_ALLOWED_PORT_NUMBER"))
	if port > maxAllowedPortNumber {
		return "", ErrMaxPortNumber
	}

	return fmt.Sprint(port), nil
}
