# Commentary Stage Rework Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the current log-shaped commentary panel with a real xiangqi commentary stage that supports move-style switching and analysis-intensity switching.

**Architecture:** Keep commentary generation frontend-only. Split the work into pure move-description helpers, pure analysis helpers, and a thin `app.js` integration layer with persisted user settings.

**Tech Stack:** Vanilla ES modules, Node `--test`, existing embedded static frontend

---

## File Structure

### Files to modify

- `static/commentary.mjs`
  - Upgrade from simple text mapping to layered commentary generation
- `static/commentary.test.mjs`
  - Add move style and analysis intensity coverage
- `static/app.js`
  - Persist settings and pass them into commentary rendering
- `static/index.html`
  - Add commentary controls
- `static/style.css`
  - Style commentary controls

## Task 1: Add failing commentary generation tests

**Files:**
- Modify: `static/commentary.test.mjs`
- Test: `static/commentary.test.mjs`

- [ ] **Step 1: Write the failing tests**
- [ ] **Step 2: Run `node --test static/commentary.test.mjs` and verify the new tests fail**
- [ ] **Step 3: Implement minimal move-style generation in `static/commentary.mjs`**
- [ ] **Step 4: Re-run `node --test static/commentary.test.mjs` and verify those tests pass**

Required cases:

- notation mode renders a non-coordinate move line
- plain mode renders a conversational move line
- hybrid mode renders notation plus explanation
- auto intensity can produce a stronger line for capture/check/repetition rejection

## Task 2: Integrate commentary controls in the UI

**Files:**
- Modify: `static/index.html`
- Modify: `static/app.js`
- Modify: `static/style.css`
- Test: `static/commentary.test.mjs`

- [ ] **Step 1: Add failing assertions or manual test hooks for persisted commentary settings**
- [ ] **Step 2: Run the relevant tests to confirm red state**
- [ ] **Step 3: Add `commentaryMoveStyle` and `commentaryAnalysisIntensity` controls and local-storage wiring**
- [ ] **Step 4: Re-run `node --test static/commentary.test.mjs` and manually verify commentary rendering still works**

## Task 3: Expand commentary handling for rejection / retry events

**Files:**
- Modify: `static/commentary.mjs`
- Modify: `static/commentary.test.mjs`

- [ ] **Step 1: Add failing tests for rejection / retry commentary wording**
- [ ] **Step 2: Run `node --test static/commentary.test.mjs` and verify red state**
- [ ] **Step 3: Implement minimal rejection and retry commentary mapping**
- [ ] **Step 4: Re-run `node --test static/commentary.test.mjs` and verify green state**

## Task 4: Full verification

**Files:**
- Test: `static/commentary.test.mjs`

- [ ] **Step 1: Run `node --test static/commentary.test.mjs static/board-animation.test.mjs static/board-audio.test.mjs`**
- [ ] **Step 2: Confirm zero failures**

