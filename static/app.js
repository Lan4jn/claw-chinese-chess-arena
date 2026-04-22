"use strict";

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
  defaultViewValue: null,
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
  if (dom.roomStatusBadge) {
    dom.roomStatusBadge.textContent = state.isHost ? "主持人视角" : "等待加入";
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
    throw new Error(message);
  }

  state.lastError = "";
  return payload;
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
  dom.defaultViewValue = document.getElementById("default-view");
  dom.viewButtons = Array.from(document.querySelectorAll(".view-btn[data-view]"));
}

function hydratePersistedDefaults() {
  state.clientToken = loadStoredValue(STORAGE_KEYS.clientToken, "");
  state.roomCode = loadStoredValue(STORAGE_KEYS.roomCode, "");
  state.displayName = loadStoredValue(STORAGE_KEYS.displayName, "");
  state.joinIntent = loadStoredValue(STORAGE_KEYS.joinIntent, "spectator");
  state.currentView = loadStoredValue(STORAGE_KEYS.currentView, "board");

  if (dom.roomCodeInput) {
    dom.roomCodeInput.value = state.roomCode;
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

  if (dom.randomRoomButton && dom.roomCodeInput) {
    dom.randomRoomButton.addEventListener("click", () => {
      const roomCode = "room-" + Math.random().toString(36).slice(2, 8);
      dom.roomCodeInput.value = roomCode;
      state.roomCode = roomCode;
      saveStoredValue(STORAGE_KEYS.roomCode, roomCode);
      setJoinNote("已生成比赛码，点击进入比赛。");
    });
  }

  if (dom.joinRoomButton) {
    dom.joinRoomButton.addEventListener("click", async () => {
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

      try {
        await enterRoom(payload);
      } catch (err) {
        const message = err instanceof Error ? err.message : "进入比赛失败";
        setJoinNote(message, true);
      }
    });
  }
}

function boot() {
  cacheDomElements();
  bindEvents();
  hydratePersistedDefaults();
  applyViewMode(state.currentView);
  updateHeaderState();
  setJoinNote("输入比赛码后即可进入比赛。");
}

document.addEventListener("DOMContentLoaded", boot);
