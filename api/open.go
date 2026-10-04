// Copyright (c) 2026 qyiun666
// SPDX-License-Identifier: MIT OR Apache-2.0

// Package api is the public facade of MemHop: a Go-module surface over one .meh file, with no business
// logic here.
package api

import (
	"github.com/genosis18m/Long-term-memory-go/internal"
)

// DB is the handle Open returns: one .meh file, its primary domain, and the sub-agent domains created
// under it.
type DB struct {
	db *internal.DB
}

// Open opens the database at path and settles its primary domain.
func Open(path string, llm LlmConfig, defaults MemHopDefaults, profile *ProfileInput) (*DB, error) {
	var primary *internal.ProfileSlot
	if profile != nil {
		slot := toCoreProfileSlot(*profile)
		primary = &slot
	}
	d, err := internal.OpenDB(path, llm, defaults, primary)
	if err != nil {
		return nil, err
	}
	return &DB{db: d}, nil
}

// Primary returns the handle of the domain the file was opened on.
func (d *DB) Primary() (*Session, error) {
	s, err := d.db.Primary()
	if err != nil {
		return nil, err
	}
	return &Session{s}, nil
}

// SubAgent returns the handle of the sub-agent domain named profile.Name, creating that domain the
// first time and handing back the same one every time after.
func (d *DB) SubAgent(llm LlmConfig, profile ProfileInput) (*Session, error) {
	s, err := d.db.SubAgent(llm, toCoreProfileSlot(profile))
	if err != nil {
		return nil, err
	}
	return &Session{s}, nil
}

// Agent returns the handle of an existing domain by Session.AgentID and points it at llm.
func (d *DB) Agent(llm LlmConfig, agentID string) (*Session, error) {
	s, err := d.db.Agent(llm, agentID)
	if err != nil {
		return nil, err
	}
	return &Session{s}, nil
}

// Agents lists every domain the file holds: the primary first, then the registered sub-agents in id
// order.
func (d *DB) Agents() ([]AgentInfo, error) {
	list, err := d.db.Agents()
	if err != nil {
		return nil, err
	}
	return mapSlice(list, fromAgentInfo), nil
}

// Checkpoint persists the per-agent index snapshots without closing.
func (d *DB) Checkpoint() error { return d.db.Checkpoint() }

// DBStats is the file-level view Stats hands back: the size of the .meh file in bytes and the number
// of live records across every domain of the file.
type DBStats struct {
	FileBytes   int64 `json:"file_bytes"`
	RecordCount int64 `json:"record_count"`
}

// Stats reports how big the file has grown and how many live records it holds — the numbers a
// compaction decision is made from.
func (d *DB) Stats() (DBStats, error) {
	size, records, err := d.db.Stats()
	if err != nil {
		return DBStats{}, err
	}
	return DBStats{FileBytes: size, RecordCount: int64(records)}, nil
}

// CompactTo writes a defragmented copy of the whole file at newPath — only live records, in one fresh
// log with its own rebuilt index — and leaves the open file untouched.
func (d *DB) CompactTo(newPath string) error { return d.db.CompactTo(newPath) }

// Close checkpoints every agent domain and releases the file.
func (d *DB) Close() error { return d.db.Close() }

// IsClosed reports whether the database has been closed.
func (d *DB) IsClosed() bool { return d.db.IsClosed() }
