package main

import (
	"fmt"
	"runtime"

	"github.com/OperatorFoundation/AdversaryLab/storage"
	"github.com/OperatorFoundation/AdversaryLab/services"
)

// This is the server class that receives the training packets and sends out rules to subscribers.

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	// channel of best rule updates ("dataset1-incoming" + rule candidate)
	updates := make(chan services.Update, 100)

	fmt.Println("*** INIT")

	// map from keys ("dataset1-incoming") to the store containing the training packet
	// payloads (both index and source files).
	// later on, keys ("dataset1-incoming") are reset to map to the store containing
	// the offset/subsequence pairs (both index and source files, but not countmap file).
	// need to investigate the storeCache a bit more because it seems worrying that the
	// map could be changing each time a new training packet is sent or a new best rule is
	// generated.
	storeCache := storage.NewStoreCache()

	train := services.NewTrainPacketService("tcp://localhost:4567", updates, storeCache)
	//	test := services.NewTestPacketService("tcp://localhost:4569", updates)
	fmt.Println("2")
	rule := services.NewRuleService("tcp://localhost:4568", updates, storeCache)

	fmt.Println("*** RUN")

	go train.Run()
	//	go test.Run()
	rule.Run()

	fmt.Println("*** FINISHED")
}
