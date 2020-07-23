package json

import (
	"encoding/json"
)

// Marshal represent function for encode given object into JSON
func Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// Unmarshal represent function for decode given JSON into given object
func Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
