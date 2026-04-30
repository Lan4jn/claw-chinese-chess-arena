package main

import (
	"fmt"
	"sort"
	"strings"
)

type MoveCommentary struct {
	Move       string
	Piece      string
	Notation   string
	Plain      string
	Capture    string
	GivesCheck bool
	FromSquare string
	ToSquare   string
}

var redDigits = []string{"一", "二", "三", "四", "五", "六", "七", "八", "九"}

func buildMoveCommentary(board Board, side Side, moveText string) MoveCommentary {
	mv, err := ParseMove(moveText)
	if err != nil {
		return MoveCommentary{Move: strings.TrimSpace(moveText)}
	}
	piece := board[mv.FromY][mv.FromX]
	if piece == 0 {
		return MoveCommentary{Move: mv.String(), FromSquare: squareName(mv.FromX, mv.FromY), ToSquare: squareName(mv.ToX, mv.ToY)}
	}
	return MoveCommentary{
		Move:       mv.String(),
		Piece:      string(piece),
		Notation:   buildChineseMoveNotation(board, side, mv, piece),
		Plain:      buildPlainMoveCommentary(board, side, mv, piece),
		FromSquare: squareName(mv.FromX, mv.FromY),
		ToSquare:   squareName(mv.ToX, mv.ToY),
	}
}

func buildChineseMoveNotation(board Board, side Side, mv Move, piece Piece) string {
	name := pieceChineseName(piece)
	if name == "" {
		return ""
	}
	prefix := notationPiecePrefix(board, side, mv, piece)
	action := notationAction(side, mv)
	if action == "平" {
		return prefix + action + notationFileLabel(side, mv.ToX)
	}
	if isFileTargetNotationPiece(piece) {
		return prefix + action + notationFileLabel(side, mv.ToX)
	}
	return prefix + action + notationStepLabel(side, mv)
}

func buildPlainMoveCommentary(board Board, side Side, mv Move, piece Piece) string {
	name := pieceChineseName(piece)
	if name == "" {
		return ""
	}
	lane := plainLaneLabel(side, mv.FromX)
	action := notationAction(side, mv)
	switch asciiLower(byte(piece)) {
	case 'n':
		if action == "进" {
			return fmt.Sprintf("%s先把%s跳出来，往%s靠。", sideLabelCN(side), name, plainTargetArea(side, mv.ToX))
		}
		return fmt.Sprintf("%s把%s重新挪动，继续照看%s。", sideLabelCN(side), name, plainTargetArea(side, mv.ToX))
	case 'r', 'c':
		if action == "平" {
			return fmt.Sprintf("%s把%s的%s横到%s。", sideLabelCN(side), lane, name, plainTargetArea(side, mv.ToX))
		}
		if action == "进" {
			return fmt.Sprintf("%s让%s的%s继续往前压。", sideLabelCN(side), lane, name)
		}
		return fmt.Sprintf("%s把%s的%s先往后收一下。", sideLabelCN(side), lane, name)
	case 'p':
		if action == "平" {
			return fmt.Sprintf("%s把%s横着挪了一步，继续找接触点。", sideLabelCN(side), name)
		}
		return fmt.Sprintf("%s把%s往前顶了一步。", sideLabelCN(side), name)
	case 'k':
		return fmt.Sprintf("%s把%s稍作调整，先稳住中路。", sideLabelCN(side), name)
	default:
		if action == "平" {
			return fmt.Sprintf("%s把%s往%s调整。", sideLabelCN(side), name, plainTargetArea(side, mv.ToX))
		}
		if action == "进" {
			return fmt.Sprintf("%s继续出动%s，准备参与正面争夺。", sideLabelCN(side), name)
		}
		return fmt.Sprintf("%s把%s往回收，重新整理阵型。", sideLabelCN(side), name)
	}
}

func notationPiecePrefix(board Board, side Side, mv Move, piece Piece) string {
	positions := samePieceSameFilePositions(board, piece, mv.FromX)
	name := pieceChineseName(piece)
	if len(positions) <= 1 {
		return name + notationFileLabel(side, mv.FromX)
	}
	ordered := orderPositionsForSide(side, positions)
	for idx, pos := range ordered {
		if pos[0] == mv.FromX && pos[1] == mv.FromY {
			switch len(ordered) {
			case 2:
				if idx == 0 {
					return "前" + name
				}
				return "后" + name
			case 3:
				if idx == 0 {
					return "前" + name
				}
				if idx == 1 {
					return "中" + name
				}
				return "后" + name
			}
		}
	}
	return name + notationFileLabel(side, mv.FromX)
}

func samePieceSameFilePositions(board Board, piece Piece, fileX int) [][2]int {
	out := make([][2]int, 0, 3)
	for y := 0; y < len(board); y++ {
		if board[y][fileX] == piece {
			out = append(out, [2]int{fileX, y})
		}
	}
	return out
}

func orderPositionsForSide(side Side, positions [][2]int) [][2]int {
	cp := append([][2]int(nil), positions...)
	sort.Slice(cp, func(i, j int) bool {
		if side == SideRed {
			return cp[i][1] < cp[j][1]
		}
		return cp[i][1] > cp[j][1]
	})
	return cp
}

func notationAction(side Side, mv Move) string {
	if mv.FromY == mv.ToY {
		return "平"
	}
	if isForwardMove(side, mv.FromY, mv.ToY) {
		return "进"
	}
	return "退"
}

func notationStepLabel(side Side, mv Move) string {
	steps := absInt(mv.ToY - mv.FromY)
	if side == SideRed {
		if steps >= 1 && steps <= 9 {
			return redDigits[steps-1]
		}
	}
	return fmt.Sprintf("%d", steps)
}

func notationFileLabel(side Side, fileX int) string {
	number := fileNumberForSide(side, fileX)
	if side == SideRed {
		return redDigits[number-1]
	}
	return fmt.Sprintf("%d", number)
}

func fileNumberForSide(side Side, fileX int) int {
	if side == SideRed {
		return 9 - fileX
	}
	return fileX + 1
}

func pieceChineseName(piece Piece) string {
	switch asciiLower(byte(piece)) {
	case 'r':
		return "车"
	case 'n':
		return "马"
	case 'b':
		if piece >= 'A' && piece <= 'Z' {
			return "相"
		}
		return "象"
	case 'a':
		if piece >= 'A' && piece <= 'Z' {
			return "仕"
		}
		return "士"
	case 'k':
		if piece >= 'A' && piece <= 'Z' {
			return "帅"
		}
		return "将"
	case 'c':
		return "炮"
	case 'p':
		if piece >= 'A' && piece <= 'Z' {
			return "兵"
		}
		return "卒"
	default:
		return ""
	}
}

func isFileTargetNotationPiece(piece Piece) bool {
	switch asciiLower(byte(piece)) {
	case 'n', 'b', 'a':
		return true
	default:
		return false
	}
}

func plainLaneLabel(side Side, fileX int) string {
	number := fileNumberForSide(side, fileX)
	switch {
	case number <= 3:
		return "右路"
	case number >= 7:
		return "左路"
	default:
		return "中路"
	}
}

func plainTargetArea(side Side, fileX int) string {
	number := fileNumberForSide(side, fileX)
	switch {
	case number <= 2:
		return "边线"
	case number <= 4:
		return "右翼"
	case number == 5:
		return "中路"
	case number <= 7:
		return "左翼"
	default:
		return "边线"
	}
}

func sideLabelCN(side Side) string {
	if side == SideRed {
		return "红方"
	}
	return "黑方"
}

func isForwardMove(side Side, fromY, toY int) bool {
	if side == SideRed {
		return toY < fromY
	}
	return toY > fromY
}

func squareName(x, y int) string {
	return fmt.Sprintf("%c%d", rune('a'+x), y)
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func asciiLower(v byte) byte {
	if v >= 'A' && v <= 'Z' {
		return v + 32
	}
	return v
}
