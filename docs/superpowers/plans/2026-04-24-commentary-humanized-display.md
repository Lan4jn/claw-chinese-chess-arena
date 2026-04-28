# Commentary Humanized Display Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Humanize commentary-stage event text when raw replies are hidden, without changing backend log formats.

**Architecture:** Add a small frontend-only formatter module that converts log entries into display-ready commentary text. `static/app.js` will call this formatter when rendering the commentary feed and will preserve raw text only when the checkbox is enabled.

**Tech Stack:** Vanilla browser JavaScript, Node `--test`

---

## File Structure

### Files to modify

- `static/app.js`
  - Use the formatter output instead of directly printing `log.reply` / `log.error`
- `static/index.html`
  - Load the app as an ES module so the formatter can be imported cleanly

### Files to create

- `static/commentary.mjs`
  - Commentary formatting helpers
- `static/commentary.test.mjs`
  - Node tests for commentary formatting

## Task 1: Create formatter tests first

**Files:**
- Create: `static/commentary.test.mjs`
- Test: `static/commentary.test.mjs`

- [ ] **Step 1: Write the failing test**

```javascript
import test from "node:test";
import assert from "node:assert/strict";
import { formatCommentaryLog } from "./commentary.mjs";

test("humanizes move reply when raw mode is disabled", () => {
  const view = formatCommentaryLog(
    {
      side: "red",
      message: "请求选手走子失败（message 模式）",
      reply: "MOVE: a3-a4",
      error: "picoclaw reply did not contain a legal move",
    },
    { showRawReply: false }
  );

  assert.equal(view.replyText, "选手示意将走 a3-a4。");
  assert.equal(view.errorText, "选手已经作答，但给出的着法不在当前允许范围内，本回合未被接受。");
});

test("keeps raw reply and raw error when raw mode is enabled", () => {
  const view = formatCommentaryLog(
    {
      side: "black",
      message: "请求选手走子失败（message 模式）",
      reply: "MOVE: g9-i7",
      error: "move causes forbidden idle repetition",
    },
    { showRawReply: true }
  );

  assert.equal(view.replyText, "MOVE: g9-i7");
  assert.equal(view.errorText, "move causes forbidden idle repetition");
});

test("humanizes repetition error when raw mode is disabled", () => {
  const view = formatCommentaryLog(
    {
      side: "black",
      message: "请求选手走子失败（message 模式）",
      error: "move causes forbidden long-chase repetition",
    },
    { showRawReply: false }
  );

  assert.equal(view.errorText, "这步棋会形成长捉重复，裁判系统已驳回。");
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test static/commentary.test.mjs`
Expected: FAIL with module-not-found or missing export for `formatCommentaryLog`

## Task 2: Implement formatter module

**Files:**
- Create: `static/commentary.mjs`
- Test: `static/commentary.test.mjs`

- [ ] **Step 1: Write minimal implementation**

```javascript
const MOVE_PATTERN = /([a-i][0-9]-[a-i][0-9])/i;

function humanizeReply(reply) {
  const raw = String(reply || "").trim();
  if (!raw) {
    return "";
  }
  const match = raw.match(MOVE_PATTERN);
  if (match) {
    return `选手示意将走 ${match[1].toLowerCase()}。`;
  }
  return "选手已作答。";
}

function humanizeError(error) {
  const raw = String(error || "").trim();
  switch (raw) {
    case "picoclaw reply did not contain a legal move":
      return "选手已经作答，但给出的着法不在当前允许范围内，本回合未被接受。";
    case "move causes forbidden long-check repetition":
      return "这步棋会形成长将重复，裁判系统已驳回。";
    case "move causes forbidden long-chase repetition":
      return "这步棋会形成长捉重复，裁判系统已驳回。";
    case "move causes forbidden idle repetition":
      return "这步棋会形成闲着循环，裁判系统已驳回。";
    default:
      return raw ? "选手响应暂时异常，本回合请求未成功完成。" : "";
  }
}

export function formatCommentaryLog(log, options = {}) {
  const showRawReply = Boolean(options.showRawReply);
  return {
    messageText: String(log?.message || "").trim(),
    replyText: showRawReply ? String(log?.reply || "").trim() : humanizeReply(log?.reply),
    errorText: showRawReply ? String(log?.error || "").trim() : humanizeError(log?.error),
  };
}
```

- [ ] **Step 2: Run test to verify it passes**

Run: `node --test static/commentary.test.mjs`
Expected: PASS

## Task 3: Wire formatter into commentary rendering

**Files:**
- Modify: `static/app.js`
- Modify: `static/index.html`
- Test: `static/commentary.test.mjs`

- [ ] **Step 1: Update app entry to import formatter**

Change `static/index.html`:

```html
<script type="module" src="/static/app.js"></script>
```

- [ ] **Step 2: Use formatter in `renderEvents()`**

Update `static/app.js` to:

```javascript
import { formatCommentaryLog } from "./commentary.mjs";
```

And replace the direct `reply` / `error` rendering in `renderEvents()` with formatter output driven by the checkbox state.

- [ ] **Step 3: Run formatter tests again**

Run: `node --test static/commentary.test.mjs`
Expected: PASS

- [ ] **Step 4: Run Go test suite to catch embedded-static regressions**

Run: `GOTOOLCHAIN=local go test ./...`
Expected: PASS
