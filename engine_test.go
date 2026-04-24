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
