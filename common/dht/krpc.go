package dht

import (
	bencode2 "dht-ocean/common/bencode"
	"fmt"
	"net"
	"strings"
)

type Packet struct {
	buf  []byte
	Data map[string]any
	Addr *net.UDPAddr
}

func NewPacket() *Packet {
	pkt := &Packet{
		Data: make(map[string]any),
	}
	//pkt.SetV("AG20")
	return pkt
}

func NewEmptyResponsePacket(nodeID []byte) *Packet {
	pkt := NewPacket()
	pkt.SetY("r")
	pkt.Set("r", map[string]any{
		"id": nodeID,
	})
	return pkt
}

func NewPacketFromBuffer(buf []byte) *Packet {
	return &Packet{buf: buf}
}

func (p *Packet) Decode() error {
	dict, err := bencode2.BDecode(p.buf)
	if err != nil {
		return err
	}
	switch dict.(type) {
	case map[string]any:
		p.Data = dict.(map[string]any)
		return nil
	default:
		return fmt.Errorf("illegal packet: %s", p.buf)
	}
}

func (p *Packet) Encode() (string, error) {
	ret, err := bencode2.BEncode(p.Data)
	if err != nil {
		return "", err
	}
	p.buf = []byte(ret)
	return ret, nil
}

func (p *Packet) SetT(tid []byte) {
	p.Data["t"] = tid
}

func (p *Packet) SetY(y string) {
	p.Data["y"] = y
}

func (p *Packet) SetV(v string) {
	p.Data["v"] = v
}

func (p *Packet) Set(key string, value any) {
	p.Data[key] = value
}

func (p *Packet) SetError(code int, msg string) {
	p.Data["e"] = []any{code, msg}
}

func (p *Packet) GetT() []byte {
	t, ok := p.Data["t"]
	if ok {
		return t.([]byte)
	} else {
		return nil
	}
}

func (p *Packet) GetY() string {
	t, ok := p.Data["y"]
	if ok {
		return string(t.([]byte))
	} else {
		return ""
	}
}

func (p *Packet) GetQ() string {
	q, ok := p.Data["q"]
	if ok {
		return string(q.([]byte))
	} else {
		return ""
	}
}

func (p *Packet) Get(key string) interface{} {
	t, ok := p.Data[key]
	if !ok {
		return nil
	}
	return t
}

func printAny(item interface{}, prefix string) string {
	str := &strings.Builder{}
	switch item.(type) {
	case string:
		str.WriteString(fmt.Sprintf("%X\n", item))
	case map[string]any:
		str.WriteString(fmt.Sprintf("{\n"))
		str.WriteString(printMap(item.(map[string]any), prefix+"  "))
		str.WriteString(fmt.Sprintf("%s}\n", prefix))
	case []any:
		str.WriteString(fmt.Sprint("[\n"))
		for _, e := range item.([]interface{}) {
			str.WriteString(printAny(e, prefix+"  "))
		}
	case int:
		str.WriteString(fmt.Sprintf("%d\n", item))
	default:
		str.WriteString(fmt.Sprintf("%s\n", item))
	}
	return str.String()
}

func printMap(m map[string]any, prefix string) string {
	str := &strings.Builder{}
	for k, v := range m {
		str.WriteString(fmt.Sprintf("%s%s: ", prefix, k))
		str.WriteString(printAny(v, prefix+"  "))
	}
	return str.String()
}

func (p *Packet) String() string {
	return printMap(p.Data, "")
}

func (p *Packet) Size() int {
	return len(p.buf)
}
