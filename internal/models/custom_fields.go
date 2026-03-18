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

// Timetable represents a grid of events organized by time slots and rooms.
type Timetable struct {
	Section   string
	TimeSlots []string                               // row headers (start times)
	Rooms     []string                               // column headers
	Grid      map[string]map[string]*TimetableCell   // [startTime][room]
	// CSS Grid layout data
	StartMinute int    // earliest minute of day (e.g. 09:20 = 560)
	EndMinute   int    // latest minute of day
	TotalRows   int    // total grid rows (1 row per minute)
	PxPerMinute float64 // pixels per minute
}

// TimetableCell holds a single cell in the timetable grid.
type TimetableCell struct {
	Event     database.Event
	Subject   string
	TimeRange string // e.g. "09:20-10:00"
	// CSS Grid positioning
	GridRowStart int
	GridRowEnd   int
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
