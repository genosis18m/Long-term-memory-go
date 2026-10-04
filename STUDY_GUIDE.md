# MemHop Study Guide — Beginner → Interview Ready

> A progressive guide to understanding this repository: what it is, how it works, where the code lives, edge cases, tradeoffs, and interview talking points.
>
> **Project:** MemHop (this mirror: `Long-term-memory-go`) — an embedded long-term memory database for AI agents. Pure Go, one `.meh` file, no vector DB, no server.
>
> **Version covered:** ~v1.6.6 · Go 1.27+ · Public package: `api/`

---

## How to use this guide

| Level | Goal | Time (rough) |
|-------|------|--------------|
| **L0 — Orientation** | Know what MemHop is and why it exists | 30–45 min |
| **L1 — Go + repo map** | Navigate the codebase as a Go beginner | 1–2 days |
| **L2 — Memory loop** | Explain Search → Append → Update → Dream | 2–3 days |
| **L3 — Six layers** | Explain L0–L5 with file paths | 3–5 days |
| **L4 — Storage engine** | Explain `.meh`, crash recovery, locks | 2–3 days |
| **L5 — Interview depth** | Tradeoffs, edge cases, “how would you improve it?” | ongoing |

Suggested order: read sections top-to-bottom; after each level, write a short summary in your own words without looking.

---

# LEVEL 0 — What is this project?

## One-sentence pitch

**MemHop gives an AI agent a brain-like memory that lives in a single file on disk**, structured into layers (identity, scenes, keywords, knowledge graph, archives, plans), consolidated by a “Dream” pass — without embeddings, without a database server.

## Why it exists (the design stance)

Most agent “memory” today is one of:

1. Dump chat history into the context window (doesn’t scale).
2. Store chunks in a vector DB and retrieve by similarity (good for docs, weak for identity / session structure / forgetting).

MemHop’s claim: memory should be **cognitive** (structured, compressed, consolidated, forgotten) and **embedded** (inside the agent process, one file, zero infra).

## What it is *not*

| Not this | Why |
|----------|-----|
| Vector database | No embeddings, no similarity search over chunks |
| Chat log | Raw text expires; keywords / profile / graph remain |
| Multi-process shared DB | Exclusive file lock — one process owns the file |
| HTTP / MCP server (anymore) | Go module only; host embeds `api` |

## Mental model (brain analogy)

```
L0 Profile     → Who am I? (personality, prefs)
L1 Engram      → Which conversations are related? (scene graph)
L2 Context     → What am I talking about right now? (scene = session)
L3 Knowledge   → What facts/projects do I know? (shared graph)
L4 Archive     → What exactly was said / happened? (expires ~7 days)
L5 Plan        → What am I trying to do this turn? (task tree)
Dream          → “Sleep”: compress, decay, distill, prune
```

## The host’s happy path (copy this into your head)

```go
db, _ := memhop.Open("agent.meh", llm, memhop.DefaultMemHopDefaults, &memhop.ProfileInput{Name: "my-agent", Role: "assistant"})
defer db.Close()
sess, _ := db.Primary()

res, _ := sess.Search(memhop.SearchQuery{})           // open turn + read context keywords
_, _ = sess.AppendArchive(memhop.ArchiveInput{...})    // mid-turn events / extras
topic, _ := sess.Update(memhop.TurnEnd{               // close turn (1 LLM call)
    Input: "...", Output: "...", Outcome: "resolved", CreatedAt: time.Now().UnixMilli(),
})
// Dream often runs in the background when a scene gets too many topics
```

After `Search`, the library **remembers** which scene and turn are open. Later writes do **not** pass scene/topic IDs.

---

# LEVEL 1 — Go beginner toolkit for this repo

You do not need to be a Go expert. You need these ideas:

## Packages and `internal/`

- `package api` — **public**. Hosts import this.
- `package internal` (and subfolders) — **private** to this module. Other modules cannot import `internal/...`. That is intentional encapsulation.
- Dependency rule (strict):  
  `api → internal → repo → core` · `common` sits at the bottom and imports nothing else from this project.

## Types you will see constantly

| Concept | In this project |
|---------|-----------------|
| `struct` | Records like `TopicSlot`, `SceneSlot`, `ArchiveInput` |
| Pointer receivers `func (s *Session)` | Methods that mutate or share state |
| `error` as return value | Almost every API returns `(T, error)` — check `err` |
| `context.Context` | Cancellation for Dream / LLM (Close cancels in-flight work) |
| `sync.Mutex` / `sync.RWMutex` | Domain lock (same agent serial); engine lock |
| `atomic` | Idle timestamps, reclaimed flags |
| Hex IDs at the boundary | Public API uses 16-char hex strings; engine uses `uint64` |

## Facade pattern (why `api/` is thin)

`api/` has almost **no business logic**. It:

1. Accepts host-friendly types (hex IDs, DTOs).
2. Calls `internal.*`.
3. Maps results back (`mapping.go`).

When debugging behavior, jump to `internal/search.go`, `internal/update.go`, etc. — not `api/session.go`.

## How to run / explore

```bash
go build ./...
go vet ./...
go test ./internal/...                    # unit tests, no LLM
go test -tags integration ./test/...      # needs LLM key (see README)
```

Integration tests hit the **public `api` surface only** — good for learning “what a host can do.”

---

# LEVEL 2 — Repository map

```
Long-term-memory-go/
├── api/                    ← PUBLIC: Open, Session, types, errors
├── internal/               ← Composition root + big methods
│   ├── db.go, agents.go    ← Multi-agent file handle
│   ├── session.go          ← Thin bind agentID → DB methods
│   ├── search.go, update.go, dream.go
│   ├── l0.go … l5.go       ← Layer orchestration
│   ├── scene/, turn/, content/, plan/, dream/, graph/
│   ├── domain/             ← Per-agent lock + Scene/Turn + caches
│   ├── cap/                ← engram, llmops, profile, knowledge
│   ├── config/             ← LlmConfig, MemHopDefaults
│   ├── llm/                ← OpenAI-compatible Chat + retries
│   ├── repo/               ← l0–l5 layer writers + agent registry
│   │   ├── index/          ← L2Meta, L4 indexes (in-memory)
│   │   └── core/           ← .meh engine (mmap, frames, lock)
│   └── common/             ← errors, hash, enums, time helpers
├── test/                   ← Integration tests (tag: integration)
├── benches/fixtures/       ← Benchmark datasets
├── README.md               ← Product docs (read first)
└── STUDY_GUIDE.md          ← This file
```

### Who owns what?

| Layer of code | Responsibility | Touches domain lock? |
|---------------|----------------|----------------------|
| `api/` | DTOs, hex mapping | No |
| `internal/*.go` (search/update/dream/l*) | Orchestration | Yes (via domain) |
| `internal/scene|turn|content|plan|dream|graph` | Small pure-ish ops | No (caller holds lock) |
| `internal/cap/*` | Identity-neutral algorithms + LLM prompts | Injected deps |
| `internal/repo/*` | Serialize records to engine | Uses engine |
| `internal/repo/core` | Bytes on disk | Own RWMutex |

---

# LEVEL 3 — The memory loop (master this first)

This is the heart of the product. If you can explain this cold, you already understand 60% of MemHop.

## Diagram

```
┌─────────────┐     opens turn      ┌──────────────────┐
│   Search    │ ──────────────────► │ domain.Scene     │
│ (0 LLM)     │                     │ domain.Turn      │
└─────────────┘                     │ NewTopicID       │
       │                            └──────────────────┘
       │ inject Topics[].FusedKeywords into prompt
       ▼
┌─────────────┐  optional mid-turn  ┌──────────────────┐
│ AppendArchive│ ─────────────────► │ L4 records       │
│ PlanNodeAdd  │                    │ L5 plan nodes    │
└─────────────┘                     └──────────────────┘
       │
       ▼
┌─────────────┐  1 LLM distill      ┌──────────────────┐
│   Update    │ ──────────────────► │ L2 topic row     │
│             │                     │ keywords + Seq1/2│
└─────────────┘                     └──────────────────┘
       │
       │ if depth-1 topics > threshold (default 24)
       ▼
┌─────────────┐                     ┌──────────────────┐
│   Dream     │ ──────────────────► │ compress / prune │
│ (async OK)  │                     │ L1 + L0 distill  │
└─────────────┘                     └──────────────────┘
```

## Search — “read session + open turn”

**File:** `internal/search.go`  
**Cost:** zero LLM, zero embedding — L2Meta cache only.

Behavior:

1. Lock the agent domain.
2. Resolve which **scene** (a scene *is* a host session):
   - empty `SceneID` → continue current / restore furthest-turn scene after reopen
   - `NewScene: true` → create a fresh conversation
   - named `SceneID` → that scene must already exist
3. Bump the scene’s turn counter; mint `NewTopicID` for this turn.
4. Return L0 profile + depth-1 topics (`FusedKeywords` = injectable context).

**Key idea:** the host does not guess which memory to retrieve. The scene *is* the session.

## AppendArchive — mid-turn content

**Files:** `internal/l4.go`, `internal/content/`  
**Cost:** zero LLM.

- Writes into the turn Search opened (no IDs in the call).
- Two kinds, same layer:
  - `KindUtterance` — something said (budget **64 KiB**)
  - `KindEvent` — something happened (budget **4 KiB**)
- `Seq: 0` allocates; naming a Seq rewrites that slot.
- Seq **1 and 2** are reserved for Update’s Input/Output.
- Events may bind to a plan step via `NodeSeq` — that step must already exist.

## Update — close the turn

**File:** `internal/update.go`  
**Cost:** exactly **one** LLM call (keyword distillation).

1. Write Input → Seq 1 (`RoleUser`), Output → Seq 2 (`RoleAgent`).
2. Append Outcome as a `turn_outcome` event (host vocabulary; engine does not branch on it).
3. Distill utterances → `FusedKeywords`.
4. Persist L2 topic. Failed distill → **no topic written** (host content remains for retry).
5. Maybe schedule background Dream when surface grows past threshold.

## Dream — consolidation + retention

**Files:** `internal/dream.go`, `internal/dream/*`  
Always starts with prune; then compress / rebuild / decay / distill (details in Level 5).

---

# LEVEL 4 — The six layers in detail

## L0 — Profile (identity)

| | |
|--|--|
| **Human parallel** | Personality / self-model |
| **Code** | `internal/l0.go`, `internal/cap/profile/` |
| **Storage** | `repo/l0layer.go` → `RecL0Profile` |
| **Host API** | `GetL0`, `UpdateL0` |

Field ownership:

- **Host-owned:** `Name`, `Role`, `Preferences` (Dream does not overwrite these).
- **Host-seeded + Dream-refined:** `Personality`.
- **Dream-written signals:** emotion / MBTI distillation into the profile.
- **Stamped at creation:** `AgentType` (primary vs sub) — cannot be moved by edits.

Edge case: a host `UpdateL0` that omits `Personality` can clear the Dream-refined value until the next Dream.

## L1 — Engram (scene hypergraph)

| | |
|--|--|
| **Human parallel** | “These memories feel related” |
| **Code** | `internal/l1.go`, `internal/cap/engram/` |
| **Host API** | `ListL1()` only — **read-only** |
| **Writer** | Dream only |

- One node per scene; edges when keyword sets overlap (**Jaccard ≥ 0.15**, fixed floor).
- Edges **decay** over time; they strengthen only when an endpoint’s topic evidence actually changed — re-measuring unchanged lists never undoes fade.
- Importance decays with emotional slowdown (strong emotion fades slower).

Interview line: *“L1 is maintained for association queries and future use; Search itself does not walk or score the graph anymore.”*

## L2 — Context (working memory / scenes)

| | |
|--|--|
| **Human parallel** | Working memory for one conversation |
| **Code** | `internal/l2.go`, `internal/scene/`, `internal/search.go` |
| **Host API** | `ListScenes`, `UpdateScene`, `RenameTopic`, `SceneContext`, `MergeScenes`, `DeleteTopic`, `DeleteScene` |

Key concepts:

- **Scene = host session.**
- **Topic** = one turn’s distilled keyword track (and after Dream, fused parents).
- Depth-1 topics on the scene surface are what Search returns as context.
- Dream compression sinks related topics under a fused parent.
- Scene may optionally anchor to an L3 project graph (`L3ID`).

`SceneContext(sceneID)` reads transcript without opening a turn (unlike Search).

## L3 — Knowledge (semantic / project graph)

| | |
|--|--|
| **Human parallel** | Semantic / world knowledge |
| **Code** | `internal/l3.go`, `internal/l3query.go`, `internal/graph/`, `cap/knowledge/` |
| **Host API** | `ImportL3`, `GetL3`, `ListL3`, `UpdateL3`, `DeleteL3`, `QueryL3Nodes`, `QueryL3Subgraph` |

**Critical design choice:** L3 is **file-wide shared**. Every agent domain sees the same project knowledge pool (`SharedPoolAgentID`). Episodic memory (L0–L2, L4, L5) stays isolated per agent.

Hypergraphs: nodes + multi-member edges with kinds (`related`, `causal`, `part_of`, …). Import supports skip/merge/overwrite modes and relation edges resolved by title.

## L4 — Archive (turn content)

| | |
|--|--|
| **Human parallel** | Episodic detail (what exactly happened) |
| **Code** | `internal/l4.go`, `internal/content/` |
| **Retention** | Default **7 days** (`ContentRetentionMs`) — then Dream prunes |

After prune: topic **keywords** remain; `Messages` may be empty or have Seq gaps — that is a **legal** end state, not a broken read.

Dialogue slots:

| Seq | Meaning |
|-----|---------|
| 1 | Turn Input (user) |
| 2 | Turn Output (agent) |
| ≥3 | Host events / extra utterances |

## L5 — Plan (task tree for the open turn)

| | |
|--|--|
| **Human parallel** | Working plan for the current task |
| **Code** | `internal/l5.go`, `internal/plan/` |
| **Host API** | `PlanNodeAdd`, `PlanNodeUpdate`, `PlanState` |

- Tree is keyed by the **turn topic id** Search opened.
- Steps addressed by library-issued **ordinals** (`Seq`), not host paths.
- Statuses: `in_progress` | `done` | `failed` (no “planned but not started”).
- Parent summaries can **fold** children’s summaries when marked folded — host-written summaries are not overwritten.
- Retention prune exempts trees still in flight.

Plans are **per-turn**, not a long-lived cross-session project manager. That is a deliberate simplification (and a common interview discussion point).

---

# LEVEL 5 — Dream pipeline (sleep for agents)

**Orchestration:** `internal/dream.go` → stages in `internal/dream/{prune,compress,structure,report}.go`

### Actual stage order

| Order | Stage | Purpose |
|------:|-------|---------|
| 1 | `l4_prune` | Drop expired content (domain-wide) |
| 2 | `l5_prune` | Drop expired plan nodes (exempt in-flight trees) |
| 3 | `l2_compress` | LLM groups depth-1 topics → fused parents |
| 4 | `index_rebuild` | Refresh L2Meta so depths are correct |
| 5 | `l1_nodes` | Sync scene nodes from L2 |
| 6 | `l1_hyperedges` | Build/refresh co-occurrence edges |
| 7 | `l1_rebuild` | Drop stale L1 structure |
| 8 | `l1_decay` | Fade importance / edge weights |
| 9 | `l0_distill` | Emotion/MBTI/personality from ranked L1 samples |

Triggers:

- Host: `sess.Dream(ctx, sceneID)` — empty sceneID = whole domain.
- Auto: after Update, if depth-1 count > `SceneDreamTopicThreshold` (default 24), background Dream for that scene (deduped via `DreamInFlight`).

Knobs (`internal/config/defaults.go`):

| Knob | Default | Meaning |
|------|---------|---------|
| `SceneDreamTopicThreshold` | 24 | Auto-Dream trigger (`0` = use default, negative = disable) |
| `DreamCompressMinTopics` | 20 | Min topics before compress runs |
| `AgentIdleTTLMs` | 1 hour | Drop idle domain caches from RAM |
| `ContentRetentionMs` | 7 days | L4/L5 sweep window (`0` = default; cannot disable) |

---

# LEVEL 6 — Multi-agent domains

One `.meh` file can hold:

| Domain | How you get it | Isolation |
|--------|----------------|-----------|
| Primary | `db.Primary()` | Own L0–L2, L4, L5 |
| Sub-agent | `db.SubAgent(llm, profile)` keyed by `profile.Name` | Same |
| Shared L3 | Automatic | **Shared** knowledge pool |

Concurrency contract:

- **Same agent:** serialized by `domain.Context.Mu`.
- **Different agents:** parallel on one `*DB`.
- **Processes:** exclusive file lock — second `Open` fails fast.

Idle reclaim: after `AgentIdleTTLMs`, in-memory context (including open Scene/Turn) may be dropped; disk records stay. Next access rebuilds caches — an abandoned open turn does not silently continue.

---

# LEVEL 7 — Storage engine (`.meh`)

**Package:** `internal/repo/core/`  
**Format version:** `0x0012` only — older files rejected, **no migration**.

## File layout

```
[ Header A 4KiB | Header B 4KiB | record frames… | SNAPSHOT ]
         ↑ active = higher CommitID
```

## Why dual headers?

Atomic metadata commit without a full WAL: write inactive header → fsync → flip active. If crash mid-write, the other header still has the previous good CommitID.

## Record frame (26-byte header)

```
type | flags | length | agent_id | id_hash | crc32 | payload…
```

- Identity of a row: `(agent_id, id_hash)`.
- Updates = append new frame with same key (last write wins).
- Deletes = append tombstone (`FlagDeleted`).
- CRC detects torn writes; open truncates / resyncs past corruption.

## mmap + append

- Reads: memory-mapped file (fast random access).
- Writes: append via file descriptor, sync, remap.
- Checkpoint: serialize live index as SNAPSHOT at EOF.
- Compact: copy live records to a new path (offline GC).

## Crash recovery (interview gold)

1. Validate A/B headers; pick max CommitID.
2. Load snapshot index if present; else full scan.
3. Scan frames after snapshot: upsert lives, drop tombstones.
4. Truncate torn tail; clear bad snapshot pointers if needed.

Tests to skim: `internal/repo/core/crash_recovery_test.go`, `crash_recovery_snapshot_test.go`.

---

# LEVEL 8 — Edge cases you must know

These show up in tests and interviews. Memorize the *behavior*, not the file names.

### Turn / session lifecycle

1. **No open turn** → `AppendArchive` / `Update` / plan writes → `ErrInvalidQuery`.
2. **Abandoned round** — Search then Search again without Update: mid-turn L4 may exist under an id that never appears on the scene surface.
3. **Re-Update same turn** — Seq 1/2 overwrite; Outcome appends another event; second distill runs.
4. **Failed LLM on Update** — no topic row; originals stay; turn remains open for retry.
5. **SceneID + NewScene together** — refused as contradictory.
6. **Named missing SceneID** — `ErrNotFound`.
7. **Reopen resume** — empty SceneID restores scene with furthest turn counter; `last_used_at` is forced strictly increasing (same-ms race fix).

### Content & validation

8. Over-budget payloads **refused**, never truncated.
9. Timestamps must be **milliseconds** (seconds/µs scale refused).
10. `RoleDream` is library-only; hosts cannot Append with it.
11. Event `NodeSeq` must name a step this turn created — no auto-growing the plan.
12. Passing a **scene id** as `topic_id` in SearchL4 is refused.

### Multi-agent / file

13. Second process Open → exclusive lock failure.
14. Closed DB → most methods `ErrClosed`; `IsClosed` / `AgentID` still answer.
15. Format ≠ `0x0012` → Open refuses (no migration) — because missing `agent_type` would decode every domain as primary.
16. MergeScenes with conflicting L3 anchors → whole merge refused; otherwise survivor adopts anchor.
17. DeleteScene cascades topics, L4, plans, L1 node.
18. Idle reclaim drops in-memory open turn — loud fail on next write, not silent mis-append.
19. Retention prune → empty/gapped Messages is legal.
20. Shared L3 delete/import affects **all** agents in the file.

---

# LEVEL 9 — Design tradeoffs (say these in interviews)

### 1. Keywords + LLM, not vectors

| Pros | Cons |
|------|------|
| No embedding infra / dims / drift | Quality depends on chat model |
| Cheap read path (cache only) | Distill failures block topic creation |
| Fits “cognitive compression” story | Harder to compare to LoCoMo-style RAG scores |

### 2. Scene-is-session (no retrieval scoring)

| Pros | Cons |
|------|------|
| Deterministic; host always knows the conversation | Host must manage multi-conversation UX |
| Zero LLM on Search | Cannot “find relevant past chat” by similarity |

*(Older versions had RRF + embeddings; removed deliberately.)*

### 3. Append-only log + snapshot

| Pros | Cons |
|------|------|
| Simple durability & crash story | File grows until Compact |
| Natural MVCC / tombstones | Write amplification |

### 4. Strict format versioning, no migrations

| Pros | Cons |
|------|------|
| No half-compatible schemas | Long-lived agents must rebuild files across major bumps |
| Clear Open contract | Ops burden on hosts |

### 5. File-wide shared L3

| Pros | Cons |
|------|------|
| Project knowledge imported once | Isolation broken for knowledge |
| Sub-agents share facts | One bad DeleteL3 hurts everyone |

### 6. Single-writer process lock

| Pros | Cons |
|------|------|
| No distributed concurrency bugs in engine | Cannot multi-process share one `.meh` |
| Embeds cleanly in one agent | Scaling = one file per owner process |

### 7. Dream holds the domain lock

| Pros | Cons |
|------|------|
| Consistent structure updates | Long LLM work blocks that agent’s APIs |
| Simple reasoning | Need async trigger + Close cancel |

### 8. Per-turn plans, not cross-turn trees

| Pros | Cons |
|------|------|
| Clean join key = turn topic id | Long tasks need host-side replay |
| Retention stays simple | Less like a project manager |

### 9. JSON payloads inside frames

| Pros | Cons |
|------|------|
| Flexible evolution inside a format version | Larger / slower than packed binary |

### 10. Public surface convergence (MCP removed)

| Pros | Cons |
|------|------|
| Fewer deps; clearer embed story | Non-Go hosts need their own bindings |

---

# LEVEL 10 — How could it be better?

These are fair “senior engineer” improvement ideas grounded in the current design:

1. **Export/import / migration tooling** — format bumps currently force rebuild; a portable dump would help long-lived agents.
2. **Async Dream observability** — background triggers mostly log; hosts want completion / error callbacks.
3. **L1 edge weight read API** — today hosts see edge IDs, not strengths.
4. **Stronger compress validation** — reject overlapping / invalid LLM merge groups before apply.
5. **Corrupt-record quarantine** — list/drop bad JSON payloads under host approval; Compact already refuses bad live CRC.
6. **Optional cross-turn plans** — opt-in evolving task tree for long jobs.
7. **Expose decay / Jaccard knobs** via the same `0 = default` pattern as other defaults.
8. **Incremental L0 distill** — skip when L1 samples unchanged; currently up to ~200 samples / pass.
9. **Fused-summary size policy** — Dream summaries bypass the host 64 KiB utterance budget.
10. **Restore ADR notes in-tree** — design decisions were removed from `notes/`; git history still has them, but a living ADR folder helps newcomers.
11. **Optional semantic retrieval plug-in** — keep keyword path default; allow hosts that want vector recall without forcing it into the engine.
12. **Compaction scheduling helpers** — `Stats` exists; a recommended “when to CompactTo” policy would help ops.

When proposing improvements in an interview: always state **what invariant you refuse to break** (e.g. “Search stays zero-LLM”, “one process per file”, “retention still bounds disk”).

---

# LEVEL 11 — Go concepts used in the wild here

Study these in Go’s tour / effective Go, then find them in this repo:

| Go topic | Where to see it |
|----------|-----------------|
| Modules / packages | `go.mod`, `api` vs `internal` |
| Interfaces | `llmops.Chat` injected into domain |
| Error wrapping / codes | `internal/common/errors.go`, `api.CodeOf` |
| Mutex vs RWMutex | domain.Context vs StorageEngine |
| atomic | idle / reclaimed flags |
| context cancel | Dream / Close / OpCtx |
| mmap + build tags / OS files | `mmap_unix.go`, `mmap_windows.go`, `filelock_*` |
| Table-driven tests | most `*_test.go` |
| Build tags | `//go:build integration` under `test/` |
| JSON tags / omitempty | public DTOs in `api/types.go` |
| Hex ↔ uint64 boundary | `api/mapping.go` |
| Append-only storage + CRC | `repo/core/frame.go` |
| Fan-out with bounds | Dream compress goroutines |

---

# LEVEL 12 — Suggested reading path (files)

### Day 1 — Product + public API

1. `README.md` — Quick Start + Architecture + API Overview  
2. `api/open.go`  
3. `api/session.go`  
4. `api/types.go`  
5. `api/exports.go` + `api/errors.go`

### Day 2 — The loop

6. `internal/search.go`  
7. `internal/update.go`  
8. `internal/l4.go` + `internal/content/content.go`  
9. `internal/domain/context.go`  
10. `internal/abandoned_round_test.go`

### Day 3 — Layers

11. `internal/l0.go` + `cap/profile/`  
12. `internal/l2.go` + `scene/`  
13. `internal/l1.go` + `cap/engram/`  
14. `internal/l3.go` + `l3query.go`  
15. `internal/l5.go` + `plan/`

### Day 4 — Dream + agents

16. `internal/dream.go` + `dream/compress.go` + `dream/prune.go` + `dream/structure.go`  
17. `internal/agents.go` + `internal/db.go`  
18. `internal/config/defaults.go`

### Day 5 — Storage

19. `repo/core/header.go` → `frame.go` → `engine.go`  
20. `engine_write.go` → `engine_recovery.go` → `snapshot.go` → `reclaim.go`  
21. Crash recovery tests

### Day 6 — Contracts via tests

22. `api/surface_*.go` (closed DB, turn, L1–L4 shapes)  
23. `test/api_interface_*.go` (host-facing integration stories)  
24. `test/e2e_flow_test.go`, `test/fidelity_test.go`

---

# LEVEL 13 — Interview Q&A bank

Practice answering out loud in 60–90 seconds each.

### Basics

**Q: What is MemHop?**  
A: An embedded cognitive memory DB for AI agents: six layers in one `.meh` file, keyword distillation instead of vectors, Dream consolidation, exclusive single-process open.

**Q: Why not a vector DB?**  
A: The product wants structured sessions, identity, compression, and retention — not nearest-neighbor chunk recall. Search is deterministic (scene = session) and zero-LLM.

**Q: Walk me through one turn.**  
A: Search opens scene+turn and returns keyword context → host may AppendArchive / grow a plan → Update writes dialogue slots, one distill, topic persist → maybe async Dream.

### Architecture

**Q: Where does business logic live?**  
A: `internal/`; `api/` is a thin facade for hex DTOs.

**Q: How do agents share a file?**  
A: Isolated domains for episodic layers; L3 knowledge is a reserved shared pool. Same-agent ops serialize; cross-agent parallel; one process lock on the file.

**Q: How durable is a write?**  
A: Append frame with CRC → sync → remap; headers A/B commit metadata; checkpoint snapshots the live index; Compact rewrites live set.

### Harder

**Q: What happens if the process dies mid-Update?**  
A: Dialogue AppendArchives may already be durable under the turn id; if distill never finished, no topic row — host can reopen and retry. Torn frames truncated on next Open.

**Q: Why refuse old format versions?**  
A: Example: `0x0012` added `agent_type`. Without it every domain would decode as primary — a systemic lie, worse than refusing Open.

**Q: Why does Dream prune before compress?**  
A: Retention bounds disk independently of compression quality; expired detail should not influence consolidation samples.

**Q: What’s an abandoned round?**  
A: Search without Update leaves turn content orphaned from the scene surface — intentional pressure for hosts to close turns.

**Q: Name three tradeoffs you’d defend.**  
A: (1) no embeddings, (2) single-writer file, (3) no schema migrations — each buys simplicity/correctness at ops or flexibility cost.

**Q: How would you scale this?**  
A: Horizontal: one `.meh` per agent/tenant process (already the model). Vertical: Compact, retention, idle reclaim. Not: multi-writer shared file without redesigning locking and conflict rules.

---

# LEVEL 14 — Glossary

| Term | Meaning |
|------|---------|
| `.meh` | MemHop database file |
| Scene | One host conversation / session (L2) |
| Topic | Distilled turn (or fused group) with `FusedKeywords` |
| Turn | One Search→Update cycle; library holds Scene+Turn ids |
| Domain | One agent’s isolated memory inside the file |
| Primary | Domain the file was opened on |
| SubAgent | Named secondary domain under the primary |
| Dream | Consolidation + retention pipeline |
| Engram (L1) | Scene nodes + co-occurrence hyperedges |
| Hypergraph | Graph where edges can join >2 nodes |
| L2Meta | In-memory topic metadata cache for Search |
| Tombstone | Delete marker frame in the append log |
| Checkpoint | Persist live index snapshot |
| CompactTo | Write defragmented live-only copy |
| FusedKeywords | Keyword track hosts inject as context |
| SharedPoolAgentID | Reserved agent id for file-wide L3 |

---

# LEVEL 15 — Mini projects to prove you understand it

Do these locally (no need to commit):

1. **Trace a turn** — set breakpoints or add temporary logs in Search → AppendArchive → Update; confirm `domain.Scene` / `domain.Turn` change.
2. **Trigger Dream** — lower `SceneDreamTopicThreshold` in a test helper and watch compress shrink depth-1 topics while L4 still holds facts inside retention.
3. **Crash drill** — kill a test process after writes; reopen; assert live records + A/B recovery (or read existing crash tests until you can explain them).
4. **Two agents** — Primary + SubAgent; import L3 once; show both see the graph; show L2 scenes stay isolated.
5. **Explain a failing test** — pick any `api/surface_*_test.go` failure message and map it to an invariant above.

---

# Cheat sheet (print this)

```
Open(path, llm, defaults, profile) → *DB
  Primary() / SubAgent() → *Session

Session loop:
  Search(q)            → context keywords + opens turn   [0 LLM]
  AppendArchive(in)    → L4 seq                          [0 LLM]
  PlanNodeAdd/Update   → L5 tree                         [0 LLM]
  Update(end)          → topic keywords                  [1 LLM]
  Dream(ctx, scene)    → consolidate + prune             [N LLM]

Layers: L0 identity · L1 scene graph · L2 session · L3 shared knowledge
        L4 expiring content · L5 per-turn plan

Storage: A/B headers · CRC frames · mmap reads · append writes
         snapshot checkpoint · Compact offline · flock exclusive

Invariants:
  - Search never scores / never embeds
  - Writes after Search name no scene/topic ids
  - Failed distill writes no topic
  - Over-budget refused, not truncated
  - Format 0x0012 only
  - One process per file
```

---

# Attribution

This guide describes the MemHop architecture as implemented in this repository (public mirror of [qyiun666/MemHop](https://github.com/qyiun666/MemHop)). For product changelog detail, prefer `README.md`. For behavioral truth, prefer tests under `api/` and `test/`.

---

*Last structured for learners studying the v1.6.x Go module surface. If the public API or format version moves, re-check `README.md`, `api/exports.go`, and `internal/repo/core` version constants first.*
