package main

import "testing"

func TestGameStateLegalMovesExcludeForbiddenIdleRepetition(t *testing.T) {
	base := GameState{
		Board: boardFromRows([]string{
			"....k....",
			".........",
			".........",
			".........",
			"....PP...",
			".........",
			".........",
			".........",
			"....R....",
			"....K....",
		}),
		Side:   SideRed,
		Status: "playing",
	}
	afterRedOut := stateAfterMove(t, base, "e8-f8")
	afterBlackOut := stateAfterMove(t, afterRedOut, "e0-f0")
	afterRedBack := stateAfterMove(t, afterBlackOut, "f8-e8")
	afterBlackBack := stateAfterMove(t, afterRedBack, "f0-e0")

	g := base
	g.RuleTraces = []RuleTrace{
		{Side: SideRed, Move: "e8-f8", PositionKey: afterRedOut.PositionKey()},
		{Side: SideBlack, Move: "e0-f0", PositionKey: afterBlackOut.PositionKey()},
		{Side: SideRed, Move: "f8-e8", PositionKey: afterRedBack.PositionKey()},
		{Side: SideBlack, Move: "f0-e0", PositionKey: afterBlackBack.PositionKey()},
		{Side: SideRed, Move: "e8-f8", PositionKey: afterRedOut.PositionKey(), RepeatedPosition: true},
		{Side: SideBlack, Move: "e0-f0", PositionKey: afterBlackOut.PositionKey(), RepeatedPosition: true},
		{Side: SideRed, Move: "f8-e8", PositionKey: afterRedBack.PositionKey(), RepeatedPosition: true},
		{Side: SideBlack, Move: "f0-e0", PositionKey: afterBlackBack.PositionKey(), RepeatedPosition: true},
	}

	moves := g.LegalMoveStrings()
	if slicesContains(moves, "e8-f8") {
		t.Fatalf("expected idle repetition move e8-f8 to be filtered out, got %v", moves)
	}
}

func TestGameStateLegalMovesExcludeForbiddenLongCheck(t *testing.T) {
	base := GameState{
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
	}
	afterRedCheck := stateAfterMove(t, base, "e2-e1")
	afterBlackOut := stateAfterMove(t, afterRedCheck, "e0-f0")
	afterRedBack := stateAfterMove(t, afterBlackOut, "e1-e2")
	afterBlackBack := stateAfterMove(t, afterRedBack, "f0-e0")

	g := base
	g.RuleTraces = []RuleTrace{
		{Side: SideRed, Move: "e2-e1", PositionKey: afterRedCheck.PositionKey(), GivesCheck: true},
		{Side: SideBlack, Move: "e0-f0", PositionKey: afterBlackOut.PositionKey()},
		{Side: SideRed, Move: "e1-e2", PositionKey: afterRedBack.PositionKey()},
		{Side: SideBlack, Move: "f0-e0", PositionKey: afterBlackBack.PositionKey()},
		{Side: SideRed, Move: "e2-e1", PositionKey: afterRedCheck.PositionKey(), GivesCheck: true, RepeatedPosition: true},
		{Side: SideBlack, Move: "e0-f0", PositionKey: afterBlackOut.PositionKey(), RepeatedPosition: true},
		{Side: SideRed, Move: "e1-e2", PositionKey: afterRedBack.PositionKey(), RepeatedPosition: true},
		{Side: SideBlack, Move: "f0-e0", PositionKey: afterBlackBack.PositionKey(), RepeatedPosition: true},
	}

	moves := g.LegalMoveStrings()
	if slicesContains(moves, "e2-e1") {
		t.Fatalf("expected long-check move e2-e1 to be filtered out, got %v", moves)
	}
}

func TestGameStateLegalMovesExcludeForbiddenLongChase(t *testing.T) {
	base := GameState{
		Board: boardFromRows([]string{
			"...k.....",
			".........",
			".....n...",
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
	}
	afterRedChase := stateAfterMove(t, base, "e3-e2")
	afterBlackOut := stateAfterMove(t, afterRedChase, "d0-d1")
	afterRedReset := stateAfterMove(t, afterBlackOut, "e2-e3")
	afterBlackBack := stateAfterMove(t, afterRedReset, "d1-d0")

	g := base
	g.RuleTraces = []RuleTrace{
		{Side: SideRed, Move: "e3-e2", PositionKey: afterRedChase.PositionKey(), ChaseTargets: []string{"n@f2"}},
		{Side: SideBlack, Move: "d0-d1", PositionKey: afterBlackOut.PositionKey()},
		{Side: SideRed, Move: "e2-e3", PositionKey: afterRedReset.PositionKey()},
		{Side: SideBlack, Move: "d1-d0", PositionKey: afterBlackBack.PositionKey()},
		{Side: SideRed, Move: "e3-e2", PositionKey: afterRedChase.PositionKey(), ChaseTargets: []string{"n@f2"}, RepeatedPosition: true},
		{Side: SideBlack, Move: "d0-d1", PositionKey: afterBlackOut.PositionKey(), RepeatedPosition: true},
		{Side: SideRed, Move: "e2-e3", PositionKey: afterRedReset.PositionKey(), RepeatedPosition: true},
		{Side: SideBlack, Move: "d1-d0", PositionKey: afterBlackBack.PositionKey(), RepeatedPosition: true},
	}

	moves := g.LegalMoveStrings()
	if slicesContains(moves, "e3-e2") {
		t.Fatalf("expected long-chase move e3-e2 to be filtered out, got %v", moves)
	}
}

func TestGameStateApplyRejectsForbiddenIdleRepetition(t *testing.T) {
	base := GameState{
		Board: boardFromRows([]string{
			"....k....",
			".........",
			".........",
			".........",
			"....PP...",
			".........",
			".........",
			".........",
			"....R....",
			"....K....",
		}),
		Side:   SideRed,
		Status: "playing",
	}
	afterRedOut := stateAfterMove(t, base, "e8-f8")
	afterBlackOut := stateAfterMove(t, afterRedOut, "e0-f0")
	afterRedBack := stateAfterMove(t, afterBlackOut, "f8-e8")
	afterBlackBack := stateAfterMove(t, afterRedBack, "f0-e0")
	if afterBlackBack.PositionKey() != base.PositionKey() {
		t.Fatalf("expected loop to return to base position, got %s want %s", afterBlackBack.PositionKey(), base.PositionKey())
	}

	g := base
	g.RuleTraces = []RuleTrace{
		{Side: SideRed, Move: "e8-f8", PositionKey: afterRedOut.PositionKey()},
		{Side: SideBlack, Move: "e0-f0", PositionKey: afterBlackOut.PositionKey()},
		{Side: SideRed, Move: "f8-e8", PositionKey: afterRedBack.PositionKey()},
		{Side: SideBlack, Move: "f0-e0", PositionKey: afterBlackBack.PositionKey()},
		{Side: SideRed, Move: "e8-f8", PositionKey: afterRedOut.PositionKey(), RepeatedPosition: true},
		{Side: SideBlack, Move: "e0-f0", PositionKey: afterBlackOut.PositionKey(), RepeatedPosition: true},
		{Side: SideRed, Move: "f8-e8", PositionKey: afterRedBack.PositionKey(), RepeatedPosition: true},
		{Side: SideBlack, Move: "f0-e0", PositionKey: afterBlackBack.PositionKey(), RepeatedPosition: true},
	}

	err := g.Apply("e8-f8")
	if err == nil || err.Error() != "move causes forbidden idle repetition" {
		t.Fatalf("expected forbidden idle repetition, got %v", err)
	}
}

func TestGameStateApplyAllowsCaptureThatBreaksLoop(t *testing.T) {
	base := GameState{
		Board: boardFromRows([]string{
			"....k....",
			".........",
			".........",
			".........",
			"....P....",
			".........",
			".........",
			"....r....",
			"....R....",
			"....K....",
		}),
		Side:   SideRed,
		Status: "playing",
	}
	afterRedOut := stateAfterMove(t, base, "e8-f8")
	afterBlackOut := stateAfterMove(t, afterRedOut, "e7-f7")
	afterRedBack := stateAfterMove(t, afterBlackOut, "f8-e8")
	afterBlackBack := stateAfterMove(t, afterRedBack, "f7-e7")
	if afterBlackBack.PositionKey() != base.PositionKey() {
		t.Fatalf("expected loop to return to base position, got %s want %s", afterBlackBack.PositionKey(), base.PositionKey())
	}

	g := base
	g.RuleTraces = []RuleTrace{
		{Side: SideRed, Move: "e8-f8", PositionKey: afterRedOut.PositionKey()},
		{Side: SideBlack, Move: "e7-f7", PositionKey: afterBlackOut.PositionKey()},
		{Side: SideRed, Move: "f8-e8", PositionKey: afterRedBack.PositionKey()},
		{Side: SideBlack, Move: "f7-e7", PositionKey: afterBlackBack.PositionKey(), RepeatedPosition: true},
	}

	if err := g.Apply("e8-e7"); err != nil {
		t.Fatalf("expected capture to break repetition, got %v", err)
	}
}

func TestGameStateApplyAllowsIdleMoveAfterRepetitionChainReset(t *testing.T) {
	base := GameState{
		Board: boardFromRows([]string{
			"....k....",
			".........",
			".........",
			".........",
			"....PP...",
			".........",
			".........",
			".........",
			"....R....",
			"....K....",
		}),
		Side:   SideRed,
		Status: "playing",
	}
	afterRedOut := stateAfterMove(t, base, "e8-f8")
	afterBlackOut := stateAfterMove(t, afterRedOut, "e0-f0")
	afterRedBack := stateAfterMove(t, afterBlackOut, "f8-e8")
	afterBlackBack := stateAfterMove(t, afterRedBack, "f0-e0")
	afterBreakRedOut := stateAfterMove(t, afterBlackBack, "e8-d8")
	afterBreakBlackOut := stateAfterMove(t, afterBreakRedOut, "e0-d0")
	afterBreakRedBack := stateAfterMove(t, afterBreakBlackOut, "d8-e8")
	afterBreakBlackBack := stateAfterMove(t, afterBreakRedBack, "d0-e0")
	if afterBreakBlackBack.PositionKey() != base.PositionKey() {
		t.Fatalf("expected break sequence to return to base position, got %s want %s", afterBreakBlackBack.PositionKey(), base.PositionKey())
	}

	g := base
	g.RuleTraces = []RuleTrace{
		{Side: SideRed, Move: "e8-f8", PositionKey: afterRedOut.PositionKey()},
		{Side: SideBlack, Move: "e0-f0", PositionKey: afterBlackOut.PositionKey()},
		{Side: SideRed, Move: "f8-e8", PositionKey: afterRedBack.PositionKey()},
		{Side: SideBlack, Move: "f0-e0", PositionKey: afterBlackBack.PositionKey()},
		{Side: SideRed, Move: "e8-f8", PositionKey: afterRedOut.PositionKey(), RepeatedPosition: true},
		{Side: SideBlack, Move: "e0-f0", PositionKey: afterBlackOut.PositionKey(), RepeatedPosition: true},
		{Side: SideRed, Move: "f8-e8", PositionKey: afterRedBack.PositionKey(), RepeatedPosition: true},
		{Side: SideBlack, Move: "f0-e0", PositionKey: afterBlackBack.PositionKey(), RepeatedPosition: true},
		{Side: SideRed, Move: "e8-d8", PositionKey: afterBreakRedOut.PositionKey()},
		{Side: SideBlack, Move: "e0-d0", PositionKey: afterBreakBlackOut.PositionKey()},
		{Side: SideRed, Move: "d8-e8", PositionKey: afterBreakRedBack.PositionKey()},
		{Side: SideBlack, Move: "d0-e0", PositionKey: afterBreakBlackBack.PositionKey()},
	}

	if !slicesContains(g.LegalMoveStrings(), "e8-f8") {
		t.Fatalf("expected chain reset to keep e8-f8 legal, got %v", g.LegalMoveStrings())
	}
	if err := g.Apply("e8-f8"); err != nil {
		t.Fatalf("expected chain reset to allow idle move, got %v", err)
	}
}

func TestGameStateApplyRejectsForbiddenLongCheckRepetition(t *testing.T) {
	base := GameState{
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
	}
	afterRedCheck := stateAfterMove(t, base, "e2-e1")
	afterBlackOut := stateAfterMove(t, afterRedCheck, "e0-f0")
	afterRedBack := stateAfterMove(t, afterBlackOut, "e1-e2")
	afterBlackBack := stateAfterMove(t, afterRedBack, "f0-e0")

	g := base
	g.RuleTraces = []RuleTrace{
		{Side: SideRed, Move: "e2-e1", PositionKey: afterRedCheck.PositionKey(), GivesCheck: true},
		{Side: SideBlack, Move: "e0-f0", PositionKey: afterBlackOut.PositionKey()},
		{Side: SideRed, Move: "e1-e2", PositionKey: afterRedBack.PositionKey()},
		{Side: SideBlack, Move: "f0-e0", PositionKey: afterBlackBack.PositionKey()},
		{Side: SideRed, Move: "e2-e1", PositionKey: afterRedCheck.PositionKey(), GivesCheck: true, RepeatedPosition: true},
		{Side: SideBlack, Move: "e0-f0", PositionKey: afterBlackOut.PositionKey(), RepeatedPosition: true},
		{Side: SideRed, Move: "e1-e2", PositionKey: afterRedBack.PositionKey(), RepeatedPosition: true},
		{Side: SideBlack, Move: "f0-e0", PositionKey: afterBlackBack.PositionKey(), RepeatedPosition: true},
	}

	err := g.Apply("e2-e1")
	if err == nil || err.Error() != "move causes forbidden long-check repetition" {
		t.Fatalf("expected forbidden long-check repetition, got %v", err)
	}
}

func TestGameStateApplyRejectsForbiddenLongChaseRepetition(t *testing.T) {
	base := GameState{
		Board: boardFromRows([]string{
			"...k.....",
			".........",
			".....n...",
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
	}
	afterRedChase := stateAfterMove(t, base, "e3-e2")
	afterBlackOut := stateAfterMove(t, afterRedChase, "d0-d1")
	afterRedReset := stateAfterMove(t, afterBlackOut, "e2-e3")
	afterBlackBack := stateAfterMove(t, afterRedReset, "d1-d0")
	if afterBlackBack.PositionKey() != base.PositionKey() {
		t.Fatalf("expected loop to return to base position, got %s want %s", afterBlackBack.PositionKey(), base.PositionKey())
	}

	g := base
	g.RuleTraces = []RuleTrace{
		{Side: SideRed, Move: "e3-e2", PositionKey: afterRedChase.PositionKey(), ChaseTargets: []string{"n@f2"}},
		{Side: SideBlack, Move: "d0-d1", PositionKey: afterBlackOut.PositionKey()},
		{Side: SideRed, Move: "e2-e3", PositionKey: afterRedReset.PositionKey()},
		{Side: SideBlack, Move: "d1-d0", PositionKey: afterBlackBack.PositionKey()},
		{Side: SideRed, Move: "e3-e2", PositionKey: afterRedChase.PositionKey(), ChaseTargets: []string{"n@f2"}, RepeatedPosition: true},
		{Side: SideBlack, Move: "d0-d1", PositionKey: afterBlackOut.PositionKey(), RepeatedPosition: true},
		{Side: SideRed, Move: "e2-e3", PositionKey: afterRedReset.PositionKey(), RepeatedPosition: true},
		{Side: SideBlack, Move: "d1-d0", PositionKey: afterBlackBack.PositionKey(), RepeatedPosition: true},
	}

	err := g.Apply("e3-e2")
	if err == nil || err.Error() != "move causes forbidden long-chase repetition" {
		t.Fatalf("expected forbidden long-chase repetition, got %v", err)
	}
}

func TestGameStateApplyKeepsOrdinaryIllegalMoveErrorOutsideRepetitionFilter(t *testing.T) {
	base := GameState{
		Board: boardFromRows([]string{
			"....k....",
			".........",
			".........",
			".........",
			"....PP...",
			".........",
			".........",
			".........",
			"....R....",
			"....K....",
		}),
		Side:   SideRed,
		Status: "playing",
	}
	bogus := stateAfterMove(t, base, "a0-a1")

	g := base
	g.RuleTraces = []RuleTrace{
		{Side: SideRed, Move: "a0-a1", PositionKey: bogus.PositionKey()},
		{Side: SideBlack, Move: "a1-a0", PositionKey: base.PositionKey()},
		{Side: SideRed, Move: "a0-a1", PositionKey: bogus.PositionKey(), RepeatedPosition: true},
	}

	err := g.Apply("a0-a1")
	if err == nil || err.Error() != "a0-a1 is not a legal move for red" {
		t.Fatalf("expected ordinary illegal move error, got %v", err)
	}
}

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

func stateAfterMove(t *testing.T, g GameState, move string) GameState {
	t.Helper()
	mv, err := ParseMove(move)
	if err != nil {
		t.Fatalf("parse move %s: %v", move, err)
	}
	next := g
	next.applyUnchecked(mv)
	return next
}
