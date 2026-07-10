// Package model defines the canonical domain model for AERS.
//
// This package contains the core data structures that represent the
// fundamental concepts of the Adaptive Environment Recovery System.
// Every entity in this package is a plain data structure — a direct
// representation of the problem domain with no implementation coupling.
//
// # Design Principles
//
// Domain types defined here are intentionally free of behavior.
// Business logic, validation rules, and workflow orchestration belong
// in higher-level packages that consume these types.
//
// Serialization concerns (JSON, YAML, Protocol Buffers, etc.) are
// handled at the boundary layers that translate between the domain
// model and external representations. Types in this package carry
// no serialization annotations.
//
// Entities should remain implementation-independent. They must not
// import infrastructure packages, reference specific storage backends,
// or depend on any external library. This ensures the domain model
// can evolve without being constrained by implementation choices.
package model
