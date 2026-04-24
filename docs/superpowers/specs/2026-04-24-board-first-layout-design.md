# Board-First Layout Design

## Context

The current arena shell places the board stage, event stage, and right-side information column in a desktop-first two-column layout. In practice, the board can lose too much height when:

- the viewport height is limited
- the event stage keeps a large minimum share
- the right column stays beside the board instead of yielding space

The user requirement is explicit:

- the full xiangqi board should be visible on the first screen by default
- this priority is more important than keeping commentary and room information beside the board
- if space is tight, information panels may compress or move below the board

This is a layout priority change, not a data-flow change.

## Goal

Adjust the frontend layout so that:

- the full board is visible within the initial viewport on typical desktop and laptop browsers
- the board keeps its current `9 / 10` aspect ratio
- commentary and info panels stop competing with board height
- when width or height is constrained, side panels move below the board instead of shrinking the board out of view

## Non-Goals

This design does not include:

- changing the board rendering logic
- changing chess rules or move interaction
- redesigning the host drawer
- adding new views
- changing the visual theme

## Current Layout Constraints

The current layout in `static/index.html` and `static/style.css` has three main issues:

1. `main.arena-layout` reserves a persistent right column on wider screens.
2. `.stage-column` keeps both board stage and event stage in the same vertical stack with strong competition for height.
3. `.event-list` can remain visually large enough that the main board no longer dominates the viewport.

The result is that the board can become smaller than the user expects, especially on shorter screens.

## Design Principle

The board is the primary product surface.

Layout decisions should therefore follow this order:

1. keep the full board visible
2. keep essential turn/context hints visible near the board
3. let commentary and room metadata scroll or move lower

This reverses the current priority where supporting panels can consume space needed by the board.

## Target Layout Model

### Desktop and Large Laptop

Use a board-first main column:

- the board stage remains first
- the board stage gets the dominant share of available height
- the event stage is visually secondary and should accept a smaller bounded height
- the info column may remain on the right only when there is enough width and enough height

Recommended behavior:

- if both width and height are comfortable, keep a two-column layout
- in that state, the board column must still be sized from viewport availability, not from the content needs of the event stage

### Height-Constrained Desktop / Laptop

When viewport height is limited, switch aggressively to a board-first stacked layout:

- board stage first
- info column below board
- event stage below info or below board depending on the active view mode

The key rule is:

- the board should not lose first-screen visibility just to preserve a side-by-side info layout

### Mobile and Narrow Width

Use a single-column stack:

- board stage
- board footer
- seat and room info cards
- event stage

This is already directionally close to the existing responsive behavior, but the breakpoint should trigger earlier and more decisively.

## Layout Rules

### Main Container

`main.arena-layout` should support two stable modes:

- `board-first-two-column`
- `board-first-single-column`

Selection should depend on both width and height, not width alone.

### Board Stage

The board stage should:

- be the first visible stage in both `board` and `commentary` view modes
- avoid sharing fixed row fractions with the event stage on constrained heights
- size around the board content first

`board-wrap` and `board-frame` should be allowed to consume the largest safe share of viewport height after header and outer spacing are removed.

### Event Stage

The event stage should become a bounded secondary panel:

- smaller default height
- explicit internal scrolling
- no ability to force the board below the fold

The event stage should not claim a large fraction of viewport height when the board is already close to overflow.

### Info Column

The info column should yield early.

Expected behavior:

- remain beside the board only when the viewport can support the board at a comfortable visible size
- otherwise move below the board as a normal stacked section

Seat cards and room stats may become more compact, but they should not hide essential identity or turn information.

## View Mode Semantics

The existing `board` and `commentary` view toggle remains.

The layout change should not remove that distinction, but it should constrain it:

- `board` mode keeps the board as the dominant section
- `commentary` mode may place the event stage earlier in document flow, but must still preserve full first-screen board visibility whenever practical

That means `commentary` mode can reorder sections, but cannot revert to a layout where the board becomes partially hidden on initial load.

## CSS Strategy

The implementation should stay CSS-first.

Preferred changes:

- update `.arena-layout` grid behavior
- update `.stage-column` row sizing
- constrain `.event-stage` and `.event-list`
- add height-aware breakpoints using viewport height media queries
- move `.info-column` below the board at a more aggressive breakpoint than the current layout

Avoid JavaScript-driven layout measurement unless CSS alone cannot achieve the requirement.

## Testing and Verification

There is no frontend browser test harness in this repo, so verification should be practical and focused:

- confirm the board still renders correctly
- confirm the board aspect ratio remains `9 / 10`
- confirm the first viewport shows the full board on common laptop-ish dimensions
- confirm commentary and info panels remain reachable and scrollable
- confirm host drawer behavior is unchanged

Recommended manual viewport checks:

- desktop around `1440x900`
- laptop around `1366x768`
- smaller laptop around `1280x720`
- narrow mobile width

## Files in Scope

- `static/style.css`
- optionally `static/index.html` if a light structural wrapper is needed

JavaScript changes in `static/app.js` should be avoided unless the current view-mode class handling must be adjusted to support the new layout cleanly.

## Acceptance Criteria

This change is successful when:

- the full board is visible on initial load in the board-focused view on common desktop and laptop sizes
- commentary and room info no longer force the board below the fold
- the right column moves below the board on constrained layouts
- no game interaction behavior changes
- no API or backend behavior changes
