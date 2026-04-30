# Managed Agent Invalid Move Correction Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep managed agent matches running after rule-violating moves by sending a correction notice and re-requesting a move in the same turn, across message, session, and pico_ws modes.

**Architecture:** Move invalid-move retry ownership into arena turn orchestration. Transport functions should perform a single request/response exchange while arena coordinates rule validation, correction prompting, logging, and a shared 3-attempt limit.

**Tech Stack:** Go 1.20, existing arena/match/picoclaw transport code, `go test`

---

## File Structure

### Files to modify

- `arena.go`
  - Add same-turn correction loop and shared retry classification
- `match.go`
  - Add rejection/retry logging helpers
- `pico.go`
  - Add correction prompt support for message mode
- `pico_ws.go`
  - Add correction prompt support for ws mode
- `picoclaw_runtime.go`
  - Extend session turn payload if needed
- `arena_test.go`
  - Add coverage for message/session/ws correction loops

## Task 1: Add red tests for message-mode correction retry

**Files:**
- Modify: `arena_test.go`
- Test: `arena_test.go`

- [ ] **Step 1: Write failing tests for “first move rejected, second move accepted” in message mode**
- [ ] **Step 2: Run `GOTOOLCHAIN=local go test ./... -run 'TestArenaAdvanceOnceRetriesCorrectableMessageMove' -v` and verify failure**
- [ ] **Step 3: Implement minimal arena-level retry loop for non-managed message requests**
- [ ] **Step 4: Re-run the targeted test and verify pass**

## Task 2: Extend correction retry to session and pico_ws

**Files:**
- Modify: `arena_test.go`
- Modify: `arena.go`
- Modify: `pico_ws.go`
- Modify: `picoclaw_runtime.go`

- [ ] **Step 1: Add failing tests for session and pico_ws correction retries**
- [ ] **Step 2: Run targeted tests and verify red state**
- [ ] **Step 3: Implement minimal transport hooks so arena can send correction prompts through all three modes**
- [ ] **Step 4: Re-run targeted tests and verify green state**

## Task 3: Enforce 3-attempt limit and preserve failure separation

**Files:**
- Modify: `arena_test.go`
- Modify: `arena.go`
- Modify: `match.go`

- [ ] **Step 1: Add failing tests for “three rejected moves pauses the match” and “network error does not enter correction retry”**
- [ ] **Step 2: Run targeted tests to verify failure**
- [ ] **Step 3: Implement shared correctable-vs-terminal classification and 3-attempt ceiling**
- [ ] **Step 4: Re-run targeted tests and verify pass**

## Task 4: Logging and commentary-facing event semantics

**Files:**
- Modify: `match.go`
- Modify: `arena_test.go`

- [ ] **Step 1: Add failing tests for rejection and retry log entries**
- [ ] **Step 2: Run targeted tests and verify red state**
- [ ] **Step 3: Implement minimal match log helpers for “rejected move” and “retry requested”**
- [ ] **Step 4: Re-run targeted tests and verify green state**

## Task 5: Full verification

**Files:**
- Test: `arena_test.go`
- Test: full Go suite

- [ ] **Step 1: Run `GOTOOLCHAIN=local go test ./...`**
- [ ] **Step 2: Confirm zero failures**
