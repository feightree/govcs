package defaultv16

import (
	"time"

	v16 "github.com/feightree/gocpp/v16"
	engine "github.com/feightree/govcs/internal/engine/v16"
)

var _ engine.Profile = (*DefaultV16)(nil)

type DefaultV16 struct{}

func PtrOrNil[T comparable](v T) *T {
	var zero T

	if v == zero {
		return nil
	}

	return &v
}

func (p *DefaultV16) BuildBootNotificationReq(cp *engine.ChargePoint) v16.BootNotificationReq {
	m := v16.BootNotificationReq{
		ChargePointVendor:       cp.Vendor,
		ChargePointModel:        cp.Model,
		ICCID:                   PtrOrNil(cp.ICCID),
		IMSI:                    PtrOrNil(cp.IMSI),
		ChargePointSerialNumber: PtrOrNil(cp.SerialNumber),
		FirmwareVersion:         PtrOrNil(cp.FirmwareVersion),
		MeterType:               PtrOrNil(cp.MeterType),
		MeterSerialNumber:       PtrOrNil(cp.MeterSerialNumber),
	}

	return m
}

func (p *DefaultV16) OnBootNotificationConf(cp *engine.ChargePoint, conf *v16.BootNotificationConf) {
	cp.ClockOffset = time.Until(conf.CurrentTime)
}

func (p *DefaultV16) BuildHeartbeatReq(cp *engine.ChargePoint) v16.HeartbeatReq {
	return v16.HeartbeatReq{}
}

func (p *DefaultV16) OnHeartbeatConf(cp *engine.ChargePoint, conf *v16.HeartbeatConf) {
	cp.ClockOffset = time.Until(conf.CurrentTime)
}
