package orm

import (
	"sort"
	"strings"
)

// Set is a map of field:value. It implements Fields.
type Set map[string]string

// String returns all fields listed as a human readable string.
// Conveniently, exactly the format that ParseSelector takes.
func (ls Set) String() string {
	selector := make([]string, 0, len(ls))
	for key, value := range ls {
		selector = append(selector, key+"="+value)
	}
	// Sort for determinism.
	sort.StringSlice(selector).Sort()
	return strings.Join(selector, ",")
}

// Has returns whether the provided field exists in the map.
func (ls Set) Has(field string) bool {
	_, exists := ls[strings.ToUpper(field)]
	return exists
}

// Get returns the value in the map for the provided field.
func (ls Set) Get(field string) string {
	return ls[strings.ToUpper(field)]
}

func ParseFields(s string) Set {
	set := map[string]string{}
	fs := strings.Split(s, ",")
	for _, f := range fs {
		v := strings.Split(f, "=")
		k := strings.TrimSpace(strings.ToUpper(v[0]))
		if len(v) >= 2 {
			set[k] = strings.Join(v[1:], "=")
		} else if k != "" {
			set[k] = ""
		}
	}

	return set
}
