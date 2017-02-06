package protocol

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
	Dataset       string	// i.e. "dataset1"
	RequireForbid bool	// true if rule should be used for allowing.
	Incoming      bool	// whether or not this rule is for incoming or outgoing traffic.
	Sequence      []byte	// offset (2 bytes) and rest of byte subsequence concatenated
}

type ResultStatus int

// The structs are decoded as interfaces, so need to convert them back into structs.
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
