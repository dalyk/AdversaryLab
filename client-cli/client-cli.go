package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/deckarep/golang-set"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"

	"github.com/OperatorFoundation/AdversaryLab/protocol"
	"io"
)

// This client command line interface supports both capturing packets and displaying the
// generated rules.

// Note that each tab in a browser is connect to a different client port.  The server
// ports will be 80 (http) or 443 (https) for web servers.
type Connection struct {
	src layers.TCPPort	// source
	dst layers.TCPPort	// destination
}

// Create a connection struct that records the src and dst ports.
func NewConnection(packet *layers.TCP) Connection {
	return Connection{src: packet.SrcPort, dst: packet.DstPort}
}

// Return true if either the src or dst port matches the requested port.
func (conn Connection) CheckPort(port layers.TCPPort) bool {
	return conn.src == port || conn.dst == port
}

func main() {
	var mode string
	var captureName string
	var dataset string

	// If the dataset name was not specified, print out usage help
	if len(os.Args) < 3 {
		usage()
	}

	// mode is "capture" or "rules"
	mode = os.Args[1]

	if mode == "capture" {
		dataset = os.Args[2]

		var allowBlock bool = false
		if os.Args[3] == "allow" {
			allowBlock = true
		}

		if len(os.Args) > 4 {
			// The desired port to listen on is known
			capture(dataset, allowBlock, &os.Args[4])
		} else {
			capture(dataset, allowBlock, nil)
		}
	} else if mode == "rules" {
		// Note that captureName is never initialized.
		rules(captureName)
	} else {
		// Print usage help.
		usage()
	}
}

// Capture packets for training. Classify as a specific dataset and allow/block on a given port.
func capture(dataset string, allowBlock bool, port *string) {
	var lab protocol.Client
	var err error
	var input string

	// Package bufio implements buffered I/O. It wraps an io.Reader or io.Writer object,
	// creating another object (Reader or Writer) that also implements the interface but
	// provides buffering.
	// os.Stdin is for reading from the console
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Launching training packet client...")

	// Creates the client socket for sending training packets.  Is connected to the server
	// socket listening on tcp://localhost:4567.
	lab = protocol.Connect("tcp://localhost:4567")

	// Records captured packets as the values in a map, where keys are the dst/src port.  This
	// map structure is used rather than a set to retain packages captured when the desired
	// port is yet to be specified (we must be able to delete packages with non-requested ports
	// once the requested port has been specified).
	captured := map[Connection]gopacket.Packet{}

	// OpenLive opens a device and returns a *Handle.
	// It takes as arguments the name of the device ("eth0"), the maximum size to
	// read for each packet (snaplen), whether to put the interface in promiscuous
	// mode, and a timeout.
	// Handle provides a connection to a pcap handle, allowing users to read packets
	// off the wire (Next), inject packets onto the wire (Inject), and
	// perform a number of other functions to affect and understand packet output.
	handle, pcapErr := pcap.OpenLive("em1", 1024, false, 30*time.Second)
	if pcapErr != nil {
		handle.Close()
		os.Exit(1)
	}

	// gopacket takes in packet data as a []byte and decodes it into a packet with a
	// non-zero number of "layers". Each layer corresponds to a protocol within the
	// bytes. Once a packet has been decoded, the layers of the packet can be requested
	// from the packet.
	// Once you have a PacketDataSource, you can pass it into NewPacketSource, along
	// with a Decoder of your choice, to create a PacketSource.
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	// packetChannel captures all packets through the requested network device without
	// regard for the src/dst ports of the packets.
	packetChannel := make(chan gopacket.Packet)
	// start a goroutine that reads packets from the source and puts them on the packetChannel
	go readPackets(packetSource, packetChannel)

	stopDetecting := make(chan bool)
	// Package mapset implements a simple and generic set collection.
	// Items stored within it are unordered and unique. It supports
	// typical set operations: membership testing, intersection, union,
	// difference, symmetric difference and cloning.
	ports := mapset.NewSet()
	// Determine the set of ports seen on all traffic and record them in the captured map. Do
	// not yet send them out since some packets may involve non-requested ports. The stopDetecting
	// channel will immediately be true for cases where the port number is already part of the
	// original command.
	go detectPorts(ports, packetChannel, captured, stopDetecting)

	var selectedPort layers.TCPPort
	var temp uint64

	// Get the port at which to capture training data, either by using the
	// provided port or by asking the user to select a port from the ports
	// with detected traffic.
	if port == nil {
		fmt.Println("Press Enter to see ports.")
		input, _ = reader.ReadString('\n')
		stopDetecting <- true
		fmt.Println()

		portObjs := ports.ToSlice()
		fmt.Println(portObjs)

		fmt.Println("Enter port to capture:")
		input, _ = reader.ReadString('\n')
	} else {
		input = *port
		stopDetecting <- true
	}

	// Finalize the selected port.
	temp, err = strconv.ParseUint(strings.TrimSpace(input), 10, 16)
	CheckError(err)
	selectedPort = layers.TCPPort(temp) // cast the int to the tcpport type.

	fmt.Println("Read port.")

	fmt.Println("Selected port", selectedPort)

	// Remove captured packets that don't involve the selected port.
	discardUnusedPorts(selectedPort, captured)

	stopCapturing := make(chan bool)
	recordable := make(chan gopacket.Packet) // channel that will carry packets with the selected port.
	go capturePort(selectedPort, packetChannel, captured, stopCapturing, recordable)
	go saveCaptured(lab, dataset, allowBlock, stopCapturing, recordable, selectedPort)

	fmt.Println("Press Enter to stop capturing.")
	_, _ = reader.ReadString('\n')
	stopCapturing <- true
	fmt.Println()

	handle.Close()
	os.Exit(0)
}

// Print out ways to use the client command line.
func usage() {
	fmt.Println("client-cli capture [protocol] [dataset] <port>")
	fmt.Println("Example: client-cli capture testing allow")
	fmt.Println("Example: client-cli capture testing allow 80")
	fmt.Println("Example: client-cli capture testing block")
	fmt.Println("Example: client-cli capture testing block 443")
	fmt.Println()
	fmt.Println("client-cli rules [protocol]")
	fmt.Println("Example: client-client rules HTTP")
	os.Exit(1)
}

// Example:
// {"OpenVPN" : {
//   "name":"OpenVPN",
//   "target":"OpenVPN",
//   "byte_sequences" : [
//      {"rule_type":"adversary labs",
//       "action":"block",
//       "outgoing": [72, 84, 84, 80, 47, 49, 46, 49, 32, 50, 48, 48, 32,
// 79, 75, 13, 10],
//       "incoming": [71, 69, 84, 32, 47]}]}}
type RuleSet struct {
	name           string
	target         string
	byte_sequences []Rule
}

type Rule map[string]interface{}

// captureName is never set. Listen for rules as a subscriber, and create and update a cache
// of both incoming and outgoing rules for each dataset.
func rules(captureName string) {
	var lab protocol.PubsubClient

	lab = protocol.PubsubConnect("tcp://localhost:4568")	// returns both client socket and decoded rules chanel

	// Make a map that will have dataset keys (ex. "dataset1") mapping to values that are 2d arrays.
	// The first row in the array is the incoming rule sequence (offset+byte subsequence)
	// The second row in the array is the outgoing rule sequence (offset+byte subsequence)
	cache := make(map[string][2][]byte)

	// Iterate through the channel of decoded rules
	for currentRule := range lab.Rules {
		name := currentRule.Dataset	// ex. "dataset1"

		var entry [2][]byte
		var ok bool

		// If the cache doesn't already have a rule for this dataset, initialize the arrays in the value.
		if entry, ok = cache[name]; !ok {
			entry = [2][]byte{make([]byte, 0), make([]byte, 0)}
		}

		// Place the rule sequence in the appropriate row of the entry, overwriting if necessary.
		if currentRule.Incoming {
			entry[0] = currentRule.Sequence
		} else {
			entry[1] = currentRule.Sequence
		}

		cache[name] = entry

		// Convert the bytes in the rules to ints for better readability.
		outgoingBytes := entry[1]
		outgoingInts := make([]int, len(outgoingBytes))
		for index, value := range outgoingBytes {
			outgoingInts[index] = int(value)
		}

		incomingBytes := entry[0]
		incomingInts := make([]int, len(incomingBytes))
		for index, value := range incomingBytes {
			incomingInts[index] = int(value)
		}

		// FIXME - use RequireForbid field
		// Note that this marks all rules as block (undesirable)
		rule := make(map[string]interface{}, 4)
		rule["rule_type"] = "adversary labs"
		rule["action"] = "block"
		rule["outgoing"] = outgoingInts
		rule["incoming"] = incomingInts

		rules := make([]Rule, 1)
		rules[0] = rule

		data := make(map[string]interface{}, 3)
		data["name"] = name			// ex. "dataset1"
		data["target"] = name
		data["byte_sequences"] = rules

		top := make(map[string]interface{}, 1)
		top[captureName] = data			// captureName is always "" currently

		// Marshal returns the JSON encoding of v. Struct values encode as JSON objects.
		encoded, err := json.Marshal(top)
		CheckError(err)

		// Print out the rule.
		fmt.Println(string(encoded))
	}
}

/* A Simple function to verify error */
func CheckError(err error) {
	if err != nil {
		fmt.Println("Error: ", err)
		os.Exit(0)
	}
}

// Determine the set of ports (both src and dst) that are being used in incoming/outgoing traffic through
// the selected network device (all packets are arriving on the packetChannel).  Packets are also being
// stored in the map captured with a key that records the src and dst port.
func detectPorts(ports mapset.Set, packetChannel chan gopacket.Packet, captured map[Connection]gopacket.Packet, stopDetecting chan bool) {
	for {
		select {
		case <-stopDetecting:		// when the user has hit enter, stop recording port options
			return
		case packet := <-packetChannel: // analyze all packets arriving/leaving through the network device
			//fmt.Println(ports)
			fmt.Print(".")

			// Let's see if the packet is TCP
			tcpLayer := packet.Layer(layers.LayerTypeTCP)
			if tcpLayer != nil {
				//		        fmt.Println("TCP layer detected.")
				tcp, _ := tcpLayer.(*layers.TCP)

				if !ports.Contains(tcp.SrcPort) {
					ports.Add(tcp.SrcPort)
				}

				if !ports.Contains(tcp.DstPort) {
					ports.Add(tcp.DstPort)
				}

				// Store the seen packets but do not send them out
				recordPacket(packet, captured, nil)
			} else {
				//				fmt.Println("No TCP")
				//				fmt.Println(packet)
			}
		}
	}
}

// Send packets with the requested port onto the recordable channel until the user stops the capturing. Also send
// any packets that were already captured during port detection that match the requested port.
func capturePort(port layers.TCPPort, packetChannel chan gopacket.Packet, captured map[Connection]gopacket.Packet, stopCapturing chan bool, recordable chan gopacket.Packet) {
	fmt.Println("Capturing port", port)

	// Start the count of packets with the correct port that are going to be sent to the recordable channel.
	var count uint16 = uint16(len(captured))

	// Send out any packets with the correct port that were already captured during port detection.
	for _, packet := range captured {
		recordable <- packet
	}

	for {
		//		fmt.Println("capturing...", port, count)
		select {
		case <-stopCapturing:
			return
		case packet := <-packetChannel:
			//			fmt.Print(".")
			//				fmt.Println(packet)

			// Let's see if the packet is TCP
			tcpLayer := packet.Layer(layers.LayerTypeTCP)
			app := packet.ApplicationLayer()
			if tcpLayer != nil && app != nil {
				//		        fmt.Println("TCP layer captured.")
				tcp, _ := tcpLayer.(*layers.TCP)

				conn := NewConnection(tcp)
				if !conn.CheckPort(layers.TCPPort(port)) {
					continue
				}

				recordPacket(packet, captured, recordable)

				newCount := uint16(len(captured))
				if newCount > count {
					count = newCount
					fmt.Print(count)
				}
			} else {
				// fmt.Println("No TCP")
				// fmt.Println(packet)
			}
		}
	}
}

func readPackets(packetSource *gopacket.PacketSource, packetChannel chan gopacket.Packet) {
	//	fmt.Println("reading packets")
	for packet := range packetSource.Packets() {
		//		fmt.Println("readPacket")
		packetChannel <- packet
	}
	//	fmt.Println("done reading packets")
}

// Remove already captured packets that don't have a src/dst port matching the desired port
// on which to capture traffic.
func discardUnusedPorts(port layers.TCPPort, captured map[Connection]gopacket.Packet) {
	for conn := range captured {
		if !conn.CheckPort(port) {
			// The delete built-in function deletes the element with the specified key
			// (m[key]) from the map. If m is nil or there is no such element, delete
			// is a no-op.
			delete(captured, conn)
		}
	}
}

// Store the captured packet in the captured map, which uses the src/dst port as the key.
func recordPacket(packet gopacket.Packet, captured map[Connection]gopacket.Packet, recordable chan gopacket.Packet) {
	tcpLayer := packet.Layer(layers.LayerTypeTCP)
	if tcpLayer != nil {
		//		fmt.Println("TCP layer recorded.")
		tcp, _ := tcpLayer.(*layers.TCP)
		conn := NewConnection(tcp)
		_, ok := captured[conn]
		// Save first packet only for each new connection
		if !ok {
			captured[conn] = packet
			// recordable is nil when ports are being detected and one hasn't been selected yet.
			if recordable != nil {
				fmt.Print(".")
				recordable <- packet
			}
		}
	}
}

// Send the application payload from captured packets with the correct port to the server socket by adding
// the data as a training packet. Also determine if packet was incoming/outgoing by comparing the destination
// port to the request port.
func saveCaptured(lab protocol.Client, dataset string, allowBlock bool, stopCapturing chan bool, recordable chan gopacket.Packet, port layers.TCPPort) {
	fmt.Println("Saving captured byte sequences... ")

	for {
		select {
		case <-stopCapturing:
			return // FIXME - empty channel of pending packets, but don't block
		case packet := <-recordable:
			fmt.Print("*")
			if app := packet.ApplicationLayer(); app != nil {
				fmt.Print("$")
				incoming := packet.Layer(layers.LayerTypeTCP).(*layers.TCP).DstPort == port
				data := app.Payload()
				fmt.Println()
				fmt.Println(data)
				lab.AddTrainPacket(dataset, allowBlock, incoming, data)
			}
		}
	}
}
