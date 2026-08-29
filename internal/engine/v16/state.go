package v16

import (
	"time"

	v16 "github.com/feightree/gocpp/v16"
)

// Connector represents the runtime state of a single physical connector
// on the charge point: its current OCPP status/error code, and the
// transaction it's running, if any.
type Connector struct {
	ID          int32
	Status      v16.ChargePointStatus
	ErrorCode   v16.ChargePointErrorCode
	Transaction *Transaction
}

// Transaction represents a charging transaction in progress on a
// connector. A connector runs at most one Transaction at a time.
type Transaction struct {
	ID          int32
	ConnectorID int32
	IDTag       v16.IDToken
	MeterStart  int32
	StartedAt   time.Time
}

// ChargePoint is the engine's complete representation of the simulated
// charge point: its identity and configuration (as reported to the CSMS
// in BootNotification) alongside its live runtime state (Connectors,
// HeartbeatInterval). A ChargePoint is built fresh by the "new" command,
// or restored wholesale from a saved file by "load".
//
// ChargePoint is only ever touched from the engine's single run-loop
// goroutine - there is no internal locking.
type ChargePoint struct {
	ID                string
	Vendor            string
	Model             string
	CSMS              string
	IMSI              string
	ICCID             string
	HeartbeatInterval int32
	Connectors        []Connector
}

// connectorByID returns the connector with the given ID, or nil if none
// exists.
func (cp *ChargePoint) connectorByID(id int32) *Connector {
	for i := range cp.Connectors {
		if cp.Connectors[i].ID == id {
			return &cp.Connectors[i]
		}
	}

	return nil
}

// transactionByID finds the connector currently running the transaction
// with the given ID, and returns both. A connector can only run one
// transaction at a time, so this also tells the caller which connector
// to update once the transaction ends.
func (cp *ChargePoint) transactionByID(id int32) (*Connector, *Transaction) {
	for i := range cp.Connectors {
		if tx := cp.Connectors[i].Transaction; tx != nil && tx.ID == id {
			return &cp.Connectors[i], tx
		}
	}

	return nil, nil
}
