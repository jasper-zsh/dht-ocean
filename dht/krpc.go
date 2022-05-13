package dht

import (
	"fmt"
	"github.com/elliotchance/orderedmap"
)

type Packet struct {
	data *orderedmap.OrderedMap
}

func NewPacket() *Packet {
	return &Packet{
		data: orderedmap.NewOrderedMap(),
	}
}

func NewPacketFromBuffer(buf []byte) (*Packet, error) {
	ret := &Packet{}
	dict, err := BDecode(buf)
	if err != nil {
		return nil, err
	}
	switch dict.(type) {
	case *orderedmap.OrderedMap:
		ret.data = dict.(*orderedmap.OrderedMap)
		return ret, nil
	default:
		return nil, fmt.Errorf("illegal packet: %s", buf)
	}
}

func (p *Packet) Encode() (string, error) {
	return BEncode(p.data)
}

func (p *Packet) SetT(tid []byte) {
	p.data.Set("t", tid)
}

func (p *Packet) SetY(y string) {
	p.data.Set("y", y)
}

func (p *Packet) SetV(v string) {
	p.data.Set("v", v)
}

func (p *Packet) SetKey(key string, value interface{}) {
	p.data.Set(key, value)
}

func (p *Packet) SetError(code int, msg string) {
	p.data.Set("e", BTList{code, msg})
}

func (p *Packet) GetT() []byte {
	t, ok := p.data.Get("t")
	if ok {
		return t.([]byte)
	} else {
		return nil
	}
}

func (p *Packet) GetY() string {
	t, ok := p.data.Get("y")
	if ok {
		return t.(string)
	} else {
		return ""
	}
}

func (p *Packet) Get(key string) interface{} {
	t, ok := p.data.Get(key)
	if !ok {
		return nil
	}
	return t
}

func printAny(item interface{}, prefix string) {
	switch item.(type) {
	case string:
		fmt.Printf("%X\n", item)
	case *orderedmap.OrderedMap:
		fmt.Print("{\n")
		printOrderedMap(item.(*orderedmap.OrderedMap), prefix+"  ")
		fmt.Printf("%s}\n", prefix)
	case BTList, []interface{}:
		fmt.Print("[\n")
		for _, e := range item.([]interface{}) {
			printAny(e, prefix+"  ")
		}
	default:
		fmt.Printf("%X\n", item)
	}
}

func printOrderedMap(m *orderedmap.OrderedMap, prefix string) {
	for el := m.Front(); el != nil; el = el.Next() {
		fmt.Printf("%s%s: ", prefix, el.Key)
		printAny(el.Value, prefix+"  ")
	}
}

func (p *Packet) Print() {
	printOrderedMap(p.data, "")
}
