package transport

import (
	"encoding/json"
	"reflect"
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

func TestDecodeFrame(t *testing.T) {
	tt := []struct {
		name      string
		payload   []byte
		want      any
		wantError bool
		errStr    string
	}{
		{
			name:    "Call success",
			payload: []byte(`[2, "1", "foo", "bar"]`),
			want: MessageCall{
				UniqueID: "1",
				Action:   "foo",
				Payload:  json.RawMessage(`"bar"`),
			},
		},
		{
			name:      "Call failure - invalid message type",
			payload:   []byte(`[1, "1", "foo", "bar"]`),
			want:      nil,
			wantError: true,
			errStr:    "invalid message type: 1",
		},
		{
			name:      "Call failure - unmarshalling uniqueID",
			payload:   []byte(`[2, 1, "foo", "bar"]`),
			want:      nil,
			wantError: true,
			errStr:    "failed to unmarshal uniqueID: 1",
		},
		{
			name:      "Call failure - unmarshalling action",
			payload:   []byte(`[2, "1", 1, "bar"]`),
			want:      nil,
			wantError: true,
			errStr:    "failed to unmarshal action: 1",
		},
		{
			name:      "Call failure - malformed",
			payload:   []byte(`[2, "1", "foo"]`),
			want:      nil,
			wantError: true,
			errStr:    "invalid Call message: [2 \"1\" \"foo\"]",
		},
		{
			name:    "CallResult success",
			payload: []byte(`[3, "1", "foo"]`),
			want: MessageCallResult{
				UniqueID: "1",
				Payload:  json.RawMessage(`"foo"`),
			},
		},
		{
			name:      "CallResult failure - unmarshalling uniqueID",
			payload:   []byte(`[3, 1, "foo"]`),
			want:      nil,
			wantError: true,
			errStr:    "failed to unmarshal uniqueID: 1",
		},
		{
			name:      "CallResult failure - malformed",
			payload:   []byte(`[3, "1"]`),
			want:      nil,
			wantError: true,
			errStr:    "invalid CallResult message: [3 \"1\"]",
		},
		{
			name:    "CallError success",
			payload: []byte(`[4, "1", "2", "3", "4"]`),
			want: MessageCallError{
				UniqueID:         "1",
				ErrorCode:        "2",
				ErrorDescription: "3",
				ErrorDetails:     json.RawMessage(`"4"`),
			},
		},
		{
			name:      "CallError failure - unmarshalling uniqueID",
			payload:   []byte(`[4, 1, "2", "3", "4"]`),
			want:      nil,
			wantError: true,
			errStr:    "failed to unmarshal uniqueID: 1",
		},
		{
			name:      "CallError failure - unmarshalling error code",
			payload:   []byte(`[4, "1", 2, "3", "4"]`),
			want:      nil,
			wantError: true,
			errStr:    "failed to unmarshal error code: 2",
		},
		{
			name:      "CallError failure - unmarshalling error description",
			payload:   []byte(`[4, "1", "2", 3, "4"]`),
			want:      nil,
			wantError: true,
			errStr:    "failed to unmarshal error description: 3",
		},
		{
			name:      "failure - malformed message null",
			payload:   []byte(`null`),
			want:      nil,
			wantError: true,
			errStr:    "malformed frame: []",
		},
		{
			name:      "CallError failure - malformed",
			payload:   []byte(`[4, "1"]`),
			want:      nil,
			wantError: true,
			errStr:    "invalid CallError message: [4 \"1\"]",
		},
		{
			name:      "failure - malformed message {}",
			payload:   []byte(`{}`),
			want:      nil,
			wantError: true,
		},
		{
			name:      "failure - unmarshalling message type",
			payload:   []byte(`["2", "1", "foo", "bar"]`),
			want:      nil,
			wantError: true,
			errStr:    "invalid message type: \"2\"",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodeFrame(tc.payload)
			if !tc.wantError && err != nil {
				t.Fatalf("Unexpected Error.\nExpected:\t%v\nGot:\t%v", nil, err)
			}

			if tc.wantError && err == nil {
				t.Fatalf("Wanted an error but got none")
			}

			if tc.errStr != "" && tc.wantError && tc.errStr != err.Error() {
				t.Fatalf("Unexpected Error.\nExpected:\t%v\nGot:\t%v", nil, err.Error())
			}

			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("\nExpected:\t%v\nGot:\t%v", tc.want, got)
			}
		})
	}
}
