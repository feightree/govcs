package v16

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"time"

	v16 "github.com/feightree/gocpp/v16"
	"github.com/feightree/govcs/internal/transport"
)

// Profile defines the behavior a charge point profile must implement to
// participate in the engine's OCPP message flow. For each supported
// action it exposes a Build* method (constructs the outbound request
// from the charge point's current state, letting a profile decide what
// to report) and, where relevant, an On*Conf method (an optional,
// best-effort hook for reacting to the response after the engine has
// already applied any bookkeeping its own correctness depends on).
type Profile interface {
	// BuildBootNotificationReq constructs the BootNotificationReq to send
	// for cp. It's called once before the first attempt and the same
	// request is resent unchanged across any Pending/Rejected retries
	// within that handshake - cp's identity doesn't change mid-handshake,
	// so neither should what's reported about it.
	BuildBootNotificationReq(cp *ChargePoint) v16.BootNotificationReq

	// OnBootNotificationConf is called once, after an Accepted
	// BootNotificationConf has already been applied to cp
	// (HeartbeatInterval is set before this runs). It exists purely for a
	// profile to react to a successful registration - e.g. to simulate a
	// charger with quirky post-boot behavior - and is never called for
	// Pending/Rejected responses, since those aren't a final outcome.
	OnBootNotificationConf(cp *ChargePoint, conf *v16.BootNotificationConf)
}

// backoffDelay returns how long to wait before retrying the attempt-th
// failed connection attempt (0-indexed). The delay grows exponentially
// from base, doubling each attempt, capped at max. Jitter is applied by
// picking randomly from the top half of that range, so repeated callers
// (e.g. many simulated charge points reconnecting after the same CSMS
// outage) don't all retry in lockstep.
func backoffDelay(attempt int, base, max time.Duration) time.Duration {
	exp := min(attempt, 30)
	delay := min(base*time.Duration(1<<exp), max)
	return delay/2 + time.Duration(rand.Int64N(int64(delay/2)+1))
}

// connect dials the CSMS at cp.CSMSURL and drives the OCPP boot
// handshake, retrying the dial with exponential backoff if the CSMS is
// unreachable. It returns once boot registration succeeds, handing back
// a ready-to-use Transport - or once ctx is cancelled, or the handshake
// itself fails. A handshake failure is returned as-is, without retrying
// the dial: an unreachable CSMS and a CSMS that rejects the charge
// point's registration are different problems, and only the former is
// connect's job to retry.
func connect(ctx context.Context, cp *ChargePoint, p Profile) (*transport.Transport, error) {
	attempt := 0
	baseDelay := 1 * time.Second
	maxDelay := 60 * time.Second

	for {
		tr, err := transport.Dial(ctx, cp.CSMSURL, transport.Options{
			Subprotocol: "ocpp1.6",
		})

		if err != nil {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoffDelay(attempt, baseDelay, maxDelay)):
				attempt++
				continue
			}
		}

		if err = bootHandshake(ctx, tr, cp, p); err != nil {
			return nil, errors.Join(err, tr.Close())
		}

		return tr, nil
	}
}

// bootHandshake sends a BootNotification over tr and drives the
// spec-defined registration retry loop: a response of anything other
// than Accepted (Pending or Rejected) means the CSMS isn't ready to
// register the charge point yet, and per spec instructs it to wait
// conf.Interval seconds before sending an identical BootNotification
// again. It returns once the CSMS responds Accepted - having recorded
// cp.HeartbeatInterval and notified p via OnBootNotificationConf - or if
// ctx is cancelled or the call/response itself fails.
func bootHandshake(ctx context.Context, tr *transport.Transport, cp *ChargePoint, p Profile) error {
	req := p.BuildBootNotificationReq(cp)

	for {
		raw, err := tr.Call(ctx, "BootNotification", req)
		if err != nil {
			return fmt.Errorf("boot notification: %w", err)
		}

		var conf v16.BootNotificationConf
		if err := json.Unmarshal(raw, &conf); err != nil {
			return fmt.Errorf("boot notification: %w", err)
		}

		if conf.Status == v16.RegistrationStatusAccepted {
			cp.HeartbeatInterval = conf.Interval
			p.OnBootNotificationConf(cp, &conf)
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(conf.Interval) * time.Second):
			continue
		}
	}
}
