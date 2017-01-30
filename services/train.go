package services

import (
	"fmt"

	"github.com/ugorji/go/codec"

	"github.com/OperatorFoundation/AdversaryLab/storage"
	"github.com/OperatorFoundation/AdversaryLab/protocol"
)

type Handlers struct {
	handlers   map[string]*StoreHandler		// keys are i.e. "dateset1-incoming"
	updates    chan Update				// channel of best rule candidates
	storeCache *storage.StoreCache			// map of all stores for received data (not sequences)
}

// StoreHandler is a request handler that knows about storage
type StoreHandler struct {
	path  string					// i.e. "dataset1-incoming"
	store *storage.Store				// store for received data (not sequences)
	//	seqs          *storage.SequenceMap
	offseqs       *storage.OffsetSequenceMap	// struct containing store with sequence files, ctrie, best rule, update channel
	updates       chan Update			// channel of best rule updates ("dataset1-incoming' + best rule candidate)
	ruleUpdates   chan *storage.RuleCandidate	// channel of best rule candidates
	handleChannel chan *protocol.TrainPacket	// channel of decoded training packets; these get added to the store and processed
}

type TrainService struct {
	handlers Handlers
	serve    protocol.Server	// contains the socket for listening for training packets
}

type Update struct {
	Path string
	Rule *storage.RuleCandidate
}

// The server side that receives training packets
// Listen address is set up to be tcp://localhost:4567
func NewTrainPacketService(listenAddress string, updates chan Update, storeCache *storage.StoreCache) *TrainService {
	handlers := Handlers{handlers: make(map[string]*StoreHandler), updates: updates, storeCache: storeCache}
	// files, err := ioutil.ReadDir("store")
	// if err != nil {
	// 	fmt.Println("Failed to read store directory", err)
	// } else {
	// 	for _, file := range files {
	// 		handlers.Load(file.Name())
	// 	}
	// }

	// Sets up the socket for listening for training packets
	serve := protocol.Listen(listenAddress)

	return &TrainService{handlers: handlers, serve: serve}
}

// Goroutine spawned by the AdversaryLab/server.go that listens for and handles training packets.
func (self *TrainService) Run() {
	// Continuously accept training packets.
	for {
		//		fmt.Println("accepting reqresp")
		self.serve.Accept(self.handlers.Handle)
		//		fmt.Println("accepted reqresp")
	}
}

// Return the corresponding store handler if it already exists, otherwise create one.
func (self Handlers) Load(name string) *StoreHandler {
	var err error

	if handler, ok := self.handlers[name]; ok {
		return handler
	} else {
		store := self.storeCache.Get(name)
		// If the desired store (i.e. dataset1-incoming) doesn't already exist,
		// create a store for it and store it in the storeCache.
		if store == nil {
			store, err = storage.OpenStore(name)
			if err != nil {
				fmt.Println("Error opening store")
				fmt.Println(err)
				return nil
			}

			self.storeCache.Put(name, store)
		}

		// sm, err2 := storage.NewSequenceMap(name)
		// if err2 != nil {
		// 	fmt.Println("Error opening bytemap")
		// 	fmt.Println(err2)
		// 	return nil
		// }

		// Channel for passing best rule candidate updates.
		ruleUpdates := make(chan *storage.RuleCandidate, 10)

		osm, err2 := storage.NewOffsetSequenceMap(name, ruleUpdates)
		if err2 != nil {
			fmt.Println("Error opening bytemap")
			fmt.Println(err2)
			return nil
		}

		handleChannel := make(chan *protocol.TrainPacket)

		handler := &StoreHandler{path: name, store: store, offseqs: osm, updates: self.updates, ruleUpdates: ruleUpdates, handleChannel: handleChannel}
		handler.Init()
		self.handlers[name] = handler
		return handler
	}
}

// Handles a new training packet on the server side.  This function is called on packets that
// are received and performs the necessary decoding.
func (self Handlers) Handle(request []byte) []byte {
	//	fmt.Println("New packet")
	var name string

	var value = protocol.NamedType{}
	var h = protocol.NamedTypeHandle()
	var dec = codec.NewDecoderBytes(request, h)
	var err = dec.Decode(&value)
	if err != nil {
		fmt.Println("Failed to decode")
		fmt.Println(err.Error())
		return []byte("success")
	}

	switch value.Name {
	case "protocol.TrainPacket":
		//		fmt.Println("Got packet")
		packet := protocol.TrainPacketFromMap(value.Value.(map[interface{}]interface{}))
		if packet.Incoming {
			name = packet.Dataset + "-incoming"
		} else {
			name = packet.Dataset + "-outgoing"
		}

		// Get the handler for packets of this dataset-incoming/outgoing and pass the training
		// packet onto the handler's channel.
		handler := self.Load(name)
		if handler != nil {
			handler.handleChannel <- &packet
			return []byte("success")
		} else {
			fmt.Println("Could not load handler for", name)
			return []byte("success")
		}
	default:
		fmt.Println("Unknown request type")
		fmt.Println(value)
		fmt.Println("<.>")
		return []byte("success")
	}
}

// Init process all items that are already in storage
func (self *StoreHandler) Init() {
	//	fmt.Println("Loading")
	// FIXME - loading of Last value
	//	self.Load()
	//	fmt.Println("Processing")
	go self.HandleChannel(self.handleChannel)
	go self.HandleRuleUpdatesChannel(self.ruleUpdates)
	//	self.store.FromIndexDo(self.store.LastIndex(), self.processChannel)
}

// Handle training packets received on the server that have been decoded.
func (self *StoreHandler) HandleChannel(ch chan *protocol.TrainPacket) {
	for request := range ch {
		if !storage.Debug {
			fmt.Print(".")
		}
		self.Handle(request)
	}
}

// Handle best rule candidate updates that result from processing the training packets.
func (self *StoreHandler) HandleRuleUpdatesChannel(ch chan *storage.RuleCandidate) {
	for rule := range ch {
		update := Update{Path: self.path, Rule: rule}
		//		fmt.Println("training sending update", update)
		self.updates <- update
	}
}

// Handle handles requests (training packets sent from the client).  First adds the payout to the
// store and then sends the record on to the processor.
func (self *StoreHandler) Handle(request *protocol.TrainPacket) []byte {
	// Add the payload (the byte array) to the store (both the source file and index file)
	index := self.store.Add(request.Payload)
	record, err := self.store.GetRecord(index)	// checking that record was recorded correctly
	if err != nil {
		fmt.Println("Error getting new record", err)
	} else {
		self.Process(request.AllowBlock, record)
	}

	return []byte("success")
}

// Processes records (training data). Results in rules being put on update channle.
func (self *StoreHandler) Process(allowBlock bool, record *storage.Record) {
	//	fmt.Println("Processing", record.Index)

	// For records that haven't been processed yet, they have just been added to the
	// store, so record.Index should equal self.store.LastIndex().
	if record.Index < self.store.LastIndex() {
		fmt.Println("Rejecting duplicate", record.Index, "<", self.store.LastIndex())
		return
	}

	// FIXME - process bytes into bytemaps

	self.processBytes(allowBlock, record.Data)
}

// Helper function for processing records (training data).
func (self *StoreHandler) processBytes(allowBlock bool, bytes []byte) {
	//	self.seqs.ProcessBytes(allowBlock, bytes)
	self.offseqs.ProcessBytes(allowBlock, bytes)
}
