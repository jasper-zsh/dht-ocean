package bencode

import (
	"strings"
)

func GetString(dict map[string]any, key string) (string, bool) {
	r := GetByPath(dict, key)
	if r == nil {
		return "", false
	}
	switch r.(type) {
	case []byte:
		return string(r.([]byte)), true
	default:
		return "", false
	}
}

func GetInt(dict map[string]any, key string) (int, bool) {
	r := GetByPath(dict, key)
	if r == nil {
		return 0, false
	}
	switch r.(type) {
	case int:
		return r.(int), true
	default:
		return 0, false
	}
}

func GetByPath(dict map[string]any, path string) any {
	parts := strings.Split(path, ".")
	var m any = dict
	var ok bool
	for i := 0; i < len(parts); i++ {
		switch m.(type) {
		case map[string]any:
			m, ok = m.(map[string]any)[parts[i]]
			if !ok {
				return nil
			}
		default:
			if i != len(parts)-1 {
				return nil
			}
		}
	}
	return m
}

func MustGetBytes(dict map[string]any, key string) []byte {
	r := GetByPath(dict, key)
	switch r.(type) {
	case []byte:
		return r.([]byte)
	}
	return nil
}

func CheckMapPath(dict map[string]any, path string) bool {
	parts := strings.Split(path, ".")
	var m any = dict
	var ok bool
	for i := 0; i < len(parts); i++ {
		switch m.(type) {
		case map[string]any:
			m, ok = m.(map[string]any)[parts[i]]
			if !ok {
				return false
			}
		default:
			if i != len(parts)-1 {
				return false
			}
		}
	}
	return true
}
