package transport

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/coder/websocket"
)

var ErrTransportClosed = errors.New("transport closed")

type Options struct {
	Subprotocol string
}

type Transport struct {
	// The websocket connection
	conn *websocket.Conn
	// A channel for inbound messages (ie. MessageCalls)
	in chan MessageCall
	// Map for pending results by UniqueID
	pending  map[string]chan callResult
	mu       sync.Mutex
	once     sync.Once
	closed   chan struct{}
	closeErr error
}

func (tr *Transport) readLoop(ctx context.Context) {
	for {
		_, data, err := tr.conn.Read(ctx)
		if err != nil {
			if err := tr.Close(); err != nil {
				slog.ErrorContext(ctx, "error while closing transport",
					slog.Any("error", err),
				)
			}
			return
		}

		decoded, err := decodeFrame(data)
		if err != nil {
			// a malformed frame from the peer - what should happen here?
			slog.ErrorContext(ctx, "failed decoding frame",
				slog.Any("error", err),
			)
			continue
		}

		switch msg := decoded.(type) {
		case MessageCall:
			select {
			case tr.in <- msg:
				// Delivered - someone was ready to receive it
			case <-ctx.Done():
				// ctx was cancelled/expired before anyone received it
				continue
			}
		case MessageCallResult:
			// find the matching pending entry, deliver success
			tr.mu.Lock()
			p, ok := tr.pending[msg.UniqueID]
			if ok {
				delete(tr.pending, msg.UniqueID)
			}
			tr.mu.Unlock()
			if !ok {
				slog.WarnContext(ctx, "received a CallResult for an unknown message",
					slog.String("UniqueID", msg.UniqueID),
				)
				continue
			}
			p <- callResult{payload: msg.Payload}

		case MessageCallError:
			// find the matching pending entry, deliver as an error
			tr.mu.Lock()
			p, ok := tr.pending[msg.UniqueID]
			if ok {
				delete(tr.pending, msg.UniqueID)
			}
			tr.mu.Unlock()
			if !ok {
				slog.WarnContext(ctx, "received a CallError for an unknown message",
					slog.String("UniqueID", msg.UniqueID),
				)
				continue
			}
			p <- callResult{err: msg}
		}
	}
}

func (tr *Transport) Call(ctx context.Context, action string, payload any) (json.RawMessage, error) {
	id, err := newMessageID()
	if err != nil {
		return nil, err
	}

	frame, err := encodeCall(id, action, payload)
	if err != nil {
		return nil, err
	}

	ch := make(chan callResult, 1)
	// register ch in tr.pending under id
	tr.mu.Lock()
	tr.pending[id] = ch
	tr.mu.Unlock()

	// write frame to the connection
	if err := tr.conn.Write(ctx, websocket.MessageText, frame); err != nil {
		tr.mu.Lock()
		delete(tr.pending, id)
		tr.mu.Unlock()
		return nil, err
	}

	// wait for result, respecting ctx and t.closed
	select {
	case result := <-ch:
		return result.payload, result.err
	case <-ctx.Done():
		tr.mu.Lock()
		delete(tr.pending, id)
		tr.mu.Unlock()
		return nil, fmt.Errorf("call %s (%s): %w", action, id, ctx.Err())
	case <-tr.closed:
		tr.mu.Lock()
		delete(tr.pending, id)
		tr.mu.Unlock()
		return nil, fmt.Errorf("call %s (%s): %w", action, id, ErrTransportClosed)
	}
}

func (tr *Transport) Inbound() <-chan MessageCall {
	return tr.in
}

func (tr *Transport) Respond(ctx context.Context, uniqueID string, payload any) error {
	frame, err := encodeCallResult(uniqueID, payload)
	if err != nil {
		return err
	}

	return tr.conn.Write(ctx, websocket.MessageText, frame)
}

func (tr *Transport) RespondError(ctx context.Context, uniqueID, code, description string, details any) error {
	frame, err := encodeCallError(uniqueID, code, description, details)
	if err != nil {
		return err
	}

	return tr.conn.Write(ctx, websocket.MessageText, frame)
}

func (tr *Transport) Close() error {
	tr.once.Do(func() {
		err := tr.conn.Close(websocket.StatusGoingAway, "closing")
		tr.closeErr = err
		close(tr.closed)
	})

	return tr.closeErr
}

type callResult struct {
	payload json.RawMessage
	err     error
}

func Dial(ctx context.Context, url string, opts Options) (*Transport, error) {
	conn, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{
		Subprotocols: []string{opts.Subprotocol},
	})

	if err != nil {
		return nil, err
	}

	t := &Transport{
		conn:    conn,
		in:      make(chan MessageCall),
		pending: make(map[string]chan callResult),
		closed:  make(chan struct{}),
	}

	go t.readLoop(ctx)
	return t, nil
}

// newMessageID creates a random 16 char string to be used for message IDs.
func newMessageID() (string, error) {
	var b = make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return hex.EncodeToString(b), nil
}
