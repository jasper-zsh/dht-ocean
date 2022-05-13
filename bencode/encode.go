package bencode

import (
	"errors"
	"strconv"
	"strings"
)

func BEncode(obj interface{}) (string, error) {
	builder := strings.Builder{}
	err := encodeAny(&builder, obj)
	if err != nil {
		return "", err
	}
	return builder.String(), nil
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
	case []any:
		err := encodeList(builder, item.([]any))
		if err != nil {
			return err
		}
	default:
		return errors.New("unsupported type")
	}
	return nil
}

func encodeList(builder *strings.Builder, list []any) error {
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
