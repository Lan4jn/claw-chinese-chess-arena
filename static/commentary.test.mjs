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

  assert.equal(view.messageText, "红方正在请求选手应对当前局面，但这一回合没有形成可被系统接受的着法。");
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
