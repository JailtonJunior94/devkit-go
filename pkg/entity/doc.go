// Package entity provides base types for domain entities.
//
// The Base struct embeds common fields (ID, CreatedAt, UpdatedAt, DeletedAt)
// shared across all domain entities. Embed it in your aggregate roots and
// entities to get consistent identity and audit timestamps.
package entity
