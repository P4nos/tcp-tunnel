package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/P4nos/tcp-tunnel/common"
	"io"
	"log"
	"math/rand"
	"net"
	"sync"
)

type clientConnection struct {
	conn               net.Conn
	clientListener     net.Listener
	clientListenerConn net.Conn
}

type Connections struct {
	mu      sync.Mutex
	entries map[string]*clientConnection
}

func (c *Connections) AddListenerConnection(conn net.Conn, id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[id].clientListenerConn = conn
}

func (c *Connections) AddNewClient(clientConn *clientConnection, id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[id] = clientConn
}

func (c *Connections) ClientDisconnected(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// this shouldn't be necessary since we are evicting the listener
	// after proxying each request
	if listenerConn := c.entries[id].clientListenerConn; listenerConn != nil {
		c.entries[id].clientListenerConn.Close()
	}
	c.entries[id].clientListener.Close()
	c.entries[id].conn.Close()
	log.Println("cleanup client", id)
	delete(c.entries, id)
}

func (c *Connections) evictListenerConnectionForClient(id string) net.Conn {
	c.mu.Lock()
	defer c.mu.Unlock()
	clientConnListener := c.entries[id].clientListenerConn
	c.entries[id].clientListenerConn = nil
	return clientConnListener
}

type ServerConfig struct {
	serverListener net.Listener
	state          *Connections
}

type Server interface {
	createPortListener(c net.Conn, port string)
	handleNewConnection(c net.Conn)
	HandleConnection(c net.Conn)
	Shutdown()
}

func (s *ServerConfig) Shutdown() {
	s.serverListener.Close()
}

func Start() (ServerConfig, error) {
	port := common.ControlPort
	host := "localhost:" + fmt.Sprint(port)
	// move to env vars
	cert, err := tls.LoadX509KeyPair("./localhost.pem", "./localhost-key.pem")
	if err != nil {
		log.Fatal(err)
	}

	cfg := &tls.Config{Certificates: []tls.Certificate{cert}}
	l, err := tls.Listen("tcp", host, cfg)
	if err != nil {
		return ServerConfig{}, err
	}
	log.Println("server listening to port", port)

	state := &Connections{
		entries: make(map[string]*clientConnection),
	}

	sc := ServerConfig{
		serverListener: l,
		state:          state,
	}

	return sc, nil
}

func tryToBindPort() (int, error) {
	// move to env vars
	const min_port_value = 60000
	const max_port_value = 61000
	for _, val := range rand.Perm(max_port_value - min_port_value) {
		port := min_port_value + val

		host := "localhost:" + fmt.Sprint(port)
		l, err := net.Listen("tcp", host)
		if err != nil {
			// assume that port is already in use
			continue
		}
		l.Close()
		return port, nil
	}
	return 0, errors.New("Failed to find available port to bind")
}

func (s *ServerConfig) createPortListener(c net.Conn) {
	clientId := c.RemoteAddr().String()

	// try to pick a random port to listen to.
	port, err := tryToBindPort()
	if err != nil {
		log.Println("Failed to find available port to bind", clientId, err)
		common.SendMessage(c, common.Message{MessageType: common.Error, PayloadSize: 0, Payload: nil})
	}

	host := "localhost:" + fmt.Sprint(port)
	listener, err := net.Listen("tcp", host)
	if err != nil {
		log.Println("failed to create listener for client ", clientId, host, err)
		return
	}

	log.Println("listening for incoming traffic on ", host)
	// store the connection in state in order to be able to read the data from it on separate goroutine
	ingressConn := clientConnection{conn: c, clientListener: listener}
	s.state.AddNewClient(&ingressConn, clientId)

	// create payload for client. Return the server address and the port the server is forwarding from.
	_, err = common.SendMessage(c, common.Message{MessageType: common.Ok, PayloadSize: uint64(len(host)), Payload: []byte(host)})
	if err != nil {
		log.Println("failed to notify client that listener is ready", err)
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
					log.Println("client closed connection", listener.Addr().String())
					return
				}
				log.Println("failed to accept connection ", listener.Addr().String(), err)
				return
			}

			// store the connection object so that we can close it after the request has been forwarded
			// to the client
			s.state.AddListenerConnection(conn, clientId)
			log.Println("new incoming connection for client: ", clientId, conn.LocalAddr().String())

			_, err = common.SendMessage(c, common.Message{MessageType: common.Incoming, PayloadSize: uint64(len(clientId)), Payload: []byte(clientId)})
			if err != nil {
				log.Println("error sending Incoming message to client", err)
			}
		}
	}()
}

func (s *ServerConfig) handleNewConnection(c net.Conn) {
	for {
		message, err := common.ParseMessage(c)
		if err != nil {
			if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
				log.Println("connection error", c.RemoteAddr().String(), err)
				s.state.ClientDisconnected(c.RemoteAddr().String())
				break
			}
			log.Println("failed to parse message ", err)
		}

		switch message.MessageType {
		case common.Connect:
			go s.createPortListener(c)

		case common.Ok:
			connId := string(message.Payload)

			// retrieve the listener connection and proxy data to client
			listenerConn := s.state.evictListenerConnectionForClient(connId)

			common.Pipe(c, listenerConn)
			log.Println("closing proxy connection for client", c.RemoteAddr().String())
			listenerConn.Close()

		default:
			log.Println("received unknown message", message.MessageType)
		}
	}
}

func (s *ServerConfig) HandleConnection() {
	conn, err := s.serverListener.Accept()
	if err != nil {
		log.Fatal(err)
	}
	// Handle multiple concurrent connections
	go s.handleNewConnection(conn)
}
