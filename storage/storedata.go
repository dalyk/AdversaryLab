package storage

import (
	"encoding/binary"
	"fmt"
	"os"
)

// StoreData contains data derived from inputs
type StoreData struct {
	Last int64
}

// Save saves StoreData to storage
func (self *StoreData) Save(path string) error {
	fmt.Println("Saving...", self.Last)
	output, err := os.OpenFile("store/"+path+"/derived", os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		return err
	}

	buff := make([]byte, 8)
	binary.PutVarint(buff, self.Last)
	output.Write(buff)

	output.Close()

	return nil
}
