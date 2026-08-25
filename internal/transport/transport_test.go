package transport

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func NewTestCSMS(t *testing.T) (string, <-chan *websocket.Conn) {
	t.Helper()

	ch := make(chan *websocket.Conn, 1)
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("websocket accept: %v", err)
			return
		}

		// Send connection on channel
		ch <- conn
	}))

	t.Cleanup(svr.Close)

	url := strings.Replace(svr.URL, "http://", "ws://", 1)
	return url, ch
}

func TestNewMessageID(t *testing.T) {
	t.Run("success - new message id", func(t *testing.T) {
		str1, err := newMessageID()

		if err != nil {
			t.Fatalf("Unexpected Error.\nExpected:\t%v\nGot:\t%v", nil, err)
		}

		if len(str1) != 16 {
			t.Errorf("\nExpected:\t%v\nGot:\t%v", 16, len(str1))
		}

		str2, err := newMessageID()

		if err != nil {
			t.Fatalf("Unexpected Error.\nExpected:\t%v\nGot:\t%v", nil, err)
		}

		if str1 == str2 {
			t.Errorf("\nExpected:\t%v\nGot:\t%v", false, true)
		}
	})

	t.Run("success - decode", func(t *testing.T) {
		str, err := newMessageID()

		if err != nil {
			t.Fatalf("Unexpected Error.\nExpected:\t%v\nGot:\t%v", nil, err)
		}

		b, err := hex.DecodeString(str)

		if err != nil {
			t.Fatalf("Unexpected Error.\nExpected:\t%v\nGot:\t%v", nil, err)
		}

		if len(b) != 8 {
			t.Errorf("\nExpected:\t%v\nGot:\t%v", 8, len(b))
		}
	})
}

// 1. Happy path — peer reads the Call frame, echoes back [3,id,payload]; assert Call returns that payload with no error.
// 2. CallError path — peer responds [4,id,code,description,details]; assert errors.As into MessageCallError recovers the code/description.
// 3. ctx cancellation — peer intentionally never responds; cancel (or timeout) the ctx passed to Call; assert errors.Is(err, context.DeadlineExceeded) (or Canceled) still works through your wrapping.
// 4. Transport closed mid-call — peer closes the conn without responding; assert the error wraps ErrTransportClosed.
// 5. Out-of-order correlation — two concurrent Calls, peer responds to the second UniqueID first; assert each caller gets its own matching payload back. This is the one that actually exercises the pending-map logic, not just the happy path.
// 6. Bonus: peer sends a CallResult for an unregistered ID (a stray/duplicate) before the real one — proves the readLoop's warn-and-continue doesn't corrupt state for the still-pending call.

func TestCall(t *testing.T) {
	type callOutcome struct {
		payload json.RawMessage
		err     error
	}

	// Happy Path
	t.Run("call success", func(t *testing.T) {
		// Set up CSMS test server
		url, connCh := NewTestCSMS(t)

		// Dial WS
		tr, err := Dial(t.Context(), url, Options{})
		if err != nil {
			t.Fatalf("\nUnexpected Error:\t%v", err)
		}

		resultCh := make(chan callOutcome, 1)
		go func() {
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()

			payload, err := tr.Call(ctx, "SomeAction", "SomePayload")
			resultCh <- callOutcome{payload, err}
		}()

		go func() {
			conn := <-connCh
			_, b, err := conn.Read(t.Context())
			if err != nil {
				t.Errorf("\nWebsocket read error:\t%v", err)
				return
			}

			raw, err := decodeFrame(b)
			if err != nil {
				t.Errorf("\nDecode error:\t%v", err)
				return
			}

			msg, ok := raw.(MessageCall)
			if !ok {
				t.Errorf("\nExpected message call")
				return
			}

			frame, err := encodeCallResult(msg.UniqueID, "success")
			if err != nil {
				t.Errorf("\nEncodeCallResult error:\t%v", err)
				return
			}

			if err := conn.Write(t.Context(), websocket.MessageText, frame); err != nil {
				t.Errorf("\nWebsocket write error:\t%v", err)
			}
		}()

		outcome := <-resultCh
		if outcome.err != nil {
			t.Fatalf("\nUnexpected error:\t%v", outcome.err)
		}

		var got string
		if err := json.Unmarshal(outcome.payload, &got); err != nil {
			t.Fatalf("\nFailed to unmarshal payload:\t%v", err)
		}

		if got != "success" {
			t.Errorf("\nExpected:\t%v\nGot:\t\t%v", "success", got)
		}
	})

	// CallError path
	t.Run("call error", func(t *testing.T) {
		// Set up CSMS test server
		url, connCh := NewTestCSMS(t)

		// Dial WS
		tr, err := Dial(t.Context(), url, Options{})
		if err != nil {
			t.Fatalf("\nUnexpected Error:\t%v", err)
		}

		resultCh := make(chan callOutcome, 1)
		go func() {
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()

			payload, err := tr.Call(ctx, "SomeAction", "SomePayload")
			resultCh <- callOutcome{payload, err}
		}()

		go func() {
			conn := <-connCh
			_, b, err := conn.Read(t.Context())
			if err != nil {
				t.Errorf("\nWebsocket read error:\t%v", err)
				return
			}

			raw, err := decodeFrame(b)
			if err != nil {
				t.Errorf("\nDecode error:\t%v", err)
				return
			}

			msg, ok := raw.(MessageCall)
			if !ok {
				t.Errorf("\nExpected message call")
				return
			}

			frame, err := encodeCallError(msg.UniqueID, "error code", "error description", "error!")
			if err != nil {
				t.Errorf("\nEncodeCallError error:\t%v", err)
				return
			}

			if err := conn.Write(t.Context(), websocket.MessageText, frame); err != nil {
				t.Errorf("\nWebsocket write error:\t%v", err)
			}
		}()

		outcome := <-resultCh

		var msgErr MessageCallError
		if !errors.As(outcome.err, &msgErr) {
			t.Fatalf("\nExpected outcome to be of type MessageCallError")
		}

		if msgErr.ErrorCode != "error code" {
			t.Errorf("\nExpected:\t%v\nGot:\t\t%v", "error code", msgErr.ErrorCode)
		}

		if msgErr.ErrorDescription != "error description" {
			t.Errorf("\nExpected:\t%v\nGot:\t\t%v", "error description", msgErr.ErrorDescription)
		}

		var details string
		if err := json.Unmarshal(msgErr.ErrorDetails, &details); err != nil {
			t.Fatalf("\nFailed to unmarshal payload:\t%v", err)
		}

		if details != "error!" {
			t.Errorf("\nExpected:\t%v\nGot:\t\t%v", "error!", details)
		}
	})

	// CTX Cancellation path
	t.Run("ctx timeout", func(t *testing.T) {
		// Set up CSMS test server
		url, _ := NewTestCSMS(t)

		// Dial WS
		tr, err := Dial(t.Context(), url, Options{})
		if err != nil {
			t.Fatalf("\nUnexpected Error:\t%v", err)
		}

		ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
		defer cancel()

		_, err = tr.Call(ctx, "SomeAction", "SomePayload")
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("\nExpected:\t%v\nGot:\t\t%v", context.DeadlineExceeded, err)
		}
	})

	// Transport Closed mid call path
	t.Run("tr closed", func(t *testing.T) {
		// Set up CSMS test server
		url, connCh := NewTestCSMS(t)

		// Dial WS
		tr, err := Dial(t.Context(), url, Options{})
		if err != nil {
			t.Fatalf("\nUnexpected Error:\t%v", err)
		}

		resultCh := make(chan callOutcome, 1)
		go func() {
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()

			payload, err := tr.Call(ctx, "SomeAction", "SomePayload")
			resultCh <- callOutcome{payload, err}
		}()

		go func() {
			conn := <-connCh
			_, _, err := conn.Read(t.Context())

			if err != nil {
				t.Errorf("\nWebsocket read error:\t%v", err)
				return
			}

			if err := conn.Close(websocket.StatusGoingAway, "bye"); err != nil {
				t.Errorf("\nFailed closing connection:\t%v", err)
			}
		}()

		outcome := <-resultCh

		if !errors.Is(outcome.err, ErrTransportClosed) {
			t.Errorf("\nExpected:\t%v\nGot:\t\t%v", ErrTransportClosed, outcome.err)
		}
	})

	// Out of order path
	t.Run("out of order success", func(t *testing.T) {
		// Set up CSMS test server
		url, connCh := NewTestCSMS(t)

		// Dial WS
		tr, err := Dial(t.Context(), url, Options{})
		if err != nil {
			t.Fatalf("\nUnexpected Error:\t%v", err)
		}

		resultCh1 := make(chan callOutcome, 1)
		resultCh2 := make(chan callOutcome, 1)

		go func() {
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()

			payload, err := tr.Call(ctx, "SomeAction1", "SomePayload1")
			resultCh1 <- callOutcome{payload, err}
		}()

		go func() {
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()

			payload, err := tr.Call(ctx, "SomeAction2", "SomePayload2")
			resultCh2 <- callOutcome{payload, err}
		}()

		// Set up two readers for the two calls, but reply to them in reverse
		// order
		go func() {
			conn := <-connCh

			_, in1, err := conn.Read(t.Context())
			if err != nil {
				t.Errorf("\nWebsocket read error:\t%v", err)
				return
			}

			raw1, err := decodeFrame(in1)
			if err != nil {
				t.Errorf("\nDecode error:\t%v", err)
				return
			}

			msg1, ok := raw1.(MessageCall)
			if !ok {
				t.Errorf("\nExpected message1 to be MessageCall")
				return
			}

			_, in2, err := conn.Read(t.Context())
			if err != nil {
				t.Errorf("\nWebsocket read error:\t%v", err)
				return
			}

			raw2, err := decodeFrame(in2)
			if err != nil {
				t.Errorf("\nDecode error:\t%v", err)
				return
			}

			msg2, ok := raw2.(MessageCall)
			if !ok {
				t.Errorf("\nExpected message2 to be MessageCall")
				return
			}

			out1, err := encodeCallResult(msg1.UniqueID, msg1.Action)
			if err != nil {
				t.Errorf("\nEncodeCallResult1 error:\t%v", err)
				return
			}

			out2, err := encodeCallResult(msg2.UniqueID, msg2.Action)
			if err != nil {
				t.Errorf("\nEncodeCallResult2 error:\t%v", err)
				return
			}

			if err := conn.Write(t.Context(), websocket.MessageText, out2); err != nil {
				t.Errorf("\nWebsocket write error for out2:\t%v", err)
			}

			if err := conn.Write(t.Context(), websocket.MessageText, out1); err != nil {
				t.Errorf("\nWebsocket write error for out1:\t%v", err)
			}
		}()

		outcome1 := <-resultCh1
		outcome2 := <-resultCh2

		if outcome1.err != nil {
			t.Fatalf("\nUnexpected error:\t%v", outcome1.err)
		}

		if outcome2.err != nil {
			t.Fatalf("\nUnexpected error:\t%v", outcome2.err)
		}

		var got string
		if err := json.Unmarshal(outcome1.payload, &got); err != nil {
			t.Fatalf("\nFailed to unmarshal payload:\t%v", err)
		}

		if got != "SomeAction1" {
			t.Errorf("\nExpected:\t%v\nGot:\t\t%v", "SomeAction1", got)
		}

		if err := json.Unmarshal(outcome2.payload, &got); err != nil {
			t.Fatalf("\nFailed to unmarshal payload:\t%v", err)
		}

		if got != "SomeAction2" {
			t.Errorf("\nExpected:\t%v\nGot:\t\t%v", "SomeAction2", got)
		}
	})

	t.Run("stray call success", func(t *testing.T) {
		// Set up CSMS test server
		url, connCh := NewTestCSMS(t)

		// Dial WS
		tr, err := Dial(t.Context(), url, Options{})
		if err != nil {
			t.Fatalf("\nUnexpected Error:\t%v", err)
		}

		resultCh := make(chan callOutcome, 1)
		go func() {
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()

			payload, err := tr.Call(ctx, "SomeAction", "SomePayload")
			resultCh <- callOutcome{payload, err}
		}()

		go func() {
			conn := <-connCh
			_, b, err := conn.Read(t.Context())
			if err != nil {
				t.Errorf("\nWebsocket read error:\t%v", err)
				return
			}

			raw, err := decodeFrame(b)
			if err != nil {
				t.Errorf("\nDecode error:\t%v", err)
				return
			}

			msg, ok := raw.(MessageCall)
			if !ok {
				t.Errorf("\nExpected message call")
				return
			}

			frame, err := encodeCallResult(msg.UniqueID, "success")
			if err != nil {
				t.Errorf("\nEncodeCallResult error:\t%v", err)
				return
			}

			bogus, err := encodeCallResult("bogusID", "success")
			if err != nil {
				t.Errorf("\nEncodeCallResult error:\t%v", err)
				return
			}

			if err := conn.Write(t.Context(), websocket.MessageText, bogus); err != nil {
				t.Errorf("\nWebsocket write error:\t%v", err)
			}

			if err := conn.Write(t.Context(), websocket.MessageText, frame); err != nil {
				t.Errorf("\nWebsocket write error:\t%v", err)
			}
		}()

		outcome := <-resultCh
		if outcome.err != nil {
			t.Fatalf("\nUnexpected error:\t%v", outcome.err)
		}

		var got string
		if err := json.Unmarshal(outcome.payload, &got); err != nil {
			t.Fatalf("\nFailed to unmarshal payload:\t%v", err)
		}

		if got != "success" {
			t.Errorf("\nExpected:\t%v\nGot:\t\t%v", "success", got)
		}
	})
}

func TestInbound(t *testing.T) {
	t.Run("receives inbound", func(t *testing.T) {
		// Set up CSMS test server
		url, connCh := NewTestCSMS(t)

		// Dial WS
		tr, err := Dial(t.Context(), url, Options{})
		if err != nil {
			t.Fatalf("\nUnexpected Error:\t%v", err)
		}

		uniqueID, err := newMessageID()
		if err != nil {
			t.Errorf("\nFailed to create a new message ID:\t%v", err)
			return
		}

		go func() {
			conn := <-connCh

			frame, err := encodeCall(uniqueID, "Action", "Payload")
			if err != nil {
				t.Errorf("\nEncodeCallResult error:\t%v", err)
				return
			}

			if err := conn.Write(t.Context(), websocket.MessageText, frame); err != nil {
				t.Errorf("\nWebsocket write error:\t%v", err)
			}
		}()

		select {
		case inbound := <-tr.Inbound():
			if inbound.UniqueID != uniqueID {
				t.Errorf("\nExpected:\t%v\nGot:\t\t%v", uniqueID, inbound.UniqueID)
			}

			if inbound.Action != "Action" {
				t.Errorf("\nExpected:\t%v\nGot:\t\t%v", "Action", inbound.Action)
			}

			var payload string
			if err := json.Unmarshal(inbound.Payload, &payload); err != nil {
				t.Fatalf("\nFailed to unmarshal payload:\t%v", err)
			}

			if payload != "Payload" {
				t.Errorf("\nExpected:\t%v\nGot:\t\t%v", "Payload", payload)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for inbound message")
		}
	})
}

func TestResponse(t *testing.T) {
	url, connCh := NewTestCSMS(t)
	tr, err := Dial(t.Context(), url, Options{})
	if err != nil {
		t.Fatalf("\nUnexpected Dial Error:\t%v", err)
	}

	conn := <-connCh

	if err := tr.Respond(t.Context(), "UniqueID", "Payload"); err != nil {
		t.Fatalf("\nUnexpected Respond Error:\t%v", err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	_, b, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("\nWebsocket read error:\t%v", err)
	}

	raw, err := decodeFrame(b)
	if err != nil {
		t.Fatalf("\nDecode error:\t%v", err)
	}

	msg, ok := raw.(MessageCallResult)
	if !ok {
		t.Errorf("\nExpected MessageCallResult")
		return
	}

	if msg.UniqueID != "UniqueID" {
		t.Errorf("\nExpected:\t%v\nGot:\t%v", "UniqueID", msg.UniqueID)
	}

	var payload string
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		t.Fatalf("\nFailed to unmarshal payload:\t%v", err)
	}

	if payload != "Payload" {
		t.Errorf("\nExpected:\t%v\nGot:\t\t%v", "Payload", payload)
	}
}

func TestResponseError(t *testing.T) {
	url, connCh := NewTestCSMS(t)
	tr, err := Dial(t.Context(), url, Options{})
	if err != nil {
		t.Fatalf("\nUnexpected Dial Error:\t%v", err)
	}

	conn := <-connCh

	if err := tr.RespondError(t.Context(), "UniqueID", "ErrorCode", "ErrorDescription", "ErrorDetails"); err != nil {
		t.Fatalf("\nUnexpected RespondError Error:\t%v", err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	_, b, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("\nWebsocket read error:\t%v", err)
	}

	raw, err := decodeFrame(b)
	if err != nil {
		t.Fatalf("\nDecode error:\t%v", err)
	}

	msg, ok := raw.(MessageCallError)
	if !ok {
		t.Errorf("\nExpected MessageCallError")
		return
	}

	if msg.UniqueID != "UniqueID" {
		t.Errorf("\nExpected:\t%v\nGot:\t%v", "UniqueID", msg.UniqueID)
	}

	if msg.ErrorCode != "ErrorCode" {
		t.Errorf("\nExpected:\t%v\nGot:\t%v", "ErrorCode", msg.ErrorCode)
	}

	if msg.ErrorDescription != "ErrorDescription" {
		t.Errorf("\nExpected:\t%v\nGot:\t%v", "ErrorDescription", msg.ErrorDescription)
	}

	var details string
	if err := json.Unmarshal(msg.ErrorDetails, &details); err != nil {
		t.Fatalf("\nFailed to unmarshal payload:\t%v", err)
	}

	if details != "ErrorDetails" {
		t.Errorf("\nExpected:\t%v\nGot:\t\t%v", "ErrorDetails", details)
	}
}
