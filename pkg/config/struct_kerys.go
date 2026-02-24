package config

import (
	"reflect"
	"strings"
)

const sep = "."

// GetStructKeys returns all keys in a nested struct type, taking the name from the tag name or
// the field name.  It handles an additional suffix squashValue like mapstructure does: if
// present on an embedded struct, name components for that embedded struct should not be
// included.  It does not handle maps, does chase pointers, but does not check for loops in
// nesting.
func GetStructKeys(typ reflect.Type, tag, squashValue string) []string {
	return appendStructKeys(typ, tag, ","+squashValue, nil, nil)
}

// appendStructKeys recursively appends to keys all keys of nested struct type typ, taking tag
// and squashValue from GetStructKeys.  prefix holds all components of the path from the typ
// passed to GetStructKeys down to this typ.
func appendStructKeys(typ reflect.Type, tag, squashValue string, prefix []string, keys []string) []string {
	// Dereference any pointers.  This is a finite loop: Go types are well-founded.
	for ; typ.Kind() == reflect.Ptr; typ = typ.Elem() {
	}

	// Handle only struct containers; terminate the recursion on anything else.
	if typ.Kind() != reflect.Struct {
		return append(keys, strings.Join(prefix, sep))
	}

	for i := 0; i < typ.NumField(); i++ {
		fieldType := typ.Field(i)
		var (
			// fieldName is the name to use for the field.
			fieldName string
			// If squash is true, squash the sub-struct no additional accessor.
			squash bool
			ok     bool
		)
		if fieldName, ok = fieldType.Tag.Lookup(tag); ok {
			if strings.HasSuffix(fieldName, squashValue) {
				squash = true
				fieldName = strings.TrimSuffix(fieldName, squashValue)
			}
		} else {
			fieldName = strings.ToLower(fieldType.Name)
		}
		// Update prefix to recurse into this field.
		if !squash {
			prefix = append(prefix, fieldName)
		}
		keys = appendStructKeys(fieldType.Type, tag, squashValue, prefix, keys)
		// Restore prefix.
		if !squash {
			prefix = prefix[:len(prefix)-1]
		}
	}
	return keys
}

