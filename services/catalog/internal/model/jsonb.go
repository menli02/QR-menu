package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// localizedText scans/stores a JSONB column shaped like
// {"en": "Latte", "ru": "Латте"} — the wire shape of the proto
// LocalizedText message (a map<string,string> of BCP-47 locale to
// translation). Used for categories.name, menu_items.name/description,
// modifier_groups.name and modifier_options.name.
type localizedText map[string]string

func (t *localizedText) Scan(src any) error {
	if src == nil {
		*t = nil
		return nil
	}
	var b []byte
	switch v := src.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return fmt.Errorf("model: cannot scan %T into localizedText", src)
	}
	return json.Unmarshal(b, (*map[string]string)(t))
}

// Value implements driver.Valuer so a localizedText can be passed
// directly as a query parameter — the caller never has to remember to
// json.Marshal it first.
func (t localizedText) Value() (driver.Value, error) {
	if t == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(map[string]string(t))
}
