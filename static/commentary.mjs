const MOVE_PATTERN = /([a-i][0-9]-[a-i][0-9])/i;

function normalizeText(value) {
  return String(value || "").trim();
}

function sideLabel(side) {
  return side === "red" ? "红方" : side === "black" ? "黑方" : "系统";
}

function extractMoveText(text) {
  const match = normalizeText(text).match(MOVE_PATTERN);
  return match ? match[1].toLowerCase() : "";
}

function normalizeMoveStyle(value) {
  return value === "notation" || value === "plain" || value === "hybrid" ? value : "hybrid";
}

function normalizeIntensity(value) {
  return value === "conservative" || value === "balanced" || value === "aggressive" || value === "auto"
    ? value
    : "auto";
}

function moveLead(log, moveStyle) {
  const side = sideLabel(log?.side);
  const notation = normalizeText(log?.notation);
  const plain = normalizeText(log?.plain);
  switch (moveStyle) {
    case "notation":
      return notation ? `${side} ${notation}。` : normalizeText(log?.message);
    case "plain":
      return plain || normalizeText(log?.message);
    default:
      if (notation && plain) {
        return `${side}走出${notation}，${plain.replace(new RegExp(`^${side}`), "").replace(/^把/, "把")}`;
      }
      if (notation) {
        return `${side}走出${notation}。`;
      }
      return plain || normalizeText(log?.message);
  }
}

function rejectedMoveLead(log, moveStyle) {
  const side = sideLabel(log?.side);
  const notation = normalizeText(log?.notation);
  const plain = normalizeText(log?.plain).replace(/[。！!？?]$/, "");
  if (moveStyle === "notation" && notation) {
    return `${side}刚才尝试走出${notation}，但这步棋没有被裁判允许。`;
  }
  if (moveStyle === "plain" && plain) {
    return `${plain}，但这步棋没有被裁判允许。`;
  }
  if (notation && plain) {
    return `${side}刚才尝试走出${notation}，${plain.replace(new RegExp(`^${side}`), "").replace(/^把/, "把")}，但这步棋没有被裁判允许。`;
  }
  return `${side}刚才提交了一步没有被裁判允许的着法。`;
}

function autoIntensity(log) {
  if (normalizeText(log?.error) || normalizeText(log?.capture) || log?.gives_check) {
    return "aggressive";
  }
  if (normalizeText(log?.piece)) {
    return "balanced";
  }
  return "conservative";
}

function buildMoveAnalysis(log, intensity) {
  const resolved = intensity === "auto" ? autoIntensity(log) : intensity;
  if (resolved === "conservative") {
    return "这步先把子力调出来，继续整理阵型。";
  }
  if (resolved === "balanced") {
    return "这步在保持出子节奏的同时，也是在继续试探对手的正面反应。";
  }
  if (normalizeText(log?.capture)) {
    return "这步已经形成实质交换，黑方先把场上的子力关系打散了。".replace("黑方", sideLabel(log?.side));
  }
  if (log?.gives_check) {
    return `${sideLabel(log?.side)}这一将给得很直接，对手必须先处理眼前压力。`;
  }
  return `${sideLabel(log?.side)}这步已经把主动手抢出来了，后续如果继续跟上，局面压力会明显增加。`;
}

function humanizeError(error) {
  const raw = normalizeText(error);
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

function fallbackHumanizedMessage(log) {
  const raw = normalizeText(log?.message);
  const side = sideLabel(log?.side);
  if (!raw) {
    return "";
  }
  if (raw.startsWith("请求选手走子失败")) {
    return `${side}正在请求选手应对当前局面，但这一回合没有形成可被系统接受的着法。`;
  }
  if (raw.startsWith("选手返回非法走法：")) {
    const move = extractMoveText(raw);
    return move ? `${side}提交了 ${move}，但这步棋未被系统接受。` : `${side}提交了一步系统未接受的着法。`;
  }
  if (raw.startsWith("选手走子：")) {
    const move = extractMoveText(raw);
    return move ? `${side}已经落子 ${move}。` : `${side}已经完成本回合落子。`;
  }
  if (raw.startsWith("手动走子：")) {
    const move = extractMoveText(raw);
    return move ? `${side}手动走出 ${move}。` : `${side}手动完成了一步走子。`;
  }
  if (raw.startsWith("手动走子失败")) {
    return `${side}尝试手动走子，但这一步未被系统接受。`;
  }
  if (raw.startsWith("走子模式切换：")) {
    return `${side}的托管通道已经发生切换，系统正在尝试新的联络方式。`;
  }
  return raw;
}

export function formatCommentaryLog(log, options = {}) {
  const showRawReply = Boolean(options.showRawReply);
  const moveStyle = normalizeMoveStyle(options.moveStyle);
  const analysisIntensity = normalizeIntensity(options.analysisIntensity);

  if (showRawReply) {
    return {
      messageText: normalizeText(log?.message),
      replyText: normalizeText(log?.reply),
      errorText: normalizeText(log?.error),
    };
  }

  switch (normalizeText(log?.type)) {
    case "agent_move":
    case "human_move": {
      return {
        messageText: `${moveLead(log, moveStyle)} ${buildMoveAnalysis(log, analysisIntensity)}`.trim(),
        replyText: "",
        errorText: "",
      };
    }
    case "agent_move_rejected": {
      return {
        messageText: `${rejectedMoveLead(log, moveStyle)} ${humanizeError(log?.error)}`.trim(),
        replyText: "",
        errorText: "",
      };
    }
    case "agent_retry_requested": {
      const attempt = Number(log?.correction_attempt || 0);
      const limit = Number(log?.correction_limit || 0);
      return {
        messageText: `系统已经通知${sideLabel(log?.side)}重新选择下一手，目前进入本回合第 ${attempt}/${limit} 次尝试。`,
        replyText: "",
        errorText: "",
      };
    }
    case "agent_retry_exhausted": {
      const limit = Number(log?.correction_limit || 0);
      return {
        messageText: `${sideLabel(log?.side)}连续${limit}次提交违规着法，这一回合未能形成有效落子。`,
        replyText: "",
        errorText: "",
      };
    }
    default:
      return {
        messageText: fallbackHumanizedMessage(log),
        replyText: normalizeText(log?.reply) ? "选手已经作出回应。" : "",
        errorText: humanizeError(log?.error),
      };
  }
}
