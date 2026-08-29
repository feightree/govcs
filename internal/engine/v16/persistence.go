package v16

import (
	"encoding/json"
	"fmt"
	"os"
)

func Save(cp *ChargePoint, path string) error {
	b, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal chargepoint: %w", err)
	}

	tmp := fmt.Sprintf("%s.tmp", path)
	err = os.WriteFile(tmp, b, 0644)
	if err != nil {
		return fmt.Errorf("write chargepoint state to %s: %w", path, err)
	}

	err = os.Rename(tmp, path)
	if err != nil {
		return fmt.Errorf("save chargepoint state to %s: %w", path, err)
	}

	return nil
}

func Load(path string) (*ChargePoint, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var cp ChargePoint
	if err := json.Unmarshal(b, &cp); err != nil {
		return nil, fmt.Errorf("unmarshal chargepoint: %w", err)
	}

	return &cp, nil
}
