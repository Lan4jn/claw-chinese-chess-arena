# Xiangqi Repetition Rules Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add engine-level long-check, long-chase, and idle-repetition constraints that reject violating moves, filter them out of legal move lists, and surface stable error messages through existing match logs.

**Architecture:** Keep repetition ownership inside the engine. Extend `GameState` with deterministic position keys and per-move rule traces, classify simulated move effects before commit, and reuse the same rule filter in both `LegalMoves` and `Apply` so human and `picoclaw` share identical move availability and validation semantics.

**Tech Stack:** Go 1.20, existing engine/match model, `testing`, `go test`

---

## File Structure

### Files to modify

- `engine.go`
  - Add position-key helpers, repetition trace types, move-effect classification, repetition rule evaluation, legal-move filtering, and `Apply` integration.
- `match.go`
  - Preserve engine repetition errors verbatim in existing illegal-move log paths.
- `arena_test.go`
  - Add match-level coverage for human and agent repetition error propagation.

### Files to create

- `engine_test.go`
  - Add deterministic engine-level repetition rule tests and helper builders for crafted board states.

## Task 1: Add red tests for repetition-aware legal move filtering

**Files:**
- Create: `engine_test.go`
- Modify: `engine.go`

- [ ] **Step 1: Write the failing engine tests for legal move filtering**

```go
func TestGameStateLegalMovesExcludeForbiddenIdleRepetition(t *testing.T) {
	g := GameState{
		Board: boardFromRows([]string{
			"....k....",
			".........",
			".........",
			".........",
			".........",
			".........",
			".........",
			".........",
			"....R....",
			"....K....",
		}),
		Side:   SideRed,
		Status: "playing",
		RuleTraces: []RuleTrace{
			{Side: SideRed, Move: "e8-f8", PositionKey: "idle-a", RepeatedPosition: false},
			{Side: SideBlack, Move: "e0-f0", PositionKey: "idle-b", RepeatedPosition: false},
			{Side: SideRed, Move: "f8-e8", PositionKey: "idle-a", RepeatedPosition: true},
			{Side: SideBlack, Move: "f0-e0", PositionKey: "idle-b", RepeatedPosition: true},
		},
	}

	moves := g.LegalMoveStrings()
	if slicesContains(moves, "e8-f8") {
		t.Fatalf("expected idle repetition move e8-f8 to be filtered out, got %v", moves)
	}
}

func TestGameStateLegalMovesExcludeForbiddenLongCheck(t *testing.T) {
	g := GameState{
		Board: boardFromRows([]string{
			"....k....",
			".........",
			"....R....",
			".........",
			".........",
			".........",
			".........",
			".........",
			".........",
			"....K....",
		}),
		Side:   SideRed,
		Status: "playing",
		RuleTraces: []RuleTrace{
			{Side: SideRed, Move: "e2-e1", PositionKey: "check-a", GivesCheck: true},
			{Side: SideBlack, Move: "e0-f0", PositionKey: "check-b"},
			{Side: SideRed, Move: "e1-e2", PositionKey: "check-a", GivesCheck: true, RepeatedPosition: true},
			{Side: SideBlack, Move: "f0-e0", PositionKey: "check-b", RepeatedPosition: true},
		},
	}

	moves := g.LegalMoveStrings()
	if slicesContains(moves, "e2-e1") {
		t.Fatalf("expected long-check move e2-e1 to be filtered out, got %v", moves)
	}
}
```

- [ ] **Step 2: Run the new engine filtering tests to verify red state**

Run: `GOTOOLCHAIN=local go test ./... -run 'TestGameStateLegalMovesExcludeForbiddenIdleRepetition|TestGameStateLegalMovesExcludeForbiddenLongCheck' -v`

Expected: FAIL with unknown `RuleTrace`, unknown `RuleTraces`, or missing repetition filtering in `LegalMoveStrings`

- [ ] **Step 3: Add minimal engine scaffolding for position keys and rule traces**

Add to `engine.go` near `GameState`:

```go
type RuleTrace struct {
	Side             Side     `json:"side"`
	Move             string   `json:"move"`
	PositionKey      string   `json:"position_key"`
	GivesCheck       bool     `json:"gives_check"`
	IsCapture        bool     `json:"is_capture"`
	ChaseTargets     []string `json:"chase_targets,omitempty"`
	RepeatedPosition bool     `json:"repeated_position"`
}

type GameState struct {
	Board      Board        `json:"board"`
	Side       Side         `json:"side"`
	Winner     Side         `json:"winner,omitempty"`
	Status     string       `json:"status"`
	Reason     string       `json:"reason,omitempty"`
	MoveCount  int          `json:"move_count"`
	LastMove   string       `json:"last_move,omitempty"`
	History    []MoveRecord `json:"history"`
	RuleTraces []RuleTrace  `json:"rule_traces,omitempty"`
}

func (g GameState) PositionKey() string {
	rows := BoardRows(g.Board)
	return string(g.Side) + "|" + strings.Join(rows, "/")
}
```

Also add test helpers at the bottom of `engine_test.go`:

```go
func boardFromRows(rows []string) Board {
	var b Board
	for y, row := range rows {
		for x := range row {
			if row[x] != '.' {
				b[y][x] = Piece(row[x])
			}
		}
	}
	return b
}

func slicesContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Filter legal move generation through a new repetition gate**

Update `GameState.LegalMoves` in `engine.go`:

```go
func (g GameState) LegalMoves(side Side) []Move {
	pseudo := g.pseudoMoves(side)
	legal := make([]Move, 0, len(pseudo))
	for _, mv := range pseudo {
		next := g
		next.applyUnchecked(mv)
		if next.king(side) == nil {
			continue
		}
		if next.kingsFacing() {
			continue
		}
		if next.inCheck(side) {
			continue
		}
		if reason := g.repetitionViolationForMove(mv); reason != "" {
			continue
		}
		legal = append(legal, mv)
	}
	return legal
}
```

Add a temporary stub in `engine.go`:

```go
func (g GameState) repetitionViolationForMove(mv Move) string {
	_ = mv
	return ""
}
```

- [ ] **Step 5: Run the filtering tests again to verify they still fail for the right reason**

Run: `GOTOOLCHAIN=local go test ./... -run 'TestGameStateLegalMovesExcludeForbiddenIdleRepetition|TestGameStateLegalMovesExcludeForbiddenLongCheck' -v`

Expected: FAIL because the stubbed repetition gate returns no violation and the forbidden moves are still present

- [ ] **Step 6: Commit the scaffolding**

```bash
git add engine.go engine_test.go
git commit -m "test: scaffold xiangqi repetition rule engine"
```

## Task 2: Implement position-repeat tracing and idle repetition detection

**Files:**
- Modify: `engine.go`
- Modify: `engine_test.go`

- [ ] **Step 1: Write the failing idle repetition application tests**

Append to `engine_test.go`:

```go
func TestGameStateApplyRejectsForbiddenIdleRepetition(t *testing.T) {
	g := GameState{
		Board: boardFromRows([]string{
			"....k....",
			".........",
			".........",
			".........",
			".........",
			".........",
			".........",
			".........",
			"....R....",
			"....K....",
		}),
		Side:   SideRed,
		Status: "playing",
		RuleTraces: []RuleTrace{
			{Side: SideRed, Move: "e8-f8", PositionKey: "red-loop-a"},
			{Side: SideBlack, Move: "e0-f0", PositionKey: "black-loop-b"},
			{Side: SideRed, Move: "f8-e8", PositionKey: "red-loop-a", RepeatedPosition: true},
			{Side: SideBlack, Move: "f0-e0", PositionKey: "black-loop-b", RepeatedPosition: true},
		},
	}

	err := g.Apply("e8-f8")
	if err == nil || err.Error() != "move causes forbidden idle repetition" {
		t.Fatalf("expected forbidden idle repetition, got %v", err)
	}
}

func TestGameStateApplyAllowsCaptureThatBreaksLoop(t *testing.T) {
	g := GameState{
		Board: boardFromRows([]string{
			"....k....",
			".........",
			".........",
			".........",
			".........",
			".........",
			".........",
			"....r....",
			"....R....",
			"....K....",
		}),
		Side:   SideRed,
		Status: "playing",
		RuleTraces: []RuleTrace{
			{Side: SideRed, Move: "e8-f8", PositionKey: "loop-a"},
			{Side: SideBlack, Move: "e7-f7", PositionKey: "loop-b"},
			{Side: SideRed, Move: "f8-e8", PositionKey: "loop-a", RepeatedPosition: true},
			{Side: SideBlack, Move: "f7-e7", PositionKey: "loop-b", RepeatedPosition: true},
		},
	}

	if err := g.Apply("e8-e7"); err != nil {
		t.Fatalf("expected capture to break repetition, got %v", err)
	}
}
```

- [ ] **Step 2: Run the idle repetition tests to verify red state**

Run: `GOTOOLCHAIN=local go test ./... -run 'TestGameStateApplyRejectsForbiddenIdleRepetition|TestGameStateApplyAllowsCaptureThatBreaksLoop' -v`

Expected: FAIL because `Apply` does not yet classify repetition or preserve capture-as-progress semantics

- [ ] **Step 3: Add move-effect classification helpers**

Add to `engine.go`:

```go
type moveEffects struct {
	PositionKey      string
	GivesCheck       bool
	IsCapture        bool
	ChaseTargets     []string
	RepeatedPosition bool
}

func (g GameState) classifyMoveEffects(mv Move) moveEffects {
	next := g
	captured := next.Board[mv.ToY][mv.ToX]
	next.applyUnchecked(mv)
	effects := moveEffects{
		PositionKey: next.PositionKey(),
		GivesCheck:  next.inCheck(next.Side),
		IsCapture:   captured != 0,
	}
	for _, trace := range g.RuleTraces {
		if trace.PositionKey == effects.PositionKey {
			effects.RepeatedPosition = true
			break
		}
	}
	effects.ChaseTargets = next.chaseTargetsForSide(opposite(next.Side))
	return effects
}

func (g GameState) buildRuleTrace(side Side, move string, effects moveEffects) RuleTrace {
	return RuleTrace{
		Side:             side,
		Move:             move,
		PositionKey:      effects.PositionKey,
		GivesCheck:       effects.GivesCheck,
		IsCapture:        effects.IsCapture,
		ChaseTargets:     append([]string(nil), effects.ChaseTargets...),
		RepeatedPosition: effects.RepeatedPosition,
	}
}
```

- [ ] **Step 4: Add idle repetition rule evaluation**

Add to `engine.go`:

```go
func (g GameState) repetitionViolationForMove(mv Move) string {
	effects := g.classifyMoveEffects(mv)
	if !effects.RepeatedPosition {
		return ""
	}
	if effects.IsCapture {
		return ""
	}
	if effects.GivesCheck {
		return ""
	}
	if g.isLongChaseRepetition(effects) {
		return "move causes forbidden long-chase repetition"
	}
	if g.isIdleRepetition(effects) {
		return "move causes forbidden idle repetition"
	}
	return ""
}

func (g GameState) isIdleRepetition(effects moveEffects) bool {
	if effects.IsCapture || effects.GivesCheck || len(effects.ChaseTargets) > 0 {
		return false
	}
	matches := 0
	for i := len(g.RuleTraces) - 1; i >= 0; i-- {
		trace := g.RuleTraces[i]
		if trace.PositionKey != effects.PositionKey {
			continue
		}
		if trace.IsCapture || trace.GivesCheck || len(trace.ChaseTargets) > 0 {
			return false
		}
		matches++
		if matches >= 2 {
			return true
		}
	}
	return false
}
```

- [ ] **Step 5: Wire `Apply` to reject idle repetition before commit**

Update `GameState.Apply` in `engine.go`:

```go
func (g *GameState) Apply(moveText string) error {
	if g.Status != "playing" {
		return fmt.Errorf("game is not playing")
	}
	mv, err := ParseMove(moveText)
	if err != nil {
		return err
	}
	ok := false
	for _, legal := range g.LegalMoves(g.Side) {
		if legal == mv {
			ok = true
			break
		}
	}
	if !ok {
		if reason := g.repetitionViolationForMove(mv); reason != "" {
			return fmt.Errorf(reason)
		}
		return fmt.Errorf("%s is not a legal move for %s", moveText, g.Side)
	}
	piece := g.Board[mv.FromY][mv.FromX]
	captured := g.Board[mv.ToY][mv.ToX]
	effects := g.classifyMoveEffects(mv)
	movingSide := g.Side
	g.applyUnchecked(mv)
	g.LastMove = mv.String()
	g.MoveCount++
	g.History = append(g.History, MoveRecord{
		Side:    movingSide,
		Move:    mv.String(),
		Piece:   string(piece),
		Capture: pieceString(captured),
	})
	g.RuleTraces = append(g.RuleTraces, g.buildRuleTrace(movingSide, mv.String(), effects))
```

Leave the existing terminal-state checks below this block unchanged except for using `movingSide` instead of `opposite(g.Side)` when needed.

- [ ] **Step 6: Run the idle repetition tests to verify green state**

Run: `GOTOOLCHAIN=local go test ./... -run 'TestGameStateApplyRejectsForbiddenIdleRepetition|TestGameStateApplyAllowsCaptureThatBreaksLoop' -v`

Expected: PASS

- [ ] **Step 7: Commit the idle repetition implementation**

```bash
git add engine.go engine_test.go
git commit -m "feat: block idle repetition in xiangqi engine"
```

## Task 3: Implement long-check repetition detection

**Files:**
- Modify: `engine.go`
- Modify: `engine_test.go`

- [ ] **Step 1: Write the failing long-check application test**

Append to `engine_test.go`:

```go
func TestGameStateApplyRejectsForbiddenLongCheckRepetition(t *testing.T) {
	g := GameState{
		Board: boardFromRows([]string{
			"....k....",
			".........",
			"....R....",
			".........",
			".........",
			".........",
			".........",
			".........",
			".........",
			"....K....",
		}),
		Side:   SideRed,
		Status: "playing",
		RuleTraces: []RuleTrace{
			{Side: SideRed, Move: "e2-e1", PositionKey: "check-a", GivesCheck: true},
			{Side: SideBlack, Move: "e0-f0", PositionKey: "check-b"},
			{Side: SideRed, Move: "e1-e2", PositionKey: "check-a", GivesCheck: true, RepeatedPosition: true},
			{Side: SideBlack, Move: "f0-e0", PositionKey: "check-b", RepeatedPosition: true},
		},
	}

	err := g.Apply("e2-e1")
	if err == nil || err.Error() != "move causes forbidden long-check repetition" {
		t.Fatalf("expected forbidden long-check repetition, got %v", err)
	}
}
```

- [ ] **Step 2: Run the long-check test to verify red state**

Run: `GOTOOLCHAIN=local go test ./... -run TestGameStateApplyRejectsForbiddenLongCheckRepetition -v`

Expected: FAIL because check repetition is not yet distinguished from generic legality

- [ ] **Step 3: Add long-check repetition classification**

Add to `engine.go`:

```go
func (g GameState) isLongCheckRepetition(side Side, effects moveEffects) bool {
	if !effects.GivesCheck || !effects.RepeatedPosition || effects.IsCapture {
		return false
	}
	matches := 0
	for i := len(g.RuleTraces) - 1; i >= 0; i-- {
		trace := g.RuleTraces[i]
		if trace.Side != side {
			continue
		}
		if trace.PositionKey != effects.PositionKey {
			continue
		}
		if !trace.GivesCheck || trace.IsCapture {
			return false
		}
		matches++
		if matches >= 2 {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Route the repetition gate through long-check first**

Update `repetitionViolationForMove` in `engine.go`:

```go
func (g GameState) repetitionViolationForMove(mv Move) string {
	effects := g.classifyMoveEffects(mv)
	if !effects.RepeatedPosition {
		return ""
	}
	if effects.IsCapture {
		return ""
	}
	if g.isLongCheckRepetition(g.Side, effects) {
		return "move causes forbidden long-check repetition"
	}
	if g.isLongChaseRepetition(effects) {
		return "move causes forbidden long-chase repetition"
	}
	if g.isIdleRepetition(effects) {
		return "move causes forbidden idle repetition"
	}
	return ""
}
```

- [ ] **Step 5: Run the long-check test to verify green state**

Run: `GOTOOLCHAIN=local go test ./... -run TestGameStateApplyRejectsForbiddenLongCheckRepetition -v`

Expected: PASS

- [ ] **Step 6: Commit the long-check rule**

```bash
git add engine.go engine_test.go
git commit -m "feat: block long-check repetition"
```

## Task 4: Implement bounded long-chase repetition detection

**Files:**
- Modify: `engine.go`
- Modify: `engine_test.go`

- [ ] **Step 1: Write the failing long-chase application test**

Append to `engine_test.go`:

```go
func TestGameStateApplyRejectsForbiddenLongChaseRepetition(t *testing.T) {
	g := GameState{
		Board: boardFromRows([]string{
			"....k....",
			".........",
			"....n....",
			"....R....",
			".........",
			".........",
			".........",
			".........",
			".........",
			"....K....",
		}),
		Side:   SideRed,
		Status: "playing",
		RuleTraces: []RuleTrace{
			{Side: SideRed, Move: "e3-f3", PositionKey: "chase-a", ChaseTargets: []string{"n@e2"}},
			{Side: SideBlack, Move: "e2-f2", PositionKey: "chase-b"},
			{Side: SideRed, Move: "f3-e3", PositionKey: "chase-a", ChaseTargets: []string{"n@f2"}, RepeatedPosition: true},
			{Side: SideBlack, Move: "f2-e2", PositionKey: "chase-b", RepeatedPosition: true},
		},
	}

	err := g.Apply("e3-f3")
	if err == nil || err.Error() != "move causes forbidden long-chase repetition" {
		t.Fatalf("expected forbidden long-chase repetition, got %v", err)
	}
}
```

- [ ] **Step 2: Run the long-chase test to verify red state**

Run: `GOTOOLCHAIN=local go test ./... -run TestGameStateApplyRejectsForbiddenLongChaseRepetition -v`

Expected: FAIL because chase targets are not yet detected

- [ ] **Step 3: Add bounded chase-target detection helpers**

Add to `engine.go`:

```go
func (g GameState) chaseTargetsForSide(attacker Side) []string {
	targets := make([]string, 0, 4)
	seen := map[string]bool{}
	for _, mv := range g.LegalMovesIgnoringRepetition(attacker) {
		target := g.Board[mv.ToY][mv.ToX]
		if target == 0 {
			continue
		}
		if pieceSide(target) == attacker {
			continue
		}
		lower := strings.ToLower(string(target))
		if lower == "k" || lower == "p" {
			continue
		}
		key := fmt.Sprintf("%s@%c%d", lower, 'a'+mv.ToX, mv.ToY)
		if seen[key] {
			continue
		}
		seen[key] = true
		targets = append(targets, key)
	}
	sort.Strings(targets)
	return targets
}

func (g GameState) LegalMovesIgnoringRepetition(side Side) []Move {
	pseudo := g.pseudoMoves(side)
	legal := make([]Move, 0, len(pseudo))
	for _, mv := range pseudo {
		next := g
		next.applyUnchecked(mv)
		if next.king(side) == nil {
			continue
		}
		if next.kingsFacing() {
			continue
		}
		if next.inCheck(side) {
			continue
		}
		legal = append(legal, mv)
	}
	return legal
}
```

- [ ] **Step 4: Add long-chase repetition evaluation**

Add to `engine.go`:

```go
func (g GameState) isLongChaseRepetition(effects moveEffects) bool {
	if effects.IsCapture || effects.GivesCheck || !effects.RepeatedPosition || len(effects.ChaseTargets) == 0 {
		return false
	}
	matches := 0
	for i := len(g.RuleTraces) - 1; i >= 0; i-- {
		trace := g.RuleTraces[i]
		if trace.Side != g.Side {
			continue
		}
		if trace.PositionKey != effects.PositionKey {
			continue
		}
		if trace.IsCapture || trace.GivesCheck {
			return false
		}
		if !sameStringSet(trace.ChaseTargets, effects.ChaseTargets) {
			continue
		}
		matches++
		if matches >= 2 {
			return true
		}
	}
	return false
}

func sameStringSet(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
```

- [ ] **Step 5: Run the long-chase test to verify green state**

Run: `GOTOOLCHAIN=local go test ./... -run TestGameStateApplyRejectsForbiddenLongChaseRepetition -v`

Expected: PASS

- [ ] **Step 6: Commit the long-chase rule**

```bash
git add engine.go engine_test.go
git commit -m "feat: block long-chase repetition"
```

## Task 5: Propagate repetition errors through match logs for human and agent paths

**Files:**
- Modify: `match.go`
- Modify: `arena_test.go`

- [ ] **Step 1: Write the failing match-level tests**

Append to `arena_test.go`:

```go
func boardFromRows(rows []string) Board {
	var b Board
	for y, row := range rows {
		for x := range row {
			if row[x] != '.' {
				b[y][x] = Piece(row[x])
			}
		}
	}
	return b
}

func TestMatchApplyHumanMovePreservesRepetitionErrorText(t *testing.T) {
	match, err := NewMatch("repeat-room", 3000, map[Side]PlayerConfig{
		SideRed:   {Type: AgentTypeHuman, Name: "Red"},
		SideBlack: {Type: AgentTypeHuman, Name: "Black"},
	}, map[Side]string{
		SideRed:   "Red",
		SideBlack: "Black",
	}, map[Side]string{
		SideRed:   "red-id",
		SideBlack: "black-id",
	})
	if err != nil {
		t.Fatalf("NewMatch() error = %v", err)
	}

	match.State = GameState{
		Board: boardFromRows([]string{
			"....k....",
			".........",
			"....R....",
			".........",
			".........",
			".........",
			".........",
			".........",
			".........",
			"....K....",
		}),
		Side:   SideRed,
		Status: "playing",
		RuleTraces: []RuleTrace{
			{Side: SideRed, Move: "e2-e1", PositionKey: "check-a", GivesCheck: true},
			{Side: SideBlack, Move: "e0-f0", PositionKey: "check-b"},
			{Side: SideRed, Move: "e1-e2", PositionKey: "check-a", GivesCheck: true, RepeatedPosition: true},
			{Side: SideBlack, Move: "f0-e0", PositionKey: "check-b", RepeatedPosition: true},
		},
	}

	err = match.ApplyHumanMove(SideRed, "e2-e1")
	if err == nil || err.Error() != "move causes forbidden long-check repetition" {
		t.Fatalf("expected long-check repetition error, got %v", err)
	}
	last := match.Logs[len(match.Logs)-1]
	if last.Error != "move causes forbidden long-check repetition" {
		t.Fatalf("expected repetition error to reach logs, got %#v", last)
	}
}

func TestMatchApplyAgentMovePreservesRepetitionErrorText(t *testing.T) {
	match, err := NewMatch("repeat-agent-room", 3000, map[Side]PlayerConfig{
		SideRed:   {Type: AgentTypePicoclaw, Name: "Red Pico"},
		SideBlack: {Type: AgentTypePicoclaw, Name: "Black Pico"},
	}, map[Side]string{
		SideRed:   "Red Pico",
		SideBlack: "Black Pico",
	}, map[Side]string{
		SideRed:   "red-id",
		SideBlack: "black-id",
	})
	if err != nil {
		t.Fatalf("NewMatch() error = %v", err)
	}

	match.State = GameState{
		Board: boardFromRows([]string{
			"....k....",
			".........",
			"....n....",
			"....R....",
			".........",
			".........",
			".........",
			".........",
			".........",
			"....K....",
		}),
		Side:   SideRed,
		Status: "playing",
		RuleTraces: []RuleTrace{
			{Side: SideRed, Move: "e3-f3", PositionKey: "chase-a", ChaseTargets: []string{"n@e2"}},
			{Side: SideBlack, Move: "e2-f2", PositionKey: "chase-b"},
			{Side: SideRed, Move: "f3-e3", PositionKey: "chase-a", ChaseTargets: []string{"n@f2"}, RepeatedPosition: true},
			{Side: SideBlack, Move: "f2-e2", PositionKey: "chase-b", RepeatedPosition: true},
		},
	}

	err = match.ApplyAgentMove(SideRed, "e3-f3", "MOVE: e3-f3")
	if err == nil || err.Error() != "move causes forbidden long-chase repetition" {
		t.Fatalf("expected long-chase repetition error, got %v", err)
	}
	last := match.Logs[len(match.Logs)-1]
	if last.Error != "move causes forbidden long-chase repetition" {
		t.Fatalf("expected repetition error to reach logs, got %#v", last)
	}
}
```

- [ ] **Step 2: Run the match-level tests to verify red state**

Run: `GOTOOLCHAIN=local go test ./... -run 'TestMatchApplyHumanMovePreservesRepetitionErrorText|TestMatchApplyAgentMovePreservesRepetitionErrorText' -v`

Expected: FAIL until the engine rules exist end-to-end and the crafted match states compile cleanly

- [ ] **Step 3: Keep repetition errors verbatim in match logs**

Review `ApplyHumanMove` and `ApplyAgentMove` in `match.go` and keep this behavior intact:

```go
if err := m.State.Apply(move); err != nil {
	now := time.Now()
	m.UpdatedAt = now
	m.appendLog(MatchLog{Time: now, Side: side, Message: "手动走子失败", Error: err.Error()})
	return err
}
```

```go
if err := m.State.Apply(move); err != nil {
	now := time.Now()
	m.UpdatedAt = now
	m.appendLog(MatchLog{Time: now, Side: side, Message: "选手返回非法走法：" + move, Reply: reply, Error: err.Error()})
	return err
}
```

If any helper refactor is needed during implementation, keep the error text unchanged and do not wrap it.

- [ ] **Step 4: Run the match-level tests to verify green state**

Run: `GOTOOLCHAIN=local go test ./... -run 'TestMatchApplyHumanMovePreservesRepetitionErrorText|TestMatchApplyAgentMovePreservesRepetitionErrorText' -v`

Expected: PASS

- [ ] **Step 5: Commit the match propagation coverage**

```bash
git add match.go arena_test.go
git commit -m "test: verify repetition errors reach match logs"
```

## Task 6: Final verification

**Files:**
- Modify: none unless verification exposes a defect

- [ ] **Step 1: Run the new engine and match repetition suite**

Run: `GOTOOLCHAIN=local go test ./... -run 'TestGameState|TestMatchApply' -v`

Expected: PASS

- [ ] **Step 2: Run the full project test suite**

Run: `GOTOOLCHAIN=local go test -count=1 ./...`

Expected: PASS

- [ ] **Step 3: Run the full build**

Run: `GOTOOLCHAIN=local go build ./...`

Expected: PASS

- [ ] **Step 4: Verify spec coverage before execution handoff**

Check that the finished implementation covers:

- position-key tracking
- rule trace persistence inside `GameState`
- legal move filtering for repetition-forbidden moves
- `Apply()` rejection with stable long-check / long-chase / idle messages
- human log propagation
- agent log propagation

- [ ] **Step 5: Commit any final fixups discovered during verification**

```bash
git add engine.go engine_test.go match.go arena_test.go
git commit -m "chore: finalize xiangqi repetition rule implementation"
```
