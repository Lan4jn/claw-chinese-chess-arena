# Xiangqi Repetition Rules Design

## Goal

Add engine-level repetition constraints that can stop managed agents and human players from repeating long-check, long-chase, or idle-loop sequences forever.

The first release should:

- detect repetition from engine state, not from UI behavior
- apply the same rule enforcement to `human` and `picoclaw`
- reject the offending move as illegal instead of auto-losing the side
- produce explicit error messages that explain which repetition rule was hit

The first release does not try to fully reproduce professional arbiter-grade Chinese chess adjudication. It should instead provide a stable, explainable, engine-level approximation that reliably breaks abusive repetition loops in live matches.

## Problem

The current engine only validates whether a move is structurally legal. It does not track repeated positions or repeated tactical pressure patterns. As a result:

- two sides can enter endless back-and-forth loops
- managed `picoclaw` players can keep forcing repeated chase sequences
- hosts only see repeated invalid game behavior after it has already consumed turns and logs

This gap lives in the rules layer, not in transport or frontend logic.

## Scope

In scope:

- engine-level repeated-position tracking
- engine-level move effect tracing for check, capture, and chase classification
- illegal-move rejection for:
  - long-check repetition
  - long-chase repetition
  - idle repetition
- propagation of explicit repetition errors through match logs
- unit tests for the new rule behaviors

Out of scope:

- full professional Chinese chess adjudication semantics
- automatic loss/win assignment for repetition violations
- frontend-specific repetition controls
- transport-specific handling for different agent types

## Design Summary

The engine should extend `GameState` with rule-evaluation history. Each accepted move records both:

- the normal move history already needed for replay/debugging
- a richer rule trace used to classify repetition behavior

Before a move is committed, the engine should:

1. parse and structurally validate the move
2. simulate the resulting position
3. classify the simulated move effects
4. compare the simulated result against prior rule traces
5. reject the move if it creates forbidden long-check, long-chase, or idle repetition

If the move is accepted, the new rule trace is appended to history.

## Data Model

### Position Key

Add a stable position identity that includes:

- full board layout
- side to move after the simulated move

This key is used to determine whether the game has returned to the same effective position.

The first release does not need a dedicated hash table optimization. A deterministic string key is sufficient.

### Rule Trace

Add a rule-trace record for each accepted move. Each record should include at least:

- moving side
- move string
- resulting `PositionKey`
- whether the move gives check
- whether the move captures
- which opponent targets are under direct chase pressure after the move
- whether the resulting position already existed in prior history

This trace is the common input for all three repetition rules.

## Rule Classification

### Long Check

A move should be rejected as forbidden long-check repetition when all of the following are true:

- the simulated move gives check
- the resulting position repeats a previously seen `PositionKey`
- the same side has already produced repeated checking traces in the same repetition chain
- the chain has not been broken by material progress or a genuinely new position

This intentionally avoids a naive "three checks in a row equals illegal" rule. A side may keep attacking as long as the attack is actually advancing the game instead of recreating the same checking loop.

### Long Chase

The first release should support a bounded, explainable form of long-chase detection.

A move should be rejected as forbidden long-chase repetition when all of the following are true:

- the simulated move does not capture
- the resulting position repeats a previously seen `PositionKey`
- the move continues direct tactical pressure against the same opponent target piece or the same stable target set
- the same side has already maintained that chase pattern across repeated positions

To reduce false positives in the first release:

- `king` / `general` is not treated as a chase target because check is already covered by long-check
- `pawn` targets are excluded from first-release chase classification
- actual captures are treated as material progress and should not be classified as long-chase violations

The implementation goal is to catch the common arena failure mode: one side repeatedly threatening the same non-king target so the opponent can only keep dodging while the overall position loops.

### Idle Repetition

A move should be rejected as forbidden idle repetition when all of the following are true:

- the simulated move does not capture
- the simulated move does not give check
- the simulated move does not qualify as a supported long-chase pattern
- the resulting position repeats a previously seen `PositionKey`
- the recent repetition chain shows no material or tactical progress from either side

Idle repetition is the fallback repetition category. It covers loops that are not active long-check or long-chase but still recreate the same position without advancing the game.

## Enforcement Behavior

When a move hits a repetition rule:

- the move is rejected before it changes the real `GameState`
- the side must choose another move
- the match is not auto-finished
- the violating side is not auto-lost

The engine should return category-specific errors:

- `move causes forbidden long-check repetition`
- `move causes forbidden long-chase repetition`
- `move causes forbidden idle repetition`

These messages should be stable because higher layers will surface them in logs.

## Integration

### Engine Layer

Primary implementation lives in `engine.go`.

Expected additions:

- `PositionKey()` or equivalent helper
- rule-trace struct(s)
- move-effect classification helper(s)
- repetition-rule evaluation helper(s)
- updated `GameState.Apply()` flow that simulates, classifies, checks, then commits

### Match Layer

`match.go` should not re-implement repetition logic.

It should only:

- receive the engine error
- keep treating the move as illegal
- expose the original repetition error text in existing logs

This keeps rule ownership in one place and ensures `human` and `picoclaw` remain consistent.

## Testing

Add engine and match coverage for at least the following:

1. legal attacking lines that do not repeat positions remain valid
2. repeated checking loop is rejected as long-check
3. repeated chase loop is rejected as long-chase
4. repeated non-progress loop is rejected as idle repetition
5. a capture that breaks the loop is not rejected as repetition
6. match logs preserve the repetition error text for both human and agent move paths

Tests should prefer deterministic constructed positions over long organic game sequences. The point is to verify rule classification, not to simulate full realistic matches.

## Risks

### Risk: Long-chase false positives

Long-chase is the most ambiguous category. The first release mitigates this by limiting the supported target classes and requiring repeated positions plus repeated target pressure.

### Risk: Rule logic becomes scattered

Keep repetition ownership inside the engine. Match and transport layers should only consume the resulting errors.

### Risk: Performance regressions

The first release can use straightforward scanning over recent traces. The arena match sizes are small enough for this to be acceptable. If needed later, the position-key lookup can be optimized without changing rule semantics.

## Acceptance Criteria

- repeated long-check loops are blocked
- repeated long-chase loops are blocked
- repeated idle loops are blocked
- blocked moves are reported as illegal, not auto-loss
- the same rule result applies to `human` and `picoclaw`
- repetition failures are visible in existing host-facing logs
