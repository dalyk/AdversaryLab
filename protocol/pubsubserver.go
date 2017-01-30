package protocol

import (
	"github.com/go-mangos/mangos"
	"github.com/go-mangos/mangos/protocol/pub"
	"github.com/go-mangos/mangos/transport/tcp"
)

type PubsubSource chan []byte

type PubsubServer struct {
	sock   mangos.Socket
	source PubsubSource
}

// Set up pub/sub server that will send out rule updates from the PubsubSource byte channel.
func PubsubListen(url string, source PubsubSource) PubsubServer {
	var sock mangos.Socket
	var err error

	if sock, err = pub.NewSocket(); err != nil {
		die("can't get new rep socket: %s", err)
	}

	sock.AddTransport(tcp.NewTransport())
	if err = sock.Listen(url); err != nil {
		die("can't listen on rep socket: %s", err.Error())
	}

	return PubsubServer{
		sock:   sock,
		source: source,
	}
}

// Continuously reads from the PubsubSource and sends the data to the pub socket to send.
func (self PubsubServer) Pump() {
	for bs := range self.source {
		//		fmt.Println("pumping")
		self.sock.Send(bs)
		//		fmt.Println("pumped")
	}
}
