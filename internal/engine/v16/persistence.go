package v16

import (
	"encoding/json"
	"fmt"
	"os"
)

// Save writes cp to path as indented JSON. It writes to a temporary file
// in the same directory first and renames it into place, so a crash or
// interrupt partway through can't leave a truncated or corrupt
// state-file at path - readers only ever see the old file or the
// complete new one, never a partial write.
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

// Load reads a ChargePoint back from the JSON state-file at path, as
// previously written by Save. It returns an error wrapping the
// underlying os error (e.g. os.ErrNotExist) if path can't be read.
func Load(path string) (*ChargePoint, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	var cp ChargePoint
	if err := json.Unmarshal(b, &cp); err != nil {
		return nil, fmt.Errorf("unmarshal chargepoint: %w", err)
	}

	return &cp, nil
}
