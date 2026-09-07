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
//
// # Merge semantics on update
//
// Every Update in this package writes these columns with JSONB `||`
// (`name = name || $n::jsonb`), so a request sets the locales it names and
// leaves the rest alone. This is a deliberate choice with a real
// trade-off, so it is worth stating in full:
//
//   - The admin API edits one locale at a time (docs/TZ.md §8.1: "Write
//     bodies edit one locale's text at a time"), while the Update RPCs
//     take a whole aggregate. Replacing the map would mean editing the
//     English name silently deletes every other translation.
//
//   - The alternative — have the caller read, merge and write back the
//     full map — has a lost-update race: two admins editing two different
//     locales at the same time each read the old map, and the second write
//     discards the first's translation. Merging in SQL is a single
//     statement, so that race does not exist.
//
//   - The cost is that there is no way to *delete* a translation through
//     Update. Nothing in R1 needs to (no UI offers it), and adding an
//     explicit "remove these locales" field later is additive. Removing a
//     translation today means a direct write.
//
// Inserts are unaffected: a new row's map is exactly what was supplied.
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
