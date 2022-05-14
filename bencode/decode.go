package bencode

import (
	"fmt"
	"strconv"
)

func BDecode(buf []byte) (interface{}, error) {
	ret, _, err := decodeAny(buf, 0)
	return ret, err
}

func BDecodeDict(buf []byte) (map[string]any, int, error) {
	return decodeDict(buf, 0)
}

func decodeAny(buf []byte, pos int) (interface{}, int, error) {
	if pos >= len(buf) {
		return nil, 0, fmt.Errorf("illegal payload")
	}
	switch buf[pos] {
	case 'i':
		return decodeInt(buf, pos)
	case '1', '2', '3', '4', '5', '6', '7', '8', '9', '0':
		return decodeBytes(buf, pos)
	case 'l':
		return decodeList(buf, pos)
	case 'd':
		return decodeDict(buf, pos)
	default:
		return nil, 0, fmt.Errorf("unsupported type: %c", buf[pos])
	}
}

func decodeList(buf []byte, pos int) ([]any, int, error) {
	ret := make([]any, 0)
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

func decodeDict(buf []byte, pos int) (map[string]any, int, error) {
	ret := make(map[string]any)
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
			ret[key] = item
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
	if l >= len(buf) {
		return nil, 0, fmt.Errorf("illegal str len")
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
