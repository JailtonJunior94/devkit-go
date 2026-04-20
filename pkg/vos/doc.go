// Package vos provides shared value objects used across domain boundaries.
//
// Value objects are immutable, self-validating types that represent domain
// concepts without identity. This package includes:
//   - UUID: V7 UUID with validation and nil-safety
//   - ULID: Lexicographically sortable unique identifier
//   - Money/Currency/Percentage: financial value objects
//   - Nullable types: NullableTime, NullableBool, NullableFloat, NullableInt, NullableString
//
// Note: For new code, prefer pkg/nullable over the Nullable* types in this package.
// The pkg/nullable types offer a more consistent API with full JSON/SQL support.
package vos
