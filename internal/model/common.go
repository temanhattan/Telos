package model

import "time"

// UUID represents a universally unique identifier as a string.
//
// The concrete format (v4, v7, etc.) is determined by the layer
// that generates identifiers. The model layer treats it as an
// opaque, immutable value.
type UUID string

// Version represents a semantic version string following the
// Major.Minor.Patch convention (e.g. "1.0.0", "2.1.3").
//
// Parsing and comparison logic belongs in higher-level packages.
type Version string

// Timestamp is an alias for time.Time, preserving all standard
// library methods and avoiding unnecessary conversions.
type Timestamp = time.Time
