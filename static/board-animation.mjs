function normalizeRows(rows) {
  return Array.isArray(rows) ? rows.map((row) => String(row || ".........")) : [];
}

function squareForCoords(row, col) {
  return String.fromCharCode(97 + col) + String(row);
}

function parseSquare(square) {
  const raw = String(square || "").trim().toLowerCase();
  if (raw.length !== 2) {
    return null;
  }
  return {
    row: Number(raw[1]),
    col: raw.charCodeAt(0) - 97,
  };
}

export function buildPieceModels(boardRows) {
  const rows = normalizeRows(boardRows);
  const pieces = [];
  const counters = new Map();
  for (let row = 0; row < rows.length; row += 1) {
    const rowText = rows[row] || ".........";
    for (let col = 0; col < rowText.length; col += 1) {
      const piece = rowText[col];
      if (!piece || piece === ".") {
        continue;
      }
      const count = (counters.get(piece) || 0) + 1;
      counters.set(piece, count);
      pieces.push({
        id: `${piece}-${count}-${row}-${col}`,
        piece,
        row,
        col,
        square: squareForCoords(row, col),
      });
    }
  }
  return pieces;
}

export function deriveBoardTransition(beforeRows, afterRows, lastMove) {
  const before = normalizeRows(beforeRows);
  const after = normalizeRows(afterRows);
  const rawMove = String(lastMove || "").trim().toLowerCase();
  const [from, to] = rawMove.split("-");
  if (!from || !to) {
    return null;
  }
  const fromCoords = parseSquare(from);
  const toCoords = parseSquare(to);
  if (!fromCoords || !toCoords) {
    return null;
  }
  const piece = before[fromCoords.row]?.[fromCoords.col] || ".";
  const capturedPiece = before[toCoords.row]?.[toCoords.col] || ".";
  const nextPiece = after[toCoords.row]?.[toCoords.col] || ".";
  const nextFrom = after[fromCoords.row]?.[fromCoords.col] || ".";
  if (piece === "." || nextPiece !== piece || nextFrom !== ".") {
    return null;
  }
  return {
    move: rawMove,
    from,
    to,
    piece,
    capture: capturedPiece !== ".",
    capturedPiece: capturedPiece !== "." ? capturedPiece : "",
  };
}

export function createAnimationController() {
  let active = null;
  let pendingSnapshot = null;
  return {
    isActive() {
      return active !== null;
    },
    start(transition) {
      active = transition;
    },
    finish() {
      active = null;
    },
    getActive() {
      return active;
    },
    queueSnapshot(snapshot) {
      pendingSnapshot = snapshot;
    },
    getPendingSnapshot() {
      return pendingSnapshot;
    },
    clearPendingSnapshot() {
      const snapshot = pendingSnapshot;
      pendingSnapshot = null;
      return snapshot;
    },
  };
}
