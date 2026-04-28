export function classifyBoardSoundEvent(transitionLike) {
  if (transitionLike?.finished) {
    return "end";
  }
  if (transitionLike?.capture) {
    return "capture";
  }
  return "move";
}

function noOp() {}

function playTone(ctx, frequency, duration, type = "triangle", gainValue = 0.03) {
  const osc = ctx.createOscillator();
  const gain = ctx.createGain();
  osc.type = type;
  osc.frequency.value = frequency;
  gain.gain.setValueAtTime(gainValue, ctx.currentTime);
  gain.gain.exponentialRampToValueAtTime(0.0001, ctx.currentTime + duration);
  osc.connect(gain);
  gain.connect(ctx.destination);
  osc.start();
  osc.stop(ctx.currentTime + duration);
}

export function createBoardAudioController(options = {}) {
  const AudioContextCtor = options.AudioContextCtor ||
    globalThis.AudioContext ||
    globalThis.webkitAudioContext;
  let ctx = null;
  let unlocked = false;

  return {
    isUnlocked() {
      return unlocked;
    },
    async unlock() {
      if (unlocked) {
        return true;
      }
      if (!AudioContextCtor) {
        return false;
      }
      try {
        ctx = ctx || new AudioContextCtor();
        if (typeof ctx.resume === "function") {
          await ctx.resume();
        }
        unlocked = true;
        return true;
      } catch (_err) {
        return false;
      }
    },
    async play(eventName) {
      if (!unlocked || !ctx) {
        return;
      }
      switch (eventName) {
        case "capture":
          playTone(ctx, 280, 0.12, "square", 0.05);
          playTone(ctx, 180, 0.16, "triangle", 0.04);
          break;
        case "end":
          playTone(ctx, 440, 0.1, "triangle", 0.04);
          window.setTimeout(() => playTone(ctx, 660, 0.14, "triangle", 0.04), 90);
          break;
        default:
          playTone(ctx, 320, 0.08, "triangle", 0.035);
          break;
      }
    },
    playNoop: noOp,
  };
}
