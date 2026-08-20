package transport

import (
	"encoding/json"
	"fmt"
)

// MessageType indicates which type of message the message frame contains
type MessageType int

const (
	// MessageTypeCall indicates a Call message
	//
	// [2, "<uniqueId>", "<action>", {payload}]
	MessageTypeCall MessageType = 2
	// MessageTypeCallResult indicates a CallResult message
	//
	// [3, "<uniqueId>", {payload}]
	MessageTypeCallResult MessageType = 3
	// MessageTypeCallError indicates a CallError message
	//
	// [4, "<uniqueId>", "<errorCode>", "<errorDescription>", {errorDetails}]
	MessageTypeCallError MessageType = 4
)

// MessageCall is the decoded Call frame used to perform a request, in
// either direction.
type MessageCall struct {
	UniqueID string
	Action   string
	Payload  json.RawMessage
}

// MessageCallResult is the decoded CallResult frame, used to indicate a
// successful response to a Call frame.
type MessageCallResult struct {
	UniqueID string
	Payload  json.RawMessage
}

// MessageCallError is the decoded CallError frame, used to indicate an
// error response to a Call frame.
type MessageCallError struct {
	UniqueID         string
	ErrorCode        string
	ErrorDescription string
	ErrorDetails     json.RawMessage
}

func (ce MessageCallError) Error() string {
	return fmt.Sprintf("callerror %s: %s", ce.ErrorCode, ce.ErrorDescription)
}

func encodeCall(uniqueID, action string, payload any) ([]byte, error) {
	return json.Marshal([]any{MessageTypeCall, uniqueID, action, payload})
}

func encodeCallResult(uniqueID string, payload any) ([]byte, error) {
	return json.Marshal([]any{MessageTypeCallResult, uniqueID, payload})
}

func encodeCallError(uniqueID, code, description string, details any) ([]byte, error) {
	if details == nil {
		details = map[string]any{}
	}

	return json.Marshal([]any{MessageTypeCallError, uniqueID, code, description, details})
}

func decodeFrame(data []byte) (any, error) {
	var raw []json.RawMessage

	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	if len(raw) == 0 || raw == nil {
		return nil, fmt.Errorf("malformed frame: %v", raw)
	}

	var messageType MessageType
	if err := json.Unmarshal(raw[0], &messageType); err != nil {
		return nil, fmt.Errorf("invalid message type: %s", string(raw[0]))
	}

	switch messageType {
	case MessageTypeCall:
		if len(raw) != 4 {
			return nil, fmt.Errorf("invalid Call message: %+v", raw)
		}

		var uniqueID string
		if err := json.Unmarshal(raw[1], &uniqueID); err != nil {
			return nil, fmt.Errorf("failed to unmarshal uniqueID: %s", string(raw[1]))
		}

		var action string
		if err := json.Unmarshal(raw[2], &action); err != nil {
			return nil, fmt.Errorf("failed to unmarshal action: %s", string(raw[2]))
		}

		return MessageCall{
			UniqueID: uniqueID,
			Action:   action,
			Payload:  raw[3],
		}, nil
	case MessageTypeCallResult:
		if len(raw) != 3 {
			return nil, fmt.Errorf("invalid CallResult message: %+v", raw)
		}

		var uniqueID string
		if err := json.Unmarshal(raw[1], &uniqueID); err != nil {
			return nil, fmt.Errorf("failed to unmarshal uniqueID: %s", string(raw[1]))
		}

		return MessageCallResult{
			UniqueID: uniqueID,
			Payload:  raw[2],
		}, nil
	case MessageTypeCallError:
		if len(raw) != 5 {
			return nil, fmt.Errorf("invalid CallError message: %+v", raw)
		}

		var uniqueID string
		if err := json.Unmarshal(raw[1], &uniqueID); err != nil {
			return nil, fmt.Errorf("failed to unmarshal uniqueID: %s", string(raw[1]))
		}

		var errorCode string
		if err := json.Unmarshal(raw[2], &errorCode); err != nil {
			return nil, fmt.Errorf("failed to unmarshal error code: %s", string(raw[2]))
		}

		var errorDescription string
		if err := json.Unmarshal(raw[3], &errorDescription); err != nil {
			return nil, fmt.Errorf("failed to unmarshal error description: %s", string(raw[3]))
		}

		return MessageCallError{
			UniqueID:         uniqueID,
			ErrorCode:        errorCode,
			ErrorDescription: errorDescription,
			ErrorDetails:     raw[4],
		}, nil
	}

	return nil, fmt.Errorf("invalid message type: %d", messageType)
}
