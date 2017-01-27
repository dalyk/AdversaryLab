package protocol

import (
	"github.com/ugorji/go/codec"
)

type TrainPacket struct {
	Dataset    string
	AllowBlock bool
	Incoming   bool
	Payload    []byte
}

type TestPacket struct {
	Dataset  string
	Incoming bool
	Payload  []byte
}

type RuleRequest struct {
	Dataset  string
	Incoming bool
}

type Rule struct {
	Dataset       string
	RequireForbid bool
	Incoming      bool
	Sequence      []byte
}

type ResultStatus int

func TrainPacketFromMap(data map[interface{}]interface{}) TrainPacket {
	packet := TrainPacket{}
	packet.Dataset = data["Dataset"].(string)
	packet.AllowBlock = data["AllowBlock"].(bool)
	packet.Incoming = data["Incoming"].(bool)
	packet.Payload = data["Payload"].([]byte)
	return packet
}

func RuleFromMap(data map[interface{}]interface{}) Rule {
	rule := Rule{}
	rule.Dataset = data["Dataset"].(string)
	rule.RequireForbid = data["RequireForbid"].(bool)
	rule.Incoming = data["Incoming"].(bool)
	rule.Sequence = data["Sequence"].([]byte)
	return rule
}
