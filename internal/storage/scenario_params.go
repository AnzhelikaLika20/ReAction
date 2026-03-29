package storage

import (
	"encoding/json"
	"fmt"
)

func ReminderMinutesBeforeFromParams(params []byte) (int32, error) {
	var m map[string]interface{}
	if err := json.Unmarshal(params, &m); err != nil {
		return 0, fmt.Errorf("unmarshal scenario params: %w", err)
	}
	v, ok := m["reminder_minutes_before"]
	if !ok {
		return 0, fmt.Errorf("reminder_minutes_before missing in scenario params")
	}
	switch x := v.(type) {
	case float64:
		return int32(x), nil
	case int32:
		return x, nil
	case int64:
		return int32(x), nil
	case json.Number:
		n, err := x.Int64()
		if err != nil {
			return 0, err
		}
		return int32(n), nil
	default:
		return 0, fmt.Errorf("reminder_minutes_before: unsupported type %T", v)
	}
}
