"use strict";

const POLL_INTERVAL_MS = 2000;

const STORAGE_KEYS = {
  clientToken: "arena.clientToken",
  roomCode: "arena.roomCode",
  displayName: "arena.displayName",
  joinIntent: "arena.joinIntent",
  currentView: "arena.currentView",
};

const state = {
  clientToken: "",
  roomCode: "",
  displayName: "",
  joinIntent: "spectator",
  currentView: "board",
  isHost: false,
  participant: null,
  publicRoom: null,
  publicMatch: null,
  hostRoom: null,
  hostMatch: null,
  selectedFrom: "",
  pollTimer: null,
  lastError: "",
  refreshInFlight: false,
  joinInFlight: false,
};

const dom = {
  appShell: null,
  joinScreen: null,
  joinNote: null,
  roomCodeInput: null,
  displayNameInput: null,
  joinIntentSelect: null,
  joinRoomButton: null,
  randomRoomButton: null,
  viewButtons: [],
  roomCodeBadge: null,
  roomStatusBadge: null,
  intervalBadge: null,
  revealBadge: null,
  defaultViewValue: null,
  mySeat: null,
  myAlias: null,
  spectatorCount: null,
  seatRedCard: null,
  seatBlackCard: null,
  stageTitle: null,
  turnPill: null,
  boardGrid: null,
  selectionHint: null,
  eventList: null,
  participantList: null,
  clearSelectionButton: null,
};

function loadStoredValue(key, fallback) {
  try {
    const value = window.localStorage.getItem(key);
    return value === null ? fallback : value;
  } catch (_err) {
    return fallback;
  }
}

function saveStoredValue(key, value) {
  try {
    window.localStorage.setItem(key, value);
  } catch (_err) {
    // Ignore localStorage write failures in minimal shell mode.
  }
}

function generateClientToken() {
  if (window.crypto && typeof window.crypto.randomUUID === "function") {
    return window.crypto.randomUUID();
  }
  return "client-" + Date.now().toString(36);
}

function setJoinNote(message, isError) {
  if (!dom.joinNote) {
    return;
  }
  dom.joinNote.textContent = message || "";
  dom.joinNote.classList.toggle("is-error", Boolean(isError));
}

function setJoinBusy(isBusy) {
  state.joinInFlight = Boolean(isBusy);
  if (!dom.joinRoomButton) {
    return;
  }
  dom.joinRoomButton.disabled = isBusy;
  dom.joinRoomButton.textContent = isBusy ? "进入中..." : "进入比赛";
}

function escapeHTML(value) {
  return String(value || "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

function seatLabel(seat) {
  switch (seat) {
    case "host":
      return "主持人";
    case "red_player":
      return "红方选手";
    case "black_player":
      return "黑方选手";
    case "spectator":
      return "观众";
    default:
      return "未入场";
  }
}

function roomStatusLabel(status) {
  switch (status) {
    case "playing":
      return "比赛进行中";
    case "paused":
      return "比赛已暂停";
    case "finished":
      return "比赛已结束";
    default:
      return "等待开始";
  }
}

function revealLabel(value) {
  switch (value) {
    case "full_reveal":
      return "身份：全部揭晓";
    case "partial_reveal":
      return "身份：部分揭晓";
    default:
      return "身份：隐藏";
  }
}

function sideLabel(side) {
  return side === "red" ? "红方" : side === "black" ? "黑方" : "未开始";
}

function applyViewMode(nextView) {
  const normalized = nextView === "commentary" ? "commentary" : "board";
  state.currentView = normalized;

  if (dom.appShell) {
    dom.appShell.classList.toggle("mode-board", normalized === "board");
    dom.appShell.classList.toggle("mode-commentary", normalized === "commentary");
  }

  dom.viewButtons.forEach((button) => {
    button.classList.toggle("is-active", button.dataset.view === normalized);
  });

  if (dom.defaultViewValue) {
    dom.defaultViewValue.textContent = normalized === "commentary" ? "解说模式" : "棋局中心";
  }

  saveStoredValue(STORAGE_KEYS.currentView, normalized);
}

function updateHeaderState() {
  if (dom.roomCodeBadge) {
    dom.roomCodeBadge.textContent = state.roomCode ? "比赛码：" + state.roomCode : "比赛码：未进入";
  }

  const publicRoom = state.publicRoom;
  if (dom.roomStatusBadge) {
    if (!state.roomCode) {
      dom.roomStatusBadge.textContent = "等待加入";
    } else if (state.isHost) {
      dom.roomStatusBadge.textContent = "主持人视角 · " + roomStatusLabel(publicRoom ? publicRoom.status : "");
    } else {
      dom.roomStatusBadge.textContent = roomStatusLabel(publicRoom ? publicRoom.status : "");
    }
  }
  if (dom.intervalBadge) {
    const interval = publicRoom && publicRoom.step_interval_ms > 0 ? publicRoom.step_interval_ms + "ms" : "-";
    dom.intervalBadge.textContent = "步间隔：" + interval;
  }
  if (dom.revealBadge) {
    dom.revealBadge.textContent = revealLabel(publicRoom ? publicRoom.reveal_state : "");
  }
}

async function apiRequest(path, options = {}) {
  const requestOptions = { ...options };
  const headers = { ...(requestOptions.headers || {}) };

  if (requestOptions.body && typeof requestOptions.body !== "string") {
    requestOptions.body = JSON.stringify(requestOptions.body);
    if (!headers["Content-Type"]) {
      headers["Content-Type"] = "application/json";
    }
  }

  requestOptions.headers = headers;

  const response = await fetch(path, requestOptions);
  const contentType = response.headers.get("Content-Type") || "";
  const payload = contentType.includes("application/json")
    ? await response.json()
    : await response.text();

  if (!response.ok) {
    const message =
      typeof payload === "object" && payload && payload.error
        ? payload.error
        : "Request failed with status " + response.status;
    state.lastError = message;
    const error = new Error(message);
    error.status = response.status;
    error.payload = payload;
    throw error;
  }

  state.lastError = "";
  return payload;
}

async function fetchPublicRoom() {
  const path = "/api/arena/" + encodeURIComponent(state.roomCode);
  return apiRequest(path);
}

async function fetchPublicMatch() {
  const path = "/api/arena/" + encodeURIComponent(state.roomCode) + "/match";
  return apiRequest(path);
}

async function fetchHostRoom() {
  const path =
    "/api/arena/" +
    encodeURIComponent(state.roomCode) +
    "/host?token=" +
    encodeURIComponent(state.clientToken);
  return apiRequest(path);
}

async function fetchHostMatch() {
  const path =
    "/api/arena/" +
    encodeURIComponent(state.roomCode) +
    "/host/match?token=" +
    encodeURIComponent(state.clientToken);
  return apiRequest(path);
}

async function enterRoom(payload) {
  const response = await apiRequest("/api/arena/enter", {
    method: "POST",
    body: payload,
  });

  state.roomCode = payload.room_code;
  state.clientToken = payload.client_token;
  state.displayName = payload.display_name || "";
  state.joinIntent = payload.join_intent || "spectator";
  state.isHost = Boolean(response.is_host);
  state.participant = response.participant || null;
  state.publicRoom = response.public_room || null;
  state.publicMatch = response.public_match || null;
  state.hostRoom = response.host_room || null;
  state.hostMatch = response.host_match || null;

  saveStoredValue(STORAGE_KEYS.clientToken, state.clientToken);
  saveStoredValue(STORAGE_KEYS.roomCode, state.roomCode);
  saveStoredValue(STORAGE_KEYS.displayName, state.displayName);
  saveStoredValue(STORAGE_KEYS.joinIntent, state.joinIntent);

  if (dom.joinScreen) {
    dom.joinScreen.classList.add("is-hidden");
  }
  updateHeaderState();
  setJoinNote("");

  return response;
}

function renderSeatCard(element, seatType, seatInfo) {
  if (!element) {
    return;
  }
  const occupied = seatInfo && seatInfo.participant_id;
  const badgeClass = seatType === "red_player" ? "red" : "black";
  const alias = occupied ? seatInfo.public_alias || "未命名" : "等待选手";
  const detail = occupied
    ? "席位已占用"
    : "暂无选手，主持人可在设置中配置";
  const realType = seatInfo && seatInfo.real_type ? seatInfo.real_type : "hidden";

  element.innerHTML =
    '<div class="seat-head">' +
    '<div><p class="label">' +
    (seatType === "red_player" ? "RED SIDE" : "BLACK SIDE") +
    '</p><h3 class="seat-name">' +
    escapeHTML(alias) +
    "</h3></div>" +
    '<span class="seat-badge ' +
    badgeClass +
    '">' +
    escapeHTML(occupied ? "已就位" : "空席") +
    "</span></div>" +
    '<div class="seat-meta">' +
    '<p class="seat-detail">席位：' +
    escapeHTML(seatLabel(seatType)) +
    "</p>" +
    '<p class="seat-detail">状态：' +
    escapeHTML(detail) +
    "</p>" +
    '<p class="seat-detail">身份：' +
    escapeHTML(realType) +
    "</p></div>";
}

function renderBoard() {
  if (!dom.boardGrid) {
    return;
  }
  dom.boardGrid.innerHTML = "";

  const boardRows =
    state.publicMatch && Array.isArray(state.publicMatch.board_rows)
      ? state.publicMatch.board_rows
      : Array(10).fill(".........");

  for (let row = 0; row < 10; row += 1) {
    const rowText = boardRows[row] || ".........";
    for (let col = 0; col < 9; col += 1) {
      const piece = rowText[col] || ".";
      const cell = document.createElement("div");
      cell.className = "board-cell";
      cell.dataset.row = String(row);
      cell.dataset.col = String(col);

      if (piece !== ".") {
        const chip = document.createElement("span");
        const isRed = piece === piece.toUpperCase();
        chip.className = "piece-chip " + (isRed ? "red" : "black");
        chip.textContent = piece;
        cell.appendChild(chip);
      }
      dom.boardGrid.appendChild(cell);
    }
  }
}

function renderEvents() {
  if (!dom.eventList) {
    return;
  }
  const logs = state.publicMatch && Array.isArray(state.publicMatch.logs) ? state.publicMatch.logs : [];
  if (logs.length === 0) {
    dom.eventList.innerHTML =
      '<article class="event-item"><p class="event-message">暂无比赛日志。加入房间后等待主持人开始比赛。</p></article>';
    return;
  }

  dom.eventList.innerHTML = logs
    .slice()
    .reverse()
    .slice(0, 40)
    .map((log) => {
      const at = log.time ? new Date(log.time).toLocaleTimeString() : "--:--:--";
      const side = log.side ? sideLabel(log.side) : "系统";
      const reply = log.reply ? '<p class="event-reply">' + escapeHTML(log.reply) + "</p>" : "";
      const error = log.error ? '<p class="event-error">' + escapeHTML(log.error) + "</p>" : "";
      return (
        '<article class="event-item"><header><strong>' +
        escapeHTML(side) +
        "</strong><span>" +
        escapeHTML(at) +
        '</span></header><p class="event-message">' +
        escapeHTML(log.message || "") +
        "</p>" +
        reply +
        error +
        "</article>"
      );
    })
    .join("");
}

function renderParticipants() {
  if (!dom.participantList) {
    return;
  }
  if (!state.isHost || !state.hostRoom || !Array.isArray(state.hostRoom.participants)) {
    dom.participantList.innerHTML =
      '<article class="participant-item"><p class="participant-meta">仅主持人可查看完整成员信息。</p></article>';
    return;
  }

  dom.participantList.innerHTML = state.hostRoom.participants
    .map((participant) => {
      return (
        '<article class="participant-item"><header><strong>' +
        escapeHTML(participant.public_alias || participant.id) +
        "</strong><span>" +
        escapeHTML(seatLabel(participant.seat)) +
        '</span></header><p class="participant-meta">连接：' +
        escapeHTML(participant.connection || "ui") +
        " · 类型：" +
        escapeHTML(participant.real_type || "human") +
        "</p></article>"
      );
    })
    .join("");
}

function renderSummary() {
  const room = state.publicRoom;
  const match = state.publicMatch;
  const participant = state.participant;

  if (dom.mySeat) {
    dom.mySeat.textContent = participant ? seatLabel(participant.seat) : "未入场";
  }
  if (dom.myAlias) {
    dom.myAlias.textContent = participant && participant.public_alias ? participant.public_alias : "-";
  }
  if (dom.spectatorCount) {
    dom.spectatorCount.textContent =
      room && typeof room.spectator_count === "number" ? String(room.spectator_count) : "0";
  }

  if (dom.stageTitle) {
    dom.stageTitle.textContent = room ? roomStatusLabel(room.status) : "等待创建比赛";
  }
  if (dom.turnPill) {
    dom.turnPill.classList.remove("is-red", "is-black");
    if (match && match.status === "playing") {
      dom.turnPill.textContent = sideLabel(match.turn) + "行动";
      dom.turnPill.classList.add(match.turn === "red" ? "is-red" : "is-black");
    } else if (match && match.status === "finished") {
      dom.turnPill.textContent = "已结束";
    } else {
      dom.turnPill.textContent = "未开始";
    }
  }
  if (dom.selectionHint) {
    if (!match) {
      dom.selectionHint.textContent = "等待主持人开始比赛。";
    } else if (match.status === "playing") {
      dom.selectionHint.textContent = "当前行动方：" + sideLabel(match.turn);
    } else {
      dom.selectionHint.textContent = "比赛状态：" + (match.status || "waiting");
    }
  }

  const seats = room && room.seats ? room.seats : {};
  renderSeatCard(dom.seatRedCard, "red_player", seats.red_player);
  renderSeatCard(dom.seatBlackCard, "black_player", seats.black_player);
  renderBoard();
  renderEvents();
  renderParticipants();
  updateHeaderState();
}

async function refreshAll() {
  if (!state.roomCode || state.refreshInFlight) {
    return;
  }
  state.refreshInFlight = true;
  try {
    state.publicRoom = await fetchPublicRoom();

    try {
      state.publicMatch = await fetchPublicMatch();
    } catch (err) {
      if (err && err.status === 404) {
        state.publicMatch = null;
      } else {
        throw err;
      }
    }

    if (state.isHost) {
      try {
        state.hostRoom = await fetchHostRoom();
      } catch (err) {
        if (err && err.status === 403) {
          state.isHost = false;
        }
        state.hostRoom = null;
        state.hostMatch = null;
      }

      if (state.isHost && state.hostRoom) {
        try {
          state.hostMatch = await fetchHostMatch();
        } catch (err) {
          if (err && err.status === 403) {
            state.isHost = false;
            state.hostRoom = null;
            state.hostMatch = null;
          } else if (err && err.status === 404) {
            state.hostMatch = null;
          } else {
            state.hostMatch = null;
          }
        }
      } else {
        state.hostMatch = null;
      }
    } else {
      state.hostRoom = null;
      state.hostMatch = null;
    }

    renderSummary();
  } catch (err) {
    const message = err instanceof Error ? err.message : "刷新比赛状态失败";
    setJoinNote(message, true);
  } finally {
    state.refreshInFlight = false;
  }
}

function startPolling() {
  if (state.pollTimer) {
    window.clearInterval(state.pollTimer);
  }
  state.pollTimer = window.setInterval(() => {
    void refreshAll();
  }, POLL_INTERVAL_MS);
}

async function handleJoin() {
  if (state.joinInFlight) {
    return;
  }

  const roomCode = (dom.roomCodeInput ? dom.roomCodeInput.value : "").trim();
  const displayName = (dom.displayNameInput ? dom.displayNameInput.value : "").trim();
  const joinIntent = dom.joinIntentSelect ? dom.joinIntentSelect.value : "spectator";

  if (!roomCode) {
    setJoinNote("请先输入比赛码。", true);
    return;
  }

  const payload = {
    room_code: roomCode,
    client_token: state.clientToken || generateClientToken(),
    display_name: displayName,
    join_intent: joinIntent,
  };

  setJoinBusy(true);
  try {
    await enterRoom(payload);
    await refreshAll();
    startPolling();
    setJoinNote("已进入比赛，正在同步场地状态。");
  } catch (err) {
    const message = err instanceof Error ? err.message : "进入比赛失败";
    setJoinNote(message, true);
  } finally {
    setJoinBusy(false);
  }
}

function cacheDomElements() {
  dom.appShell = document.getElementById("app-shell");
  dom.joinScreen = document.getElementById("join-screen");
  dom.joinNote = document.getElementById("join-note");
  dom.roomCodeInput = document.getElementById("room-code-input");
  dom.displayNameInput = document.getElementById("display-name-input");
  dom.joinIntentSelect = document.getElementById("join-intent-select");
  dom.joinRoomButton = document.getElementById("join-room-btn");
  dom.randomRoomButton = document.getElementById("random-room-btn");
  dom.roomCodeBadge = document.getElementById("room-code-badge");
  dom.roomStatusBadge = document.getElementById("room-status-badge");
  dom.intervalBadge = document.getElementById("interval-badge");
  dom.revealBadge = document.getElementById("reveal-badge");
  dom.defaultViewValue = document.getElementById("default-view");
  dom.mySeat = document.getElementById("my-seat");
  dom.myAlias = document.getElementById("my-alias");
  dom.spectatorCount = document.getElementById("spectator-count");
  dom.seatRedCard = document.getElementById("seat-red-card");
  dom.seatBlackCard = document.getElementById("seat-black-card");
  dom.stageTitle = document.getElementById("stage-title");
  dom.turnPill = document.getElementById("turn-pill");
  dom.boardGrid = document.getElementById("board-grid");
  dom.selectionHint = document.getElementById("selection-hint");
  dom.eventList = document.getElementById("event-list");
  dom.participantList = document.getElementById("participant-list");
  dom.clearSelectionButton = document.getElementById("clear-selection-btn");
  dom.viewButtons = Array.from(document.querySelectorAll(".view-btn[data-view]"));
}

function hydratePersistedDefaults() {
  state.clientToken = loadStoredValue(STORAGE_KEYS.clientToken, "");
  const persistedRoomCode = loadStoredValue(STORAGE_KEYS.roomCode, "");
  state.roomCode = "";
  state.displayName = loadStoredValue(STORAGE_KEYS.displayName, "");
  state.joinIntent = loadStoredValue(STORAGE_KEYS.joinIntent, "spectator");
  state.currentView = loadStoredValue(STORAGE_KEYS.currentView, "board");

  if (dom.roomCodeInput) {
    dom.roomCodeInput.value = persistedRoomCode;
  }
  if (dom.displayNameInput) {
    dom.displayNameInput.value = state.displayName;
  }
  if (dom.joinIntentSelect) {
    dom.joinIntentSelect.value = state.joinIntent;
  }
}

function bindEvents() {
  dom.viewButtons.forEach((button) => {
    button.addEventListener("click", () => applyViewMode(button.dataset.view));
  });

  if (dom.clearSelectionButton) {
    dom.clearSelectionButton.addEventListener("click", () => {
      state.selectedFrom = "";
      if (dom.selectionHint) {
        dom.selectionHint.textContent = "已清除选中。";
      }
    });
  }

  if (dom.randomRoomButton && dom.roomCodeInput) {
    dom.randomRoomButton.addEventListener("click", () => {
      const roomCode = "room-" + Math.random().toString(36).slice(2, 8);
      dom.roomCodeInput.value = roomCode;
      saveStoredValue(STORAGE_KEYS.roomCode, roomCode);
      setJoinNote("已生成比赛码，点击进入比赛。");
    });
  }

  if (dom.joinRoomButton) {
    dom.joinRoomButton.addEventListener("click", () => {
      void handleJoin();
    });
  }
}

function boot() {
  cacheDomElements();
  bindEvents();
  hydratePersistedDefaults();
  applyViewMode(state.currentView);
  renderSummary();
  setJoinBusy(false);
  setJoinNote("输入比赛码后即可进入比赛。");
}

document.addEventListener("DOMContentLoaded", boot);
