# Board-First Layout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the full xiangqi board visible on first screen by default, with commentary and room info yielding space when the viewport is constrained.

**Architecture:** Keep this CSS-first. Adjust the main grid, stage sizing, and responsive breakpoints so the board stage owns viewport priority. Avoid backend or API changes, and keep DOM changes minimal unless a wrapper is required for cleaner CSS behavior.

**Tech Stack:** Static HTML, CSS, small existing frontend JS, Go asset serving, manual browser verification, `go test`, `go build`

---

## File Structure

- Modify: `static/style.css`
  - Owns the board-first grid behavior, stage sizing, and height-aware responsive rules.
- Modify: `static/index.html`
  - Only if a small structural wrapper is needed for cleaner CSS targeting.
- Modify: `README.md`
  - Document the board-first layout behavior if the UI behavior materially changes.
- Verify: `http_test.go`
  - Reuse existing static asset coverage only if a minimal assertion is needed after DOM or asset changes.

### Task 1: Review and Lock the Layout Surface

**Files:**
- Modify: `static/style.css`
- Optional: `static/index.html`

- [ ] **Step 1: Identify the current layout selectors that control board competition**

Read:

```bash
sed -n '1,260p' static/style.css
sed -n '260,520p' static/style.css
sed -n '1,120p' static/index.html
```

Expected: locate `.arena-layout`, `.stage-column`, `.info-column`, `.board-stage`, `.event-stage`, `.board-wrap`, `.board-frame`, `.event-list`.

- [ ] **Step 2: Confirm no DOM change is required before editing CSS**

Rule:

```text
If `.arena-layout`, `.stage-column`, `.info-column`, `.board-stage`, and `.event-stage` are already independently targetable, keep `static/index.html` unchanged.
```

Expected: likely no HTML change needed.

### Task 2: Write a Minimal Failing Verification Target

**Files:**
- Modify: `README.md` only if needed later

- [ ] **Step 1: Define the concrete viewport targets before editing CSS**

Use these viewport checks as the failing target:

```text
1440x900
1366x768
1280x720
390x844
```

Failure condition:

```text
Any desktop-sized viewport above shows the board partially below the fold on initial load in board view.
```

- [ ] **Step 2: Verify the current layout against that target**

Run after starting the app if needed:

```bash
GOTOOLCHAIN=local go run .
```

Expected before changes: at least one constrained desktop-height viewport still gives too much space to commentary/info and reduces first-screen board visibility.

### Task 3: Implement Board-First CSS

**Files:**
- Modify: `static/style.css`

- [ ] **Step 1: Change the main grid to let the board column dominate**

Implement changes in `static/style.css` around `.arena-layout`, `.stage-column`, `.info-column`.

Target shape:

```css
.arena-layout {
  display: grid;
  grid-template-columns: minmax(0, 1.45fr) minmax(280px, 0.75fr);
  align-items: start;
}

.stage-column,
.info-column {
  align-content: start;
}
```

- [ ] **Step 2: Make the board stage size from viewport availability instead of row fractions**

Implement changes around `.board-stage`, `.board-wrap`, `.board-frame`.

Target shape:

```css
.board-stage {
  display: flex;
  flex-direction: column;
}

.board-wrap {
  flex: 1 1 auto;
}

.board-frame {
  width: min(100%, calc((100vh - 260px) * 0.9));
  max-width: 100%;
  margin: 0 auto;
}
```

Note: exact numbers may change during verification, but board height must be derived from viewport budget, not from the event stage.

- [ ] **Step 3: Bound the commentary panel so it cannot push the board below the fold**

Implement changes around `.event-stage` and `.event-list`.

Target shape:

```css
.event-stage {
  align-self: start;
}

.event-list {
  max-height: min(32vh, 320px);
  overflow: auto;
}
```

- [ ] **Step 4: Add an earlier stacked breakpoint for width and a dedicated height breakpoint**

Implement responsive rules that move `.info-column` below the board when:

```css
@media (max-width: 1180px) { ... }
@media (max-height: 820px) { ... }
```

Expected behavior:

```css
.arena-layout {
  grid-template-columns: 1fr;
}
```

And:

```css
.info-column {
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
}
```

- [ ] **Step 5: Keep commentary mode from re-breaking first-screen board visibility**

Adjust the `.app-shell.mode-commentary .stage-column` rules so commentary view can reorder sections without making the board share rigid height fractions.

Target constraint:

```text
Commentary mode may reorder the event stage, but the board must remain fully visible on common desktop/laptop first view.
```

### Task 4: Verify the Layout

**Files:**
- Modify: `static/style.css`
- Optional: `static/index.html`

- [ ] **Step 1: Run formatting-safe verification for the Go app**

Run:

```bash
GOTOOLCHAIN=local go test -count=1 ./...
```

Expected: PASS

- [ ] **Step 2: Run build verification**

Run:

```bash
GOTOOLCHAIN=local go build ./...
```

Expected: PASS

- [ ] **Step 3: Manually verify the four target viewports**

Check:

```text
1440x900
1366x768
1280x720
390x844
```

Expected:

```text
Desktop/laptop viewports show the full board on initial load in board view.
Info/commentary remain reachable without forcing the board below the fold.
```

- [ ] **Step 4: Update docs only if the user-visible behavior needs explanation**

If needed, add a short README note describing the board-first responsive behavior.

### Task 5: Final Verification Snapshot

**Files:**
- Modify: none unless verification reveals issues

- [ ] **Step 1: Re-run the full verification commands after any final adjustment**

Run:

```bash
GOTOOLCHAIN=local go test -count=1 ./...
GOTOOLCHAIN=local go build ./...
```

Expected: both PASS
