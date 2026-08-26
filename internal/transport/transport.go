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

// ErrTransportClosed is returned by Call when the Transport is closed -
// or the connection is lost - while the call is in flight.
var ErrTransportClosed = errors.New("transport closed")

// Options configures a Dial.
type Options struct {
	// Subprotocol is the WebSocket subprotocol to negotiate during the
	// handshake, e.g. "ocpp1.6".
	Subprotocol string
}

// Transport manages a single WebSocket connection's lifecycle: sending
// outbound Calls and correlating them with their CallResult/CallError
// response, replying to inbound Calls, and delivering inbound Calls to
// the caller via Inbound. A Transport is created by Dial and should
// eventually be released with Close.
//
// All exported methods are safe for concurrent use.
type Transport struct {
	// conn is the underlying WebSocket connection.
	conn *websocket.Conn
	// in delivers inbound Call frames to the caller; see Inbound.
	in chan MessageCall
	// pending correlates an outbound Call's UniqueID with the channel
	// waiting for its response. Only accessed while holding mu.
	pending map[string]chan callResult
	mu      sync.Mutex
	// once guards the actual close, so it happens exactly once no
	// matter whether Close is called directly or triggered by readLoop
	// after a read error.
	once sync.Once
	// closed is closed exactly once, by Close, when the Transport shuts
	// down. Any Call currently waiting for a response observes this via
	// its own select and returns ErrTransportClosed - nothing else
	// needs to react to it or touch pending on close.
	closed chan struct{}
	// closeErr is the result of the one real close attempt, cached so
	// every caller of Close - not just the one that ran it - sees it.
	closeErr error
}

// readLoop reads and dispatches frames for the lifetime of the
// connection. It is started once, by Dial, and runs until conn.Read
// returns an error - meaning the connection was closed or lost - at
// which point it shuts the Transport down via Close.
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
			// There's no caller waiting on this specific frame, so a
			// malformed frame is logged and skipped rather than
			// tearing down the whole connection over one bad frame.
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

// Call sends an OCPP Call for action with the given payload, and blocks
// until a matching CallResult or CallError arrives, ctx is done, or the
// Transport is closed - whichever happens first.
//
// On success, it returns the raw CallResult payload. If the peer replies
// with a CallError, the returned error is a MessageCallError (use
// errors.As to inspect it). If ctx is done first, the returned error
// wraps ctx.Err(). If the Transport is closed while waiting, the
// returned error wraps ErrTransportClosed.
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

// Inbound returns the channel on which inbound Call frames from the peer
// are delivered. Callers should read from it continuously - readLoop
// blocks trying to deliver a frame until it's received or the ctx passed
// to Dial is done.
func (tr *Transport) Inbound() <-chan MessageCall {
	return tr.in
}

// Respond sends a CallResult for the inbound Call identified by
// uniqueID, with the given payload.
func (tr *Transport) Respond(ctx context.Context, uniqueID string, payload any) error {
	frame, err := encodeCallResult(uniqueID, payload)
	if err != nil {
		return err
	}

	return tr.conn.Write(ctx, websocket.MessageText, frame)
}

// RespondError sends a CallError for the inbound Call identified by
// uniqueID.
func (tr *Transport) RespondError(ctx context.Context, uniqueID, code, description string, details any) error {
	frame, err := encodeCallError(uniqueID, code, description, details)
	if err != nil {
		return err
	}

	return tr.conn.Write(ctx, websocket.MessageText, frame)
}

// Close closes the underlying connection and shuts the Transport down.
// It is idempotent and safe to call concurrently or multiple times -
// every caller sees the result of the one real close attempt, regardless
// of whether Close was called directly or triggered by readLoop after a
// read error. Any Call currently waiting for a response returns
// ErrTransportClosed.
func (tr *Transport) Close() error {
	tr.once.Do(func() {
		err := tr.conn.Close(websocket.StatusGoingAway, "closing")
		tr.closeErr = err
		close(tr.closed)
	})

	return tr.closeErr
}

// callResult is the outcome of a single outbound Call, delivered from
// readLoop to the goroutine blocked in Call.
type callResult struct {
	payload json.RawMessage
	err     error
}

// Dial establishes a WebSocket connection to url and returns a Transport
// ready to send and receive OCPP-J messages over it. On success, a
// background goroutine is started to read inbound frames for the
// lifetime of the connection; callers should eventually call Close to
// release it.
//
// If the connection cannot be established, Dial returns a nil Transport
// and a non-nil error.
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
