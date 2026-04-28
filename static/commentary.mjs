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

function humanizeMessage(log) {
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

function humanizeReply(reply) {
  const raw = normalizeText(reply);
  if (!raw) {
    return "";
  }
  const move = extractMoveText(raw);
  if (move) {
    return `选手示意将走 ${move}。`;
  }
  return "选手已经作出回应。";
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

export function formatCommentaryLog(log, options = {}) {
  const showRawReply = Boolean(options.showRawReply);
  return {
    messageText: showRawReply ? normalizeText(log?.message) : humanizeMessage(log),
    replyText: showRawReply ? normalizeText(log?.reply) : humanizeReply(log?.reply),
    errorText: showRawReply ? normalizeText(log?.error) : humanizeError(log?.error),
  };
}
