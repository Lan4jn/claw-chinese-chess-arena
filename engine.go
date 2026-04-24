package main

import (
	"fmt"
	"strings"
	"unicode"
)

type Side string

const (
	SideRed   Side = "red"
	SideBlack Side = "black"
)

type Piece byte

type Board [10][9]Piece

type Move struct {
	FromX int `json:"from_x"`
	FromY int `json:"from_y"`
	ToX   int `json:"to_x"`
	ToY   int `json:"to_y"`
}

type MoveRecord struct {
	Side    Side   `json:"side"`
	Move    string `json:"move"`
	Piece   string `json:"piece"`
	Capture string `json:"capture,omitempty"`
}

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

func NewGame() GameState {
	var b Board
	rows := []string{
		"rnbakabnr",
		".........",
		".c.....c.",
		"p.p.p.p.p",
		".........",
		".........",
		"P.P.P.P.P",
		".C.....C.",
		".........",
		"RNBAKABNR",
	}
	for y, row := range rows {
		for x := range row {
			if row[x] != '.' {
				b[y][x] = Piece(row[x])
			}
		}
	}
	return GameState{Board: b, Side: SideRed, Status: "playing"}
}

func (g GameState) LegalMoveStrings() []string {
	moves := g.LegalMoves(g.Side)
	out := make([]string, 0, len(moves))
	for _, mv := range moves {
		out = append(out, mv.String())
	}
	return out
}

func (g GameState) PositionKey() string {
	rows := BoardRows(g.Board)
	return string(g.Side) + "|" + strings.Join(rows, "/")
}

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

func (g GameState) repetitionViolationForMove(mv Move) string {
	_ = mv
	return ""
}

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
		return fmt.Errorf("%s is not a legal move for %s", moveText, g.Side)
	}
	piece := g.Board[mv.FromY][mv.FromX]
	captured := g.Board[mv.ToY][mv.ToX]
	g.applyUnchecked(mv)
	g.LastMove = mv.String()
	g.MoveCount++
	g.History = append(g.History, MoveRecord{
		Side:    opposite(g.Side),
		Move:    mv.String(),
		Piece:   string(piece),
		Capture: pieceString(captured),
	})
	if g.king(g.Side) == nil {
		g.Winner = opposite(g.Side)
		g.Status = "finished"
		g.Reason = "king captured"
		return nil
	}
	if len(g.LegalMoves(g.Side)) == 0 {
		g.Winner = opposite(g.Side)
		g.Status = "finished"
		g.Reason = "no legal moves"
	}
	return nil
}

func (g *GameState) applyUnchecked(mv Move) {
	p := g.Board[mv.FromY][mv.FromX]
	g.Board[mv.FromY][mv.FromX] = 0
	g.Board[mv.ToY][mv.ToX] = p
	g.Side = opposite(g.Side)
}

func (g GameState) pseudoMoves(side Side) []Move {
	var moves []Move
	for y := 0; y < 10; y++ {
		for x := 0; x < 9; x++ {
			p := g.Board[y][x]
			if p == 0 || pieceSide(p) != side {
				continue
			}
			moves = append(moves, g.pseudoPieceMoves(x, y, p)...)
		}
	}
	return moves
}

func (g GameState) pseudoPieceMoves(x, y int, p Piece) []Move {
	switch unicode.ToLower(rune(p)) {
	case 'k':
		return g.kingMoves(x, y, p)
	case 'a':
		return g.advisorMoves(x, y, p)
	case 'b':
		return g.elephantMoves(x, y, p)
	case 'n':
		return g.horseMoves(x, y, p)
	case 'r':
		return g.rookMoves(x, y, p)
	case 'c':
		return g.cannonMoves(x, y, p)
	case 'p':
		return g.pawnMoves(x, y, p)
	default:
		return nil
	}
}

func (g GameState) kingMoves(x, y int, p Piece) []Move {
	var out []Move
	for _, d := range [][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}} {
		nx, ny := x+d[0], y+d[1]
		if inPalace(nx, ny, pieceSide(p)) && g.canLand(nx, ny, p) {
			out = append(out, Move{x, y, nx, ny})
		}
	}
	return out
}

func (g GameState) advisorMoves(x, y int, p Piece) []Move {
	var out []Move
	for _, d := range [][2]int{{-1, -1}, {1, -1}, {-1, 1}, {1, 1}} {
		nx, ny := x+d[0], y+d[1]
		if inPalace(nx, ny, pieceSide(p)) && g.canLand(nx, ny, p) {
			out = append(out, Move{x, y, nx, ny})
		}
	}
	return out
}

func (g GameState) elephantMoves(x, y int, p Piece) []Move {
	var out []Move
	for _, d := range [][2]int{{-2, -2}, {2, -2}, {-2, 2}, {2, 2}} {
		nx, ny := x+d[0], y+d[1]
		eyeX, eyeY := x+d[0]/2, y+d[1]/2
		if !inBoard(nx, ny) || g.Board[eyeY][eyeX] != 0 || !g.canLand(nx, ny, p) {
			continue
		}
		side := pieceSide(p)
		if side == SideRed && ny < 5 {
			continue
		}
		if side == SideBlack && ny > 4 {
			continue
		}
		out = append(out, Move{x, y, nx, ny})
	}
	return out
}

func (g GameState) horseMoves(x, y int, p Piece) []Move {
	type jump struct{ dx, dy, lx, ly int }
	jumps := []jump{
		{-1, -2, 0, -1}, {1, -2, 0, -1}, {-1, 2, 0, 1}, {1, 2, 0, 1},
		{-2, -1, -1, 0}, {-2, 1, -1, 0}, {2, -1, 1, 0}, {2, 1, 1, 0},
	}
	var out []Move
	for _, j := range jumps {
		nx, ny := x+j.dx, y+j.dy
		lx, ly := x+j.lx, y+j.ly
		if inBoard(nx, ny) && g.Board[ly][lx] == 0 && g.canLand(nx, ny, p) {
			out = append(out, Move{x, y, nx, ny})
		}
	}
	return out
}

func (g GameState) rookMoves(x, y int, p Piece) []Move {
	return g.rayMoves(x, y, p, false)
}

func (g GameState) cannonMoves(x, y int, p Piece) []Move {
	var out []Move
	for _, d := range [][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}} {
		screen := false
		for nx, ny := x+d[0], y+d[1]; inBoard(nx, ny); nx, ny = nx+d[0], ny+d[1] {
			target := g.Board[ny][nx]
			if !screen {
				if target == 0 {
					out = append(out, Move{x, y, nx, ny})
					continue
				}
				screen = true
				continue
			}
			if target == 0 {
				continue
			}
			if pieceSide(target) != pieceSide(p) {
				out = append(out, Move{x, y, nx, ny})
			}
			break
		}
	}
	return out
}

func (g GameState) rayMoves(x, y int, p Piece, _ bool) []Move {
	var out []Move
	for _, d := range [][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}} {
		for nx, ny := x+d[0], y+d[1]; inBoard(nx, ny); nx, ny = nx+d[0], ny+d[1] {
			target := g.Board[ny][nx]
			if target == 0 {
				out = append(out, Move{x, y, nx, ny})
				continue
			}
			if pieceSide(target) != pieceSide(p) {
				out = append(out, Move{x, y, nx, ny})
			}
			break
		}
	}
	return out
}

func (g GameState) pawnMoves(x, y int, p Piece) []Move {
	side := pieceSide(p)
	var dirs [][2]int
	if side == SideRed {
		dirs = append(dirs, [2]int{0, -1})
		if y <= 4 {
			dirs = append(dirs, [2]int{-1, 0}, [2]int{1, 0})
		}
	} else {
		dirs = append(dirs, [2]int{0, 1})
		if y >= 5 {
			dirs = append(dirs, [2]int{-1, 0}, [2]int{1, 0})
		}
	}
	var out []Move
	for _, d := range dirs {
		nx, ny := x+d[0], y+d[1]
		if inBoard(nx, ny) && g.canLand(nx, ny, p) {
			out = append(out, Move{x, y, nx, ny})
		}
	}
	return out
}

func (g GameState) canLand(x, y int, p Piece) bool {
	if !inBoard(x, y) {
		return false
	}
	target := g.Board[y][x]
	return target == 0 || pieceSide(target) != pieceSide(p)
}

func (g GameState) inCheck(side Side) bool {
	king := g.king(side)
	if king == nil {
		return true
	}
	for _, mv := range g.pseudoMoves(opposite(side)) {
		if mv.ToX == king[0] && mv.ToY == king[1] {
			return true
		}
	}
	return false
}

func (g GameState) kingsFacing() bool {
	red := g.king(SideRed)
	black := g.king(SideBlack)
	if red == nil || black == nil || red[0] != black[0] {
		return false
	}
	x := red[0]
	from, to := black[1]+1, red[1]
	for y := from; y < to; y++ {
		if g.Board[y][x] != 0 {
			return false
		}
	}
	return true
}

func (g GameState) king(side Side) *[2]int {
	want := Piece('K')
	if side == SideBlack {
		want = Piece('k')
	}
	for y := 0; y < 10; y++ {
		for x := 0; x < 9; x++ {
			if g.Board[y][x] == want {
				return &[2]int{x, y}
			}
		}
	}
	return nil
}

func ParseMove(s string) (Move, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if len(s) != 5 || s[2] != '-' {
		return Move{}, fmt.Errorf("move must look like a0-a1")
	}
	fromX, fromY := int(s[0]-'a'), int(s[1]-'0')
	toX, toY := int(s[3]-'a'), int(s[4]-'0')
	if !inBoard(fromX, fromY) || !inBoard(toX, toY) {
		return Move{}, fmt.Errorf("move out of board: %s", s)
	}
	return Move{FromX: fromX, FromY: fromY, ToX: toX, ToY: toY}, nil
}

func (m Move) String() string {
	return fmt.Sprintf("%c%d-%c%d", 'a'+m.FromX, m.FromY, 'a'+m.ToX, m.ToY)
}

func BoardText(b Board) string {
	var sb strings.Builder
	sb.WriteString("   a b c d e f g h i\n")
	for y := 0; y < 10; y++ {
		sb.WriteString(fmt.Sprintf("%d  ", y))
		for x := 0; x < 9; x++ {
			p := b[y][x]
			if p == 0 {
				sb.WriteByte('.')
			} else {
				sb.WriteByte(byte(p))
			}
			if x != 8 {
				sb.WriteByte(' ')
			}
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

func BoardRows(b Board) []string {
	rows := make([]string, 0, len(b))
	for y := 0; y < 10; y++ {
		var sb strings.Builder
		for x := 0; x < 9; x++ {
			p := b[y][x]
			if p == 0 {
				sb.WriteByte('.')
				continue
			}
			sb.WriteByte(byte(p))
		}
		rows = append(rows, sb.String())
	}
	return rows
}

func inBoard(x, y int) bool {
	return x >= 0 && x < 9 && y >= 0 && y < 10
}

func inPalace(x, y int, side Side) bool {
	if x < 3 || x > 5 {
		return false
	}
	if side == SideRed {
		return y >= 7 && y <= 9
	}
	return y >= 0 && y <= 2
}

func pieceSide(p Piece) Side {
	if p == 0 {
		return ""
	}
	if unicode.IsUpper(rune(p)) {
		return SideRed
	}
	return SideBlack
}

func pieceString(p Piece) string {
	if p == 0 {
		return ""
	}
	return string(p)
}

func opposite(s Side) Side {
	if s == SideRed {
		return SideBlack
	}
	return SideRed
}
