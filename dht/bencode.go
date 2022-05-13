package dht

import (
	"errors"
	"fmt"
	"github.com/elliotchance/orderedmap"
	"strconv"
	"strings"
)

type BTList []interface{}

func BEncode(obj interface{}) (string, error) {
	builder := strings.Builder{}
	err := encodeAny(&builder, obj)
	if err != nil {
		return "", err
	}
	return builder.String(), nil
}

func BDecode(buf []byte) (interface{}, error) {
	ret, _, err := decodeAny(buf, 0)
	return ret, err
}

func encodeInt(builder *strings.Builder, val int) {
	builder.WriteByte('i')
	builder.WriteString(strconv.Itoa(val))
	builder.WriteByte('e')
}

func encodeString(builder *strings.Builder, val string) {
	builder.WriteString(strconv.Itoa(len(val)))
	builder.WriteByte(':')
	builder.WriteString(val)
}

func encodeBytes(builder *strings.Builder, data []byte) {
	builder.WriteString(strconv.Itoa(len(data)))
	builder.WriteByte(':')
	builder.Write(data)
}

func encodeMap(builder *strings.Builder, m map[string]interface{}) error {
	builder.WriteByte('d')
	for k, v := range m {
		encodeString(builder, k)
		err := encodeAny(builder, v)
		if err != nil {
			return err
		}
	}
	return nil
}

func encodeOrderedMap(builder *strings.Builder, m *orderedmap.OrderedMap) error {
	builder.WriteByte('d')
	for el := m.Front(); el != nil; el = el.Next() {
		encodeString(builder, el.Key.(string))
		err := encodeAny(builder, el.Value)
		if err != nil {
			return err
		}
	}
	builder.WriteByte('e')
	return nil
}

func encodeAny(builder *strings.Builder, item interface{}) error {
	switch item.(type) {
	case int:
		encodeInt(builder, item.(int))
	case string:
		encodeString(builder, item.(string))
	case []byte:
		encodeBytes(builder, item.([]byte))
	case map[string]interface{}:
		err := encodeMap(builder, item.(map[string]interface{}))
		if err != nil {
			return err
		}
	case BTList:
		err := encodeList(builder, item.(BTList))
		if err != nil {
			return err
		}
	case *orderedmap.OrderedMap:
		err := encodeOrderedMap(builder, item.(*orderedmap.OrderedMap))
		if err != nil {
			return err
		}
	default:
		return errors.New("unsupported type")
	}
	return nil
}

func encodeList(builder *strings.Builder, list BTList) error {
	builder.WriteByte('l')
	for _, item := range list {
		err := encodeAny(builder, item)
		if err != nil {
			return err
		}
	}
	builder.WriteByte('e')
	return nil
}

func decodeAny(buf []byte, pos int) (interface{}, int, error) {
	switch buf[pos] {
	case 'i':
		return decodeInt(buf, pos)
	case '1', '2', '3', '4', '5', '6', '7', '8', '9', '0':
		return decodeBytes(buf, pos)
	case 'l':
		return decodeList(buf, pos)
	case 'd':
		return decodeMap(buf, pos)
	default:
		return nil, 0, fmt.Errorf("unsupported type: %c", buf[pos])
	}
}

func decodeList(buf []byte, pos int) ([]interface{}, int, error) {
	ret := make([]interface{}, 0)
	i := pos + 1
	for i < len(buf) && buf[i] != 'e' {
		item, offset, err := decodeAny(buf, i)
		if err != nil {
			return nil, 0, err
		}
		ret = append(ret, item)
		i = offset
	}
	return ret, i + 1, nil
}

func decodeMap(buf []byte, pos int) (*orderedmap.OrderedMap, int, error) {
	ret := orderedmap.NewOrderedMap()
	i := pos + 1
	isKey := true
	key := ""
	for i < len(buf) && buf[i] != 'e' {
		item, offset, err := decodeAny(buf, i)
		if err != nil {
			return nil, i + 1, err
		}
		if isKey {
			key = string(item.([]byte))
			isKey = false
		} else {
			ret.Set(key, item)
			isKey = true
		}
		i = offset
	}
	return ret, i + 1, nil
}

func decodeString(buf []byte, pos int) (string, int, error) {
	b, offset, err := decodeBytes(buf, pos)
	return (string)(b), offset, err
}

func decodeBytes(buf []byte, pos int) ([]byte, int, error) {
	i := pos
	for ; i < len(buf) && buf[i] != ':'; i++ {
	}
	l, err := strconv.Atoi((string)(buf[pos:i]))
	if err != nil {
		return nil, 0, err
	}
	return buf[i+1 : i+l+1], i + l + 1, nil
}

func decodeInt(buf []byte, pos int) (int, int, error) {
	begin := pos + 1
	i := begin
	for ; i < len(buf) && buf[i] != 'e'; i++ {
	}
	ret, err := strconv.Atoi((string)(buf[begin:i]))
	return ret, i + 1, err
}
