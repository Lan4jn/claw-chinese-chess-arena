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
