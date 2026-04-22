# Windows 7 Compatible MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the project a single-version MVP that runs with Go 1.20-compatible server code, serves a working browser UI, and closes the host/spectator/human-move flow.

**Architecture:** Keep the existing single-process Go server and current Arena/Match domain model. Limit backend compatibility work to Go version and HTTP routing, then wire the existing static shell with a new `static/app.js` that consumes the existing JSON APIs through polling and explicit host actions.

**Tech Stack:** Go 1.20 standard library, `net/http`, `testing`, `httptest`, static HTML/CSS/JavaScript, browser local storage

---

## File Structure

### Files to modify

- `go.mod`
  - Lower the module Go version to a Windows 7-compatible target.
- `app.go`
  - Replace Go 1.22-only routing with a Go 1.20-compatible compatibility router while preserving endpoint contracts.
- `http_test.go`
  - Update/add tests for routing and static asset serving.
- `arena_test.go`
  - Add or adjust backend tests only if a frontend-supporting contract fix requires it.
- `static/index.html`
  - Only touch if a tiny hook or attribute adjustment is required by the browser logic.

### Files to create

- `static/app.js`
  - Browser state, API client, polling, rendering, board interaction, and host control wiring.

## Task 1: Make the server Go 1.20-compatible

**Files:**
- Modify: `go.mod`
- Modify: `app.go`
- Test: `http_test.go`

- [ ] **Step 1: Write the failing routing compatibility tests**

```go
func TestArenaHTTPStartMatchAndFetchPublicState(t *testing.T) {
	app := NewApp(NewMemorySnapshotStore())

	req := httptest.NewRequest(http.MethodPost, "/api/arena/enter", bytes.NewReader([]byte(`{"room_code":"http-room","client_token":"host-token","join_intent":"player"}`)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("enter host expected 200, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/arena/enter", bytes.NewReader([]byte(`{"room_code":"http-room","client_token":"guest-token","join_intent":"player"}`)))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("enter guest expected 200, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/arena/http-room/match/start", bytes.NewReader([]byte(`{"host_token":"host-token"}`)))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("start match expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/arena/http-room/match", nil)
	rr = httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("public match expected 200, got %d", rr.Code)
	}
}

func TestArenaHTTPHostRoomRequiresMatchingRoute(t *testing.T) {
	app := NewApp(NewMemorySnapshotStore())

	req := httptest.NewRequest(http.MethodPost, "/api/arena/enter", bytes.NewReader([]byte(`{"room_code":"host-room","client_token":"host-token","join_intent":"player"}`)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("enter host expected 200, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/arena/host-room/host?token=host-token", nil)
	rr = httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("host room expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
}
```

- [ ] **Step 2: Run the HTTP tests to verify they fail under the current Go 1.20 target**

Run: `GOTOOLCHAIN=local go test ./...`
Expected: FAIL with `go.mod requires go >= 1.26` and, after lowering `go.mod`, route-related compilation or behavior failures from `app.go`

- [ ] **Step 3: Lower the module version and replace Go 1.22-only routing**

```go
module pico-xiangqi-arena

go 1.20
```

```go
func (a *App) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/" {
			http.ServeFile(w, r, filepath.Join("static", "index.html"))
			return
		}
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/static/") {
			http.StripPrefix("/static/", http.FileServer(http.Dir("static"))).ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			a.serveAPI(w, r)
			return
		}
		http.NotFound(w, r)
	})

	return loggingMiddleware(mux)
}

func (a *App) serveAPI(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(r.URL.Path, "/")
	parts := strings.Split(path, "/")

	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/api/health":
		writeJSON(w, http.StatusOK, map[string]string{
			"status": "ok",
			"time":   time.Now().Format(time.RFC3339),
		})
		return
	case r.Method == http.MethodPost && r.URL.Path == "/api/arena/enter":
		a.handleEnter(w, r)
		return
	case len(parts) >= 3 && parts[0] == "api" && parts[1] == "arena":
		code := parts[2]
		a.serveArenaRoute(w, r, code, parts[3:])
		return
	default:
		http.NotFound(w, r)
	}
}
```

- [ ] **Step 4: Run the backend tests to verify the compatibility rewrite passes**

Run: `GOTOOLCHAIN=local go test ./...`
Expected: FAIL only on missing `static/app.js` or frontend-related assertions, with no Go version or route API failures

- [ ] **Step 5: Commit the compatibility foundation**

```bash
git add go.mod app.go http_test.go
git commit -m "refactor: make server routing go1.20 compatible"
```

## Task 2: Add the missing browser application shell

**Files:**
- Create: `static/app.js`
- Test: `http_test.go`

- [ ] **Step 1: Write the failing static asset test**

```go
func TestStaticAssetsAreServed(t *testing.T) {
	app := NewApp(NewMemorySnapshotStore())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET / expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Pico Xiangqi Arena") {
		t.Fatalf("expected index page to contain app title")
	}

	for _, path := range []string{"/static/style.css", "/static/app.js"} {
		req = httptest.NewRequest(http.MethodGet, path, nil)
		rr = httptest.NewRecorder()
		app.routes().ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("GET %s expected 200, got %d", path, rr.Code)
		}
		if rr.Body.Len() == 0 {
			t.Fatalf("GET %s returned empty body", path)
		}
	}
}
```

- [ ] **Step 2: Run the static asset test to verify it fails because `static/app.js` is missing**

Run: `GOTOOLCHAIN=local go test ./... -run TestStaticAssetsAreServed -v`
Expected: FAIL with `GET /static/app.js expected 200`

- [ ] **Step 3: Create the browser app entry with API helpers, state, and bootstrapping**

```javascript
const state = {
  clientToken: loadLocal("client_token") || randomToken(),
  roomCode: loadLocal("room_code") || "",
  displayName: loadLocal("display_name") || "",
  joinIntent: "spectator",
  currentView: loadLocal("preferred_view") || "board",
  isHost: false,
  participant: null,
  publicRoom: null,
  publicMatch: null,
  hostRoom: null,
  hostMatch: null,
  selectedFrom: null,
  pollTimer: null,
  lastError: "",
};

async function apiRequest(path, options) {
  const response = await fetch(path, options);
  const payload = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(payload.error || `request failed: ${response.status}`);
  }
  return payload;
}

async function enterRoom(payload) {
  return apiRequest("/api/arena/enter", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
}

function boot() {
  cacheElements();
  bindEvents();
  applyViewMode(state.currentView);
  hydrateFormDefaults();
}

document.addEventListener("DOMContentLoaded", boot);
```

- [ ] **Step 4: Run the static asset test to verify the file is now served**

Run: `GOTOOLCHAIN=local go test ./... -run TestStaticAssetsAreServed -v`
Expected: PASS

- [ ] **Step 5: Commit the static app skeleton**

```bash
git add static/app.js http_test.go
git commit -m "feat: add browser app shell"
```

## Task 3: Wire room entry, polling, and public rendering

**Files:**
- Modify: `static/app.js`
- Test: `http_test.go`

- [ ] **Step 1: Write a failing HTTP flow test that covers room entry and public room fetch**

```go
func TestEnterArenaCreatesRoomAndReturnsPublicAlias(t *testing.T) {
	app := NewApp(NewMemorySnapshotStore())

	body, err := json.Marshal(map[string]string{
		"room_code":    "demo-room",
		"client_token": "host-token",
		"join_intent":  string(JoinIntentAuto),
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/arena/enter", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}
```

- [ ] **Step 2: Run the targeted room-entry tests to verify they still fail or are incomplete for the browser flow**

Run: `GOTOOLCHAIN=local go test ./... -run 'TestEnterArenaCreatesRoomAndReturnsPublicAlias|TestArenaHTTPStartMatchAndFetchPublicState' -v`
Expected: PASS on backend contract, while the browser flow remains incomplete because no polling/render logic exists yet

- [ ] **Step 3: Implement join flow, polling, and public room/match rendering in `static/app.js`**

```javascript
async function handleJoin() {
  const roomCode = elements.roomCodeInput.value.trim().toLowerCase();
  const displayName = elements.displayNameInput.value.trim();
  const joinIntent = elements.joinIntentSelect.value;

  const view = await enterRoom({
    room_code: roomCode,
    client_token: state.clientToken,
    display_name: displayName,
    join_intent: joinIntent,
  });

  state.roomCode = view.room.code;
  state.displayName = displayName;
  state.isHost = Boolean(view.is_host);
  state.participant = view.participant;
  saveLocal("room_code", state.roomCode);
  saveLocal("display_name", state.displayName);
  renderEntry(view);
  await refreshAll();
  startPolling();
}

async function refreshAll() {
  if (!state.roomCode) return;
  state.publicRoom = await apiRequest(`/api/arena/${encodeURIComponent(state.roomCode)}`);
  state.publicMatch = await fetchMatch();
  if (state.isHost) {
    state.hostRoom = await apiRequest(`/api/arena/${encodeURIComponent(state.roomCode)}/host?token=${encodeURIComponent(state.clientToken)}`);
    state.hostMatch = await fetchHostMatch();
  }
  render();
}

function startPolling() {
  stopPolling();
  state.pollTimer = window.setInterval(() => {
    refreshAll().catch(showError);
  }, 1500);
}
```

- [ ] **Step 4: Run all backend tests to verify frontend additions did not break the server contract**

Run: `GOTOOLCHAIN=local go test ./...`
Expected: PASS through backend and static asset tests, with no regressions in room or match HTTP behavior

- [ ] **Step 5: Commit the public flow wiring**

```bash
git add static/app.js http_test.go
git commit -m "feat: wire room entry and public match polling"
```

## Task 4: Implement host controls and host-only rendering

**Files:**
- Modify: `static/app.js`
- Test: `http_test.go`

- [ ] **Step 1: Write failing HTTP tests for host settings and reveal flow**

```go
func TestArenaHTTPHostCanUpdateSettingsAndReveal(t *testing.T) {
	app := NewApp(NewMemorySnapshotStore())

	req := httptest.NewRequest(http.MethodPost, "/api/arena/enter", bytes.NewReader([]byte(`{"room_code":"control-room","client_token":"host-token","join_intent":"player"}`)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("enter host expected 200, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/arena/control-room/settings", bytes.NewReader([]byte(`{"host_token":"host-token","step_interval_ms":1500,"default_view":"commentary"}`)))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("settings expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/arena/control-room/reveal", bytes.NewReader([]byte(`{"host_token":"host-token","scope":"all"}`)))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("reveal expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
}
```

- [ ] **Step 2: Run the host HTTP tests to verify the server contract before adding browser controls**

Run: `GOTOOLCHAIN=local go test ./... -run TestArenaHTTPHostCanUpdateSettingsAndReveal -v`
Expected: PASS once routing compatibility is correct, confirming the frontend can call these endpoints as designed

- [ ] **Step 3: Implement host drawer rendering and control handlers in `static/app.js`**

```javascript
async function saveSettings() {
  await apiRequest(`/api/arena/${encodeURIComponent(state.roomCode)}/settings`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      host_token: state.clientToken,
      step_interval_ms: Number(elements.stepIntervalInput.value) || 0,
      default_view: elements.defaultViewSelect.value,
    }),
  });
  await refreshAll();
}

async function setReveal(scope) {
  await apiRequest(`/api/arena/${encodeURIComponent(state.roomCode)}/reveal`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ host_token: state.clientToken, scope }),
  });
  await refreshAll();
}

async function controlMatch(action) {
  await apiRequest(`/api/arena/${encodeURIComponent(state.roomCode)}/match/${action}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ host_token: state.clientToken }),
  });
  await refreshAll();
}
```

- [ ] **Step 4: Run the full test suite to verify host UI support did not require extra backend changes**

Run: `GOTOOLCHAIN=local go test ./...`
Expected: PASS

- [ ] **Step 5: Commit the host control flow**

```bash
git add static/app.js http_test.go
git commit -m "feat: add host control drawer actions"
```

## Task 5: Implement board interaction and human move submission

**Files:**
- Modify: `static/app.js`
- Test: `arena_test.go`
- Test: `http_test.go`

- [ ] **Step 1: Write failing tests for human move ownership and move submission over HTTP**

```go
func TestArenaHumanMoveRequiresCurrentSeatOwner(t *testing.T) {
	store := NewMemorySnapshotStore()
	arena := NewArena(store)

	hostView, err := arena.Enter(EnterRequest{
		RoomCode:    "move-room",
		ClientToken: "host",
		JoinIntent:  JoinIntentPlayer,
	})
	if err != nil {
		t.Fatalf("host enter error = %v", err)
	}
	guestView, err := arena.Enter(EnterRequest{
		RoomCode:    "move-room",
		ClientToken: "guest",
		JoinIntent:  JoinIntentPlayer,
	})
	if err != nil {
		t.Fatalf("guest enter error = %v", err)
	}
	if _, err := arena.StartMatch(hostView.Room.Code, hostView.Participant.ID); err != nil {
		t.Fatalf("StartMatch() error = %v", err)
	}
	if _, err := arena.SubmitMove(hostView.Room.Code, guestView.Participant.ID, "a6-a5"); err == nil {
		t.Fatalf("expected black player to be rejected on red turn")
	}
}

func TestArenaHTTPMoveEndpointAcceptsCurrentHumanMove(t *testing.T) {
	app := NewApp(NewMemorySnapshotStore())

	steps := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/api/arena/enter", `{"room_code":"http-move","client_token":"host","join_intent":"player"}`},
		{http.MethodPost, "/api/arena/enter", `{"room_code":"http-move","client_token":"guest","join_intent":"player"}`},
		{http.MethodPost, "/api/arena/http-move/match/start", `{"host_token":"host"}`},
		{http.MethodPost, "/api/arena/http-move/move", `{"client_token":"host","move":"a6-a5"}`},
	}

	for _, step := range steps {
		req := httptest.NewRequest(step.method, step.path, bytes.NewReader([]byte(step.body)))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		app.routes().ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s %s expected 200, got %d body=%s", step.method, step.path, rr.Code, rr.Body.String())
		}
	}
}
```

- [ ] **Step 2: Run the move-related tests to verify the browser move path expectations**

Run: `GOTOOLCHAIN=local go test ./... -run 'TestArenaHumanMoveRequiresCurrentSeatOwner|TestArenaHTTPMoveEndpointAcceptsCurrentHumanMove' -v`
Expected: FAIL on the HTTP move test if route parsing is incomplete or path dispatch misses `/api/arena/{code}/move`

- [ ] **Step 3: Implement board selection, legal-move affordances, and human move submission in `static/app.js`**

```javascript
function onBoardCellClick(x, y) {
  const cell = fileChar(x) + String(y);
  if (!state.publicMatch || !state.publicMatch.status) return;

  if (!state.selectedFrom) {
    state.selectedFrom = cell;
    renderBoard();
    return;
  }

  const move = `${state.selectedFrom}-${cell}`;
  state.selectedFrom = null;
  submitMove(move).catch(showError);
}

async function submitMove(move) {
  await apiRequest(`/api/arena/${encodeURIComponent(state.roomCode)}/move`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      client_token: state.clientToken,
      move: move,
    }),
  });
  await refreshAll();
}
```

- [ ] **Step 4: Run the full suite to verify move submission and board interaction support the MVP**

Run: `GOTOOLCHAIN=local go test ./...`
Expected: PASS

- [ ] **Step 5: Commit the human move flow**

```bash
git add static/app.js arena_test.go http_test.go
git commit -m "feat: add browser move submission"
```

## Task 6: Verify the complete MVP

**Files:**
- Modify: `static/app.js`
- Modify: `app.go`
- Modify: `http_test.go`
- Modify: `arena_test.go`

- [ ] **Step 1: Run the complete automated verification suite**

Run: `GOTOOLCHAIN=local go test ./...`
Expected: PASS

- [ ] **Step 2: Run a local server smoke test**

Run: `GOTOOLCHAIN=local go run .`
Expected: server starts successfully and logs `Pico Xiangqi Arena listening on http://localhost:8080`

- [ ] **Step 3: Manually verify the browser flow**

```text
1. Open http://localhost:8080
2. Enter a new room code and confirm the first entrant becomes host
3. Open a second browser session and join as a second player or spectator
4. Use the host drawer to configure seats and start the match
5. Submit a legal move from the active human seat
6. Verify logs, room badges, board, and phase update correctly
```

- [ ] **Step 4: Make only minimal polish fixes discovered during smoke testing**

```javascript
function showError(error) {
  state.lastError = error instanceof Error ? error.message : String(error);
  if (elements.joinNote) {
    elements.joinNote.textContent = state.lastError;
  }
  renderStatus();
}
```

- [ ] **Step 5: Commit the verified MVP**

```bash
git add go.mod app.go static/app.js http_test.go arena_test.go
git commit -m "feat: complete windows 7 compatible MVP"
```
