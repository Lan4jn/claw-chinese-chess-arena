import test from "node:test";
import assert from "node:assert/strict";

import {
  buildPieceModels,
  createAnimationController,
  deriveBoardTransition,
} from "./board-animation.mjs";

test("buildPieceModels returns one model per non-empty square", () => {
  const pieces = buildPieceModels([
    "r........",
    ".........",
    "....k....",
    ".........",
    ".........",
    ".........",
    ".........",
    ".........",
    ".........",
    "....K....",
  ]);

  assert.equal(pieces.length, 3);
  assert.equal(pieces[0].piece, "r");
  assert.equal(pieces[0].square, "a0");
});

test("deriveBoardTransition identifies a normal move", () => {
  const before = [
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
  ];
  const after = structuredClone(before);
  after[9] = "RNBAKABN.";
  after[8] = "........R";

  const transition = deriveBoardTransition(before, after, "i9-i8");

  assert.equal(transition.move, "i9-i8");
  assert.equal(transition.from, "i9");
  assert.equal(transition.to, "i8");
  assert.equal(transition.capture, false);
  assert.equal(transition.piece, "R");
});

test("deriveBoardTransition identifies a capture", () => {
  const before = [
    "....k....",
    ".........",
    "....R....",
    "....p....",
    ".........",
    ".........",
    ".........",
    ".........",
    ".........",
    "....K....",
  ];
  const after = [
    "....k....",
    ".........",
    ".........",
    "....R....",
    ".........",
    ".........",
    ".........",
    ".........",
    ".........",
    "....K....",
  ];

  const transition = deriveBoardTransition(before, after, "e2-e3");

  assert.equal(transition.capture, true);
  assert.equal(transition.capturedPiece, "p");
});

test("controller queues a pending snapshot while animation is active", () => {
  const controller = createAnimationController();
  controller.start({ move: "a0-a1" });
  controller.queueSnapshot({ last_move: "b0-b1" });

  assert.equal(controller.getPendingSnapshot().last_move, "b0-b1");
});
