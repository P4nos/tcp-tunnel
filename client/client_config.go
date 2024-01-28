package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/P4nos/tcp-tunnel/common"
	"github.com/joho/godotenv"
	"io"
	"log"
	"net"
	"os"
)

type ClientConfig struct {
	serverConn net.Conn
	forwardTo  string
}

// Test that the local forwarding port is available
func isLocalPortAvailable(host string) bool {
	connDestination, err := net.Dial("tcp", host)
	if err != nil {
		return false
	}
	connDestination.Close()
	return true
}

func (c *ClientConfig) handshake() error {
	_, err := common.SendMessage(c.serverConn, common.Message{MessageType: common.Connect})
	if err != nil {
		log.Println("failed to start client")
		return err
	}

	message, err := common.ParseMessage(c.serverConn)
	if err != nil || message.MessageType != common.Ok {
		log.Println("failed to parse server response during initialization")
		return err
	}

	log.Println("forwarding from:", string(message.Payload))
	return nil
}

func (c *ClientConfig) HandleIncomingTraffic() error {

	message, err := common.ParseMessage(c.serverConn)
	if err != nil {
		if err == io.EOF {
			log.Println("lost conncection with server exiting")
			return err
		}
		log.Println("failed to parse message", err)
	}

	switch message.MessageType {
	case common.Incoming:
		_, err := common.SendMessage(c.serverConn, common.Message{MessageType: common.Ok, PayloadSize: message.PayloadSize, Payload: message.Payload})
		if err != nil {
			log.Println("failed to respond to Incoming message type")
			break
		}
		connDestination, err := net.Dial("tcp", c.forwardTo)
		if err != nil {
			log.Fatalf("destination unreachable")
		}
		common.Pipe(connDestination, c.serverConn)
		connDestination.Close()

	case common.Ok:
		log.Println("ok received")
	default:
		log.Println("unknown message type ", message.MessageType)
	}
	return nil
}
func (c *ClientConfig) Shutdown() {
	c.serverConn.Close()
}

func New(forwardPort int) (ClientConfig, error) {

	err := godotenv.Load(".env")
	if err != nil {
		return ClientConfig{}, err
	}

	host := os.Getenv("SERVER_URL")

	forwardTo := "localhost:" + fmt.Sprint(forwardPort)
	if err := isLocalPortAvailable(forwardTo); !err {
		return ClientConfig{}, errors.New("forwarding port not available")
	}

	connServer, err := tls.Dial("tcp", host, nil)
	if err != nil {
		return ClientConfig{}, err
	}

	log.Println("connected to the server", connServer.RemoteAddr().String())
	config := ClientConfig{
		serverConn: connServer,
		forwardTo:  forwardTo,
	}

	if err := config.handshake(); err != nil {
		return ClientConfig{}, err
	}
	return config, nil
}
