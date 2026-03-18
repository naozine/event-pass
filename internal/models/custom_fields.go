package models

import (
	"encoding/json"

	"github.com/naozine/project_crud_with_auth_tmpl/internal/database"
)

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

// EventGroup holds a section label and its events for grouped display.
type EventGroup struct {
	Section string
	Events  []database.Event
}

// GetCustomFieldValue returns the value for a given key from custom fields JSON.
func GetCustomFieldValue(raw string, key string) string {
	for _, f := range ParseCustomFields(raw) {
		if f.Key == key {
			return f.Value
		}
	}
	return ""
}
