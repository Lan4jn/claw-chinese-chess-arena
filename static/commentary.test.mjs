import test from "node:test";
import assert from "node:assert/strict";

import { formatCommentaryLog } from "./commentary.mjs";

test("renders notation-style move commentary", () => {
  const view = formatCommentaryLog(
    {
      type: "agent_move",
      side: "black",
      move: "h0-i0",
      notation: "车2平1",
      plain: "黑方把右路的车横到边线。",
      piece: "r",
    },
    { showRawReply: false, moveStyle: "notation", analysisIntensity: "conservative" }
  );

  assert.equal(view.messageText, "黑方 车2平1。 这步先把子力调出来，继续整理阵型。");
});

test("renders plain-style move commentary", () => {
  const view = formatCommentaryLog(
    {
      type: "agent_move",
      side: "red",
      move: "b7-b0",
      notation: "炮八进七",
      plain: "红方让中路的炮一路压上去。",
      piece: "C",
    },
    { showRawReply: false, moveStyle: "plain", analysisIntensity: "conservative" }
  );

  assert.equal(view.messageText, "红方让中路的炮一路压上去。 这步先把子力调出来，继续整理阵型。");
});

test("renders hybrid-style move commentary with stronger auto analysis for capture", () => {
  const view = formatCommentaryLog(
    {
      type: "agent_move",
      side: "black",
      move: "e2-e3",
      notation: "车5进1",
      plain: "黑方把中路的车继续往前压。",
      capture: "P",
      piece: "r",
    },
    { showRawReply: false, moveStyle: "hybrid", analysisIntensity: "auto" }
  );

  assert.equal(view.messageText, "黑方走出车5进1，把中路的车继续往前压。 这步已经形成实质交换，黑方先把场上的子力关系打散了。");
});

test("humanizes rejected move and retry request logs", () => {
  const rejected = formatCommentaryLog(
    {
      type: "agent_move_rejected",
      side: "black",
      move: "e2-e1",
      notation: "车5进1",
      plain: "黑方把中路的车继续往前压。",
      error: "move causes forbidden long-check repetition",
      correction_attempt: 1,
      correction_limit: 3,
    },
    { showRawReply: false, moveStyle: "hybrid", analysisIntensity: "auto" }
  );
  const retrying = formatCommentaryLog(
    {
      type: "agent_retry_requested",
      side: "black",
      mode: "message",
      correction_attempt: 2,
      correction_limit: 3,
    },
    { showRawReply: false, moveStyle: "hybrid", analysisIntensity: "auto" }
  );

  assert.equal(rejected.messageText, "黑方刚才尝试走出车5进1，把中路的车继续往前压，但这步棋没有被裁判允许。 这步棋会形成长将重复，裁判系统已驳回。");
  assert.equal(retrying.messageText, "系统已经通知黑方重新选择下一手，目前进入本回合第 2/3 次尝试。");
});

test("keeps raw reply and raw error when raw mode is enabled", () => {
  const view = formatCommentaryLog(
    {
      type: "agent_move_rejected",
      side: "black",
      message: "选手走子被驳回：e2-e1（第 1/3 次）",
      reply: "MOVE: e2-e1",
      error: "move causes forbidden idle repetition",
    },
    { showRawReply: true, moveStyle: "hybrid", analysisIntensity: "auto" }
  );

  assert.equal(view.messageText, "选手走子被驳回：e2-e1（第 1/3 次）");
  assert.equal(view.replyText, "MOVE: e2-e1");
  assert.equal(view.errorText, "move causes forbidden idle repetition");
});
