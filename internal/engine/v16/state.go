package v16

import (
	"fmt"
	"net/url"
	"time"

	ocpp "github.com/feightree/gocpp/ocpp"
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
	CSMSURL           string
	HeartbeatInterval int32
	SerialNumber      v16.CiString25Type
	Vendor            v16.CiString20Type
	Model             v16.CiString20Type
	IMSI              v16.CiString20Type
	ICCID             v16.CiString20Type
	FirmwareVersion   v16.CiString50Type
	MeterSerialNumber v16.CiString25Type
	MeterType         v16.CiString25Type
	Connectors        []Connector
}

func (cp *ChargePoint) Validate() error {
	if cp.ID == "" {
		return ocpp.NewError(ocpp.ErrOccurenceConstraintViolation, "id", "required field is missing")
	}

	if cp.CSMSURL == "" {
		return ocpp.NewError(ocpp.ErrOccurenceConstraintViolation, "csms", "required field is missing")
	}

	u, err := url.ParseRequestURI(cp.CSMSURL)
	if err != nil {
		return ocpp.NewError(ocpp.ErrFormationViolation, "csms", "is an invalid URL")
	}

	if u.Scheme != "ws" && u.Scheme != "wss" {
		return ocpp.NewError(
			ocpp.ErrFormationViolation,
			"csms",
			fmt.Sprintf("'%s' is an invalid websocket scheme - must be 'ws' or 'wss'", u.Scheme),
		)
	}

	if u.Host == "" {
		return ocpp.NewError(
			ocpp.ErrFormationViolation,
			"csms",
			fmt.Sprintf("'%s' is an invalid host", u.Host),
		)
	}

	if cp.HeartbeatInterval == 0 {
		return ocpp.NewError(ocpp.ErrPropertyConstraintViolation, "heartbeat", "must be > 0")
	}

	if err := cp.SerialNumber.Validate(); err != nil {
		return ocpp.WrapField("SerialNumber", err)
	}

	if err := cp.Vendor.Validate(); err != nil {
		return ocpp.WrapField("Vendor", err)
	}

	if err := cp.Model.Validate(); err != nil {
		return ocpp.WrapField("Model", err)
	}

	if err := cp.IMSI.Validate(); err != nil {
		return ocpp.WrapField("IMSI", err)
	}

	if err := cp.ICCID.Validate(); err != nil {
		return ocpp.WrapField("ICCID", err)
	}

	if err := cp.FirmwareVersion.Validate(); err != nil {
		return ocpp.WrapField("FirmwareVersion", err)
	}

	if err := cp.MeterSerialNumber.Validate(); err != nil {
		return ocpp.WrapField("MeterSerialNumber", err)
	}

	if err := cp.MeterType.Validate(); err != nil {
		return ocpp.WrapField("MeterType", err)
	}

	return nil
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
