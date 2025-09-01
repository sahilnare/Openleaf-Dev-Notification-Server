package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)


type JSONBArrayString []string

func (a *JSONBArrayString) Scan(value any) error {
	if value == nil {
		*a = []string{} // treat NULL as empty array
		return nil
	}
	switch v := value.(type) {
	case []byte:
		if len(v) == 0 {
			*a = []string{} // treat empty byte slice as empty array
			return nil
		}
		return json.Unmarshal(v, a)
	case string:
		if v == "" {
			*a = []string{}
			return nil
		}
		return json.Unmarshal([]byte(v), a)
	default:
		return fmt.Errorf("JSONBArrayString: unsupported type %T, value: %v", value, value)
	}
}

func (a JSONBArrayString) Value() (driver.Value, error) {
	if a == nil {
		return []byte("[]"), nil
	}
	return json.Marshal(a)
}