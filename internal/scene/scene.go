// Copyright (c) 2026 qyiun666
// SPDX-License-Identifier: MIT OR Apache-2.0

// Package scene holds the small methods over one scene record.

package scene

import (
	"cmp"
	"crypto/rand"
	"encoding/binary"
	"slices"

	"github.com/genosis18m/Long-term-memory-go/internal/common"
	"github.com/genosis18m/Long-term-memory-go/internal/repo"
	"github.com/genosis18m/Long-term-memory-go/internal/repo/core"
)

// CurrentScene names the scene a domain resumes its conversation on: the one a turn was opened in most
// recently.
func CurrentScene(engine *core.StorageEngine, agentID uint64) (uint64, error) {
	scenes, err := repo.CollectAllScenesL2(engine, agentID)
	if err != nil {
		return 0, err
	}
	if len(scenes) == 0 {
		return 0, nil
	}
	resumed := slices.MaxFunc(scenes, func(a, b core.SceneSlot) int {
		if c := cmp.Compare(a.LastUsedAt, b.LastUsedAt); c != 0 {
			return c
		}
		if c := cmp.Compare(a.TurnSeq, b.TurnSeq); c != 0 {
			return c
		}
		return -cmp.Compare(a.SceneID, b.SceneID)
	})
	return resumed.SceneID, nil
}

// ResolveExisting answers which scene a read is scoped to when the host named one, and refuses the
// combination where the host also handed over an anchor: it returns the id, not the record.
func ResolveExisting(engine *core.StorageEngine, agentID uint64, sceneID uint64, anchored bool) (uint64, error) {
	slot, err := core.ReadSceneSlot(engine, agentID, sceneID)
	if err != nil {
		return 0, err
	}
	if anchored {
		return 0, common.NewError(common.ErrInvalidQuery,
			"scene "+common.FormatHash(sceneID)+" already exists; its L3 anchor is set at creation only (use UpdateScene)")
	}
	return slot.SceneID, nil
}

// Create allocates a free scene id and persists the scene under a library-generated name, anchored on
// the given L3 graph when one is named (0 anchors nothing).
func Create(engine *core.StorageEngine, agentID uint64, anchor uint64, stamp int64) (uint64, error) {
	if anchor != 0 {
		g, err := repo.ReadSharedGraphL3(engine, anchor)
		if err != nil {
			return 0, err
		}
		anchor = g.IDHash
	}
	id, err := freshID(engine, agentID)
	if err != nil {
		return 0, err
	}
	slot := core.NewSceneSlot(id, "session:"+common.FormatHash(id))
	slot.LastUsedAt = stamp
	slot.L3ID = anchor
	if err := repo.CreateSceneL2(engine, agentID, &slot); err != nil {
		return 0, err
	}
	return slot.SceneID, nil
}

// freshID mints an unused 8-byte scene id.
func freshID(engine *core.StorageEngine, agentID uint64) (uint64, error) {
	for {
		var b [8]byte
		if _, err := rand.Read(b[:]); err != nil {
			return 0, common.NewError(common.ErrIO, "scene id allocation", err)
		}
		id := binary.LittleEndian.Uint64(b[:])
		if id == 0 {
			continue
		}
		if _, err := core.ReadSceneSlot(engine, agentID, id); err != nil {
			// Any error other than "nothing here" must not mint a scene: a closing
			// database or an IO failure says nothing about whether the id is free.
			if common.CodeOf(err) != common.ErrNotFound {
				return 0, err
			}
			return id, nil
		}
	}
}
