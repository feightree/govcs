// Package transport implements the OCPP-J WebSocket transport: encoding
// and decoding of Call/CallResult/CallError frames, and a Transport type
// that manages a WebSocket connection's lifecycle - correlating outbound
// Calls with their responses, replying to inbound Calls, and delivering
// inbound Calls to the caller.
//
// The package is version-agnostic: it only understands the OCPP-J
// envelope shape and has no dependency on any particular OCPP protocol
// version, so it's reusable across OCPP 1.6, 2.0.1, and beyond.
package transport
