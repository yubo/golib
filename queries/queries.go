package queries

import (
	"fmt"
	"sort"
	"strings"

	"github.com/yubo/golib/util/validation/field"
)

// Queries allows you to present fields independently from their storage.
type Queries interface {
	// Has returns whether the provided field exists.
	Has(field string) (exists bool)

	// Get returns the value for the provided field.
	Get(field string) (value string)
}

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
	_, exists := ls[field]
	return exists
}

// Get returns the value in the map for the provided field.
func (ls Set) Get(field string) string {
	return ls[field]
}

// AsSelector converts fields into a selectors. It does not
// perform any validation, which means the server will reject
// the request if the Set contains invalid values.
func (ls Set) AsSelector() Selector {
	return SelectorFromSet(ls)
}

// AsValidatedSelector converts fields into a selectors.
// The Set is validated client-side, which allows to catch errors early.
func (ls Set) AsValidatedSelector() (Selector, error) {
	return ValidatedSelectorFromSet(ls)
}

// AsSelectorPreValidated converts fields into a selector, but
// assumes that fields are already validated and thus doesn't
// perform any validation.
// According to our measurements this is significantly faster
// in codepaths that matter at high scale.
func (ls Set) AsSelectorPreValidated() Selector {
	return SelectorFromValidatedSet(ls)
}

// FormatFields converts field map into plain string
func FormatFields(fieldMap map[string]string) string {
	l := Set(fieldMap).String()
	if l == "" {
		l = "<none>"
	}
	return l
}

// Conflicts takes 2 maps and returns true if there a key match between
// the maps but the value doesn't match, and returns false in other cases
func Conflicts(fields1, fields2 Set) bool {
	small := fields1
	big := fields2
	if len(fields2) < len(fields1) {
		small = fields2
		big = fields1
	}

	for k, v := range small {
		if val, match := big[k]; match {
			if val != v {
				return true
			}
		}
	}

	return false
}

// Merge combines given maps, and does not check for any conflicts
// between the maps. In case of conflicts, second map (fields2) wins
func Merge(fields1, fields2 Set) Set {
	mergedMap := Set{}

	for k, v := range fields1 {
		mergedMap[k] = v
	}
	for k, v := range fields2 {
		mergedMap[k] = v
	}
	return mergedMap
}

// Equals returns true if the given maps are equal
func Equals(fields1, fields2 Set) bool {
	if len(fields1) != len(fields2) {
		return false
	}

	for k, v := range fields1 {
		value, ok := fields2[k]
		if !ok {
			return false
		}
		if value != v {
			return false
		}
	}
	return true
}

// ConvertSelectorToFieldsMap converts selector string to fields map
// and validates keys and values
func ConvertSelectorToQueriesMap(selector string, opts ...field.PathOption) (Set, error) {
	fieldsMap := Set{}

	if len(selector) == 0 {
		return fieldsMap, nil
	}

	fs := strings.Split(selector, ",")
	for _, f := range fs {
		l := strings.Split(f, "=")
		if len(l) != 2 {
			return fieldsMap, fmt.Errorf("invalid selector: %s", l)
		}
		key := strings.TrimSpace(l[0])
		if err := validateQueryKey(key, field.ToPath(opts...)); err != nil {
			return fieldsMap, err
		}
		value := strings.TrimSpace(l[1])
		if err := validateQueryValue(key, value, field.ToPath(opts...)); err != nil {
			return fieldsMap, err
		}
		fieldsMap[key] = value
	}
	return fieldsMap, nil
}
