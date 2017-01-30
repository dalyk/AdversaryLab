package protocol

import (
	"github.com/go-mangos/mangos"
	"github.com/go-mangos/mangos/protocol/rep"
	"github.com/go-mangos/mangos/transport/tcp"
)

type Responder func([]byte) []byte

type Server struct {
	sock mangos.Socket
}

// Sets up the server-side socket for receiving training packets on tcp://localhost:4567.
func Listen(url string) Server {
	var sock mangos.Socket
	var err error

	if sock, err = rep.NewSocket(); err != nil {
		die("can't get new rep socket: %s", err)
	}

	sock.AddTransport(tcp.NewTransport())
	if err = sock.Listen(url); err != nil {
		die("can't listen on rep socket: %s", err.Error())
	}

	return Server{
		sock: sock,
	}
}

// Runs in a continuous for loop to accept incoming training packets.  The responder input
// is a function that accepts a byte array and returns a byte array.
func (self Server) Accept(responder Responder) []byte {
	var err error
	var msg []byte
	var response []byte

	// Could also use sock.RecvMsg to get header
	msg, err = self.sock.Recv()
	//	fmt.Println("server received request:", string(msg))

	// Handle the received training packet and send the transformation back to the client
	// as the reply.
	response = responder(msg)
	err = self.sock.Send(response)
	if err != nil {
		die("can't send reply: %s", err.Error())
	}

	// Return the original received training packet (a byte array).
	return msg
}
