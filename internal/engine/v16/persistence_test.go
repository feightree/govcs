package v16

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	v16 "github.com/feightree/gocpp/v16"
)

func TestPersistence(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		path := fmt.Sprintf("%s/cp001", tmpDir)

		cp := ChargePoint{
			ID:     "Test",
			Vendor: "Vendor",
			Connectors: []Connector{
				{
					ID:        1,
					Status:    v16.ChargePointStatusCharging,
					ErrorCode: v16.ChargePointErrorCodeNoError,
					Transaction: &Transaction{
						ID:          1,
						ConnectorID: 1,
						IDTag:       v16.CiString20Type("idtoken"),
						MeterStart:  100,
						StartedAt:   time.Date(2026, time.January, 15, 10, 32, 21, 37, time.UTC),
					},
				},
				{
					ID:        2,
					Status:    v16.ChargePointStatusAvailable,
					ErrorCode: v16.ChargePointErrorCodeNoError,
				},
			},
		}

		if err := Save(&cp, path); err != nil {
			t.Fatalf("Unexpected Error.\nExpected:\t%v\nGot:\t%v", nil, err)
		}

		loaded, err := Load(path)
		if err != nil {
			t.Fatalf("Unexpected Error.\nExpected:\t%v\nGot:\t%v", nil, err)
		}

		if !reflect.DeepEqual(*loaded, cp) {
			t.Errorf("loaded chargepoint doesn't match saved\nExpected:\t%+v\nGot:\t%+v", cp, *loaded)
		}
	})

	t.Run("load missing file", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		path := fmt.Sprintf("%s/cp001", tmpDir)

		_, err := Load(path)

		if err == nil {
			t.Fatalf("Expected Error.\nExpected:\t%v\nGot:\t%v", os.ErrNotExist, nil)
		}

		if !errors.Is(err, os.ErrNotExist) {
			t.Errorf("Unexpected Error.\nExpected:\t%v\nGot:\t%v", os.ErrNotExist, err)
		}
	})

}
