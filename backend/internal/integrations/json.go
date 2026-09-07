package integrations

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// jsonUnmarshalStrict decodes strictly from bytes with a clear error.
func jsonUnmarshalStrict(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("invalid json: %w", err)
	}
	return nil
}
