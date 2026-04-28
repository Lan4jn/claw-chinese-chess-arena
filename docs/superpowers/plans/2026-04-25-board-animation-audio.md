# Board Animation And Audio Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add piece-layer move animation and synthesized move/capture/game-end audio to the Xiangqi board without changing backend APIs.

**Architecture:** Keep `publicMatch` as the source of truth, introduce a frontend board animation state machine that derives one-step transitions from old/new board snapshots, and add a small audio module that plays synthesized sounds after user interaction unlock. `static/app.js` stays the orchestration layer and imports focused helpers from new modules.

**Tech Stack:** Vanilla browser JavaScript ES modules, Node `--test`, CSS

---

## File Structure

### Files to modify

- `static/app.js`
  - Wire polling snapshots into animation state, render piece layer, and trigger sounds
- `static/index.html`
  - Add a dedicated piece-layer container above the board grid
- `static/style.css`
  - Add piece-layer and animation state styles

### Files to create

- `static/board-animation.mjs`
  - Pure helpers for board parsing, move transition derivation, and animation queue state
- `static/board-animation.test.mjs`
  - Node tests for move derivation and queue behavior
- `static/board-audio.mjs`
  - Browser audio unlock and move/capture/end sound playback
- `static/board-audio.test.mjs`
  - Node tests for sound event classification and no-op fallback behavior

## Task 1: Build board animation helpers with red-green tests

**Files:**
- Create: `static/board-animation.test.mjs`
- Create: `static/board-animation.mjs`
- Test: `static/board-animation.test.mjs`

- [ ] **Step 1: Write the failing tests**

Tests must cover:

```javascript
import test from "node:test";
import assert from "node:assert/strict";

import {
  buildPieceModels,
  deriveBoardTransition,
  createAnimationController,
} from "./board-animation.mjs";

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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test static/board-animation.test.mjs`
Expected: FAIL with module-not-found for `board-animation.mjs`

- [ ] **Step 3: Implement minimal animation helper module**

Implement:

- `buildPieceModels(boardRows)`
- `deriveBoardTransition(beforeRows, afterRows, lastMove)`
- `createAnimationController()`

Keep it pure. No DOM access in this module.

- [ ] **Step 4: Run test to verify it passes**

Run: `node --test static/board-animation.test.mjs`
Expected: PASS

## Task 2: Build audio helper module with unlock and event mapping tests

**Files:**
- Create: `static/board-audio.test.mjs`
- Create: `static/board-audio.mjs`
- Test: `static/board-audio.test.mjs`

- [ ] **Step 1: Write the failing tests**

Tests must cover:

```javascript
import test from "node:test";
import assert from "node:assert/strict";

import { classifyBoardSoundEvent, createBoardAudioController } from "./board-audio.mjs";

test("classifyBoardSoundEvent returns capture for capture transitions", () => {
  assert.equal(classifyBoardSoundEvent({ capture: true, finished: false }), "capture");
});

test("classifyBoardSoundEvent returns end for finished matches", () => {
  assert.equal(classifyBoardSoundEvent({ capture: false, finished: true }), "end");
});

test("audio controller starts locked", () => {
  const controller = createBoardAudioController();
  assert.equal(controller.isUnlocked(), false);
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test static/board-audio.test.mjs`
Expected: FAIL with module-not-found for `board-audio.mjs`

- [ ] **Step 3: Implement minimal audio helper module**

Implement:

- `classifyBoardSoundEvent(transitionLike)`
- `createBoardAudioController()`
- browser-safe no-op fallback when `AudioContext` is unavailable

- [ ] **Step 4: Run test to verify it passes**

Run: `node --test static/board-audio.test.mjs`
Expected: PASS

## Task 3: Wire animation and audio into the board UI

**Files:**
- Modify: `static/index.html`
- Modify: `static/style.css`
- Modify: `static/app.js`
- Test: `static/board-animation.test.mjs`
- Test: `static/board-audio.test.mjs`

- [ ] **Step 1: Add piece-layer host element to the board**

Add a sibling overlay container above `#board-grid` for animated piece rendering.

- [ ] **Step 2: Add CSS for the piece layer**

Add:

- `.board-piece-layer`
- `.board-piece`
- `.board-piece.is-moving`
- `.board-piece.is-captured`

Keep the current board look intact.

- [ ] **Step 3: Import the new modules into `static/app.js`**

Wire:

- board base snapshot state
- current animation state
- pending snapshot queue
- audio unlock on first interaction

- [ ] **Step 4: Replace static in-cell piece rendering with piece-layer rendering**

Keep board cells for highlights and clicks, but render visible pieces through the piece-layer.

- [ ] **Step 5: Trigger animation on `publicMatch.last_move` changes**

Behavior:

- no local optimistic moves
- derive one-step transition from old/new board rows
- animate if derivation succeeds
- fall back to direct refresh if derivation fails

- [ ] **Step 6: Trigger move/capture/end sounds**

Use:

- move sound for normal transitions
- capture sound when `capturedPiece` exists
- end sound when match status becomes `finished`

- [ ] **Step 7: Run frontend module tests**

Run: `node --test static/board-animation.test.mjs static/board-audio.test.mjs`
Expected: PASS

- [ ] **Step 8: Run Go test suite**

Run: `GOTOOLCHAIN=local go test ./...`
Expected: PASS
