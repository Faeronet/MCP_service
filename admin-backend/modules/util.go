package modules

import "encoding/json"

func mustJSON(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}

func mustJSONBytes(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}

func rawJSON(b []byte) interface{} {
	if len(b) == 0 {
		return nil
	}
	var v interface{}
	_ = json.Unmarshal(b, &v)
	return v
}
