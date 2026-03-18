package models

import "encoding/json"

// KeyValue represents a single custom field entry for events and registrations.
type KeyValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ParseCustomFields parses a JSON string into a slice of KeyValue.
func ParseCustomFields(raw string) []KeyValue {
	if raw == "" || raw == "[]" {
		return nil
	}
	var fields []KeyValue
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		return nil
	}
	return fields
}
