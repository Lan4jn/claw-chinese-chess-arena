package main

import "testing"

func TestGameStateLegalMovesExcludeForbiddenIdleRepetition(t *testing.T) {
	g := GameState{
		Board: boardFromRows([]string{
			"....k....",
			".........",
			".........",
			".........",
			"....P....",
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
