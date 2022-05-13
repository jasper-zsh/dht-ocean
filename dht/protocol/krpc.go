package protocol

import (
	"dht-ocean/bencode"
	"fmt"
)

type Packet struct {
	data map[string]any
}

func NewPacket() *Packet {
	return &Packet{
		data: make(map[string]any),
	}
}

func NewPacketFromBuffer(buf []byte) (*Packet, error) {
	ret := &Packet{}
	dict, err := bencode.BDecode(buf)
	if err != nil {
		return nil, err
	}
	switch dict.(type) {
	case map[string]any:
		ret.data = dict.(map[string]any)
		return ret, nil
	default:
		return nil, fmt.Errorf("illegal packet: %s", buf)
	}
}

func (p *Packet) Encode() (string, error) {
	return bencode.BEncode(p.data)
}

func (p *Packet) SetT(tid []byte) {
	p.data["t"] = tid
}

func (p *Packet) SetY(y string) {
	p.data["y"] = y
}

func (p *Packet) SetV(v string) {
	p.data["v"] = v
}

func (p *Packet) Set(key string, value any) {
	p.data[key] = value
}

func (p *Packet) SetError(code int, msg string) {
	p.data["e"] = []any{code, msg}
}

func (p *Packet) GetT() []byte {
	t, ok := p.data["t"]
	if ok {
		return t.([]byte)
	} else {
		return nil
	}
}

func (p *Packet) GetY() string {
	t, ok := p.data["y"]
	if ok {
		return t.(string)
	} else {
		return ""
	}
}

func (p *Packet) Get(key string) interface{} {
	t, ok := p.data[key]
	if !ok {
		return nil
	}
	return t
}

func printAny(item interface{}, prefix string) {
	switch item.(type) {
	case string:
		fmt.Printf("%X\n", item)
	case map[string]any:
		fmt.Print("{\n")
		printMap(item.(map[string]any), prefix+"  ")
		fmt.Printf("%s}\n", prefix)
	case []any:
		fmt.Print("[\n")
		for _, e := range item.([]interface{}) {
			printAny(e, prefix+"  ")
		}
	default:
		fmt.Printf("%X\n", item)
	}
}

func printMap(m map[string]any, prefix string) {
	for k, v := range m {
		fmt.Printf("%s%s: ", prefix, k)
		printAny(v, prefix+"  ")
	}
}

func (p *Packet) Print() {
	printMap(p.data, "")
}
