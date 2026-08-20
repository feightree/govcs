package transport

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestEncodeCall(t *testing.T) {
	tt := []struct {
		name     string
		uniqueID string
		action   string
		payload  any
		want     json.RawMessage
	}{
		{
			name:     "success with values",
			uniqueID: "1",
			action:   "T",
			payload:  json.RawMessage(`{"ok":1}`),
			want:     []byte(`[2,"1","T",{"ok":1}]`),
		},
		{
			name: "success with zero values",
			want: []byte(`[2,"","",null]`),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := encodeCall(tc.uniqueID, tc.action, tc.payload)

			if !slices.Equal(got, tc.want) {
				t.Errorf("\nExpected:\t%s\nGot:\t%s", tc.want, got)
			}
		})
	}
}

func TestEncodeCallResult(t *testing.T) {
	tt := []struct {
		name     string
		uniqueID string
		payload  any
		want     json.RawMessage
	}{
		{
			name:     "success with values",
			uniqueID: "1",
			payload:  json.RawMessage(`{"ok":1}`),
			want:     []byte(`[3,"1",{"ok":1}]`),
		},
		{
			name: "success with zero values",
			want: []byte(`[3,"",null]`),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := encodeCallResult(tc.uniqueID, tc.payload)

			if !slices.Equal(got, tc.want) {
				t.Errorf("\nExpected:\t%s\nGot:\t%s", tc.want, got)
			}
		})
	}
}

func TestEncodeCallError(t *testing.T) {
	tt := []struct {
		name        string
		uniqueID    string
		code        string
		description string
		details     any
		want        json.RawMessage
	}{
		{
			name:        "success with values",
			uniqueID:    "1",
			code:        "Code",
			description: "Desc",
			details:     json.RawMessage(`{"ok":1}`),
			want:        []byte(`[4,"1","Code","Desc",{"ok":1}]`),
		},
		{
			name: "success with zero values",
			want: []byte(`[4,"","","",{}]`),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := encodeCallError(tc.uniqueID, tc.code, tc.description, tc.details)

			if !slices.Equal(got, tc.want) {
				t.Errorf("\nExpected:\t%s\nGot:\t%s", tc.want, got)
			}
		})
	}
}
