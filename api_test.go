package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

type testClient struct {
	t   *testing.T
	srv *httptest.Server
}

func newTestClient(t *testing.T) *testClient {
	t.Helper()
	store, err := OpenStore(filepath.Join(t.TempDir(), "santa.db"))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(newHandler(store))
	t.Cleanup(func() {
		srv.Close()
		store.Close()
	})
	return &testClient{t: t, srv: srv}
}

// do sends a request and returns the status and raw body. If out is non-nil
// the body is decoded into it.
func (c *testClient) do(method, path, token string, body any, out any) (int, []byte) {
	c.t.Helper()
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			c.t.Fatal(err)
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.srv.URL+path, rd)
	if err != nil {
		c.t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		c.t.Fatal(err)
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			c.t.Fatalf("%s %s: decode %q: %v", method, path, raw, err)
		}
	}
	return resp.StatusCode, raw
}

func (c *testClient) expectError(method, path, token string, body any, status int, code string) {
	c.t.Helper()
	var e errorBody
	got, raw := c.do(method, path, token, body, &e)
	if got != status || e.Error != code {
		c.t.Fatalf("%s %s: got %d %s, want %d %s", method, path, got, raw, status, code)
	}
	if e.Message == "" {
		c.t.Fatalf("%s %s: missing message", method, path)
	}
}

func (c *testClient) createRoom(name string, participants []string) createRoomResponse {
	c.t.Helper()
	var res createRoomResponse
	status, raw := c.do("POST", "/api/rooms", "", map[string]any{"name": name, "participants": participants}, &res)
	if status != http.StatusCreated {
		c.t.Fatalf("create room: %d %s", status, raw)
	}
	return res
}

func (c *testClient) reveal(roomID, name string) revealResult {
	c.t.Helper()
	var res revealResult
	status, raw := c.do("POST", "/api/rooms/"+roomID+"/reveal", "", map[string]string{"name": name}, &res)
	if status != http.StatusOK {
		c.t.Fatalf("reveal %q: %d %s", name, status, raw)
	}
	return res
}

func checkReveal(t *testing.T, res revealResult, self string, names []string) {
	t.Helper()
	if res.Name != self {
		t.Fatalf("name = %q, want %q", res.Name, self)
	}
	if res.Giftee == self {
		t.Fatalf("%s drew themselves", self)
	}
	found := false
	for _, n := range names {
		found = found || n == res.Giftee
	}
	if !found {
		t.Fatalf("giftee %q is not a participant", res.Giftee)
	}
	if len(res.Code) != 4 {
		t.Fatalf("code has %d emojis", len(res.Code))
	}
	for _, e := range res.Code {
		ok := false
		for _, p := range emojiPalette {
			ok = ok || p == e
		}
		if !ok {
			t.Fatalf("code emoji %q not in palette", e)
		}
	}
	if res.CodeText != strings.Join(res.Code, "") {
		t.Fatalf("codeText %q does not match code %v", res.CodeText, res.Code)
	}
	if res.ViewToken == "" {
		t.Fatal("missing viewToken")
	}
}

func TestAPIFlow(t *testing.T) {
	c := newTestClient(t)
	names := []string{"Ana", "Bruno", "Carla", "Diego"}
	room := c.createRoom("Office party", names)
	if len(room.RoomID) != 10 || room.AdminToken == "" {
		t.Fatalf("bad create response: %+v", room)
	}
	base := "/api/rooms/" + room.RoomID

	var info roomResponse
	status, raw := c.do("GET", base, "", nil, &info)
	if status != http.StatusOK || info.RoomID != room.RoomID || info.Name != "Office party" ||
		info.ParticipantCount != 4 || info.RevealedCount != 0 || info.CreatedAt == "" {
		t.Fatalf("get room: %d %s", status, raw)
	}
	if len(info.Participants) != 4 || info.Participants[0].Name != "Ana" || info.Participants[3].Name != "Diego" {
		t.Fatalf("room info participants not listed by name: %s", raw)
	}
	if bytes.Contains(raw, []byte("giftee")) || bytes.Contains(raw, []byte("code")) {
		t.Fatalf("room info leaks the draw: %s", raw)
	}

	first := c.reveal(room.RoomID, "  ana ")
	checkReveal(t, first, "Ana", names)

	c.expectError("POST", base+"/reveal", "", map[string]string{"name": "ANA"}, http.StatusConflict, "already_revealed")
	c.expectError("POST", base+"/reveal", "", map[string]string{"name": "Zoe"}, http.StatusNotFound, "name_not_found")
	c.expectError("POST", base+"/reveal", "", map[string]string{"name": "  "}, http.StatusBadRequest, "invalid_name")

	var again revealResult
	status, raw = c.do("POST", base+"/me", "", map[string]string{"viewToken": first.ViewToken}, &again)
	if status != http.StatusOK {
		t.Fatalf("me: %d %s", status, raw)
	}
	if again.Name != first.Name || again.Giftee != first.Giftee || again.CodeText != first.CodeText ||
		again.ViewToken != first.ViewToken {
		t.Fatalf("me returned %+v, want %+v", again, first)
	}
	c.expectError("POST", base+"/me", "", map[string]string{"viewToken": "nope"}, http.StatusUnauthorized, "invalid_token")
	c.expectError("POST", base+"/me", "", map[string]string{"viewToken": ""}, http.StatusUnauthorized, "invalid_token")

	status, _ = c.do("GET", base, "", nil, &info)
	if status != http.StatusOK || info.RevealedCount != 1 || !info.Participants[0].Revealed || info.Participants[1].Revealed {
		t.Fatalf("after revealing Ana: %+v", info)
	}

	c.expectError("GET", base+"/admin", "", nil, http.StatusUnauthorized, "invalid_token")
	c.expectError("GET", base+"/admin", "wrong", nil, http.StatusUnauthorized, "invalid_token")
	c.expectError("GET", "/api/rooms/zzzzzzzzzz/admin", room.AdminToken, nil, http.StatusNotFound, "room_not_found")

	var admin struct {
		RoomID       string           `json:"roomId"`
		Name         string           `json:"name"`
		CreatedAt    string           `json:"createdAt"`
		Participants []map[string]any `json:"participants"`
	}
	status, raw = c.do("GET", base+"/admin", room.AdminToken, nil, &admin)
	if status != http.StatusOK || admin.RoomID != room.RoomID || len(admin.Participants) != 4 {
		t.Fatalf("admin: %d %s", status, raw)
	}
	if bytes.Contains(raw, []byte("giftee")) || bytes.Contains(raw, []byte("code")) {
		t.Fatalf("admin response leaks draw: %s", raw)
	}
	var anaID float64
	for _, p := range admin.Participants {
		if len(p) != 4 {
			t.Fatalf("unexpected participant fields: %v", p)
		}
		for _, k := range []string{"id", "name", "revealed", "revealedAt"} {
			if _, ok := p[k]; !ok {
				t.Fatalf("participant missing %q: %v", k, p)
			}
		}
		if p["name"] == "Ana" {
			anaID = p["id"].(float64)
			if p["revealed"] != true || p["revealedAt"] == nil {
				t.Fatalf("Ana should be revealed: %v", p)
			}
		} else if p["revealed"] != false || p["revealedAt"] != nil {
			t.Fatalf("%v should not be revealed", p)
		}
	}
	if anaID == 0 {
		t.Fatal("Ana not listed")
	}

	resetPath := base + "/admin/reset"
	c.expectError("POST", resetPath, "", map[string]any{"participantId": anaID}, http.StatusUnauthorized, "invalid_token")
	c.expectError("POST", resetPath, room.AdminToken, map[string]any{"participantId": 999999}, http.StatusNotFound, "participant_not_found")

	var ok map[string]bool
	status, raw = c.do("POST", resetPath, room.AdminToken, map[string]any{"participantId": anaID}, &ok)
	if status != http.StatusOK || !ok["ok"] {
		t.Fatalf("reset: %d %s", status, raw)
	}
	c.expectError("POST", base+"/me", "", map[string]string{"viewToken": first.ViewToken}, http.StatusUnauthorized, "invalid_token")

	second := c.reveal(room.RoomID, "ana")
	checkReveal(t, second, "Ana", names)
	if second.Giftee != first.Giftee || second.CodeText != first.CodeText {
		t.Fatalf("draw changed after reset: %+v vs %+v", second, first)
	}
	if second.ViewToken == first.ViewToken {
		t.Fatal("viewToken must be new after reset")
	}
}

func TestAPIResetOtherRoomParticipant(t *testing.T) {
	c := newTestClient(t)
	a := c.createRoom("", []string{"A1", "A2", "A3"})
	b := c.createRoom("", []string{"B1", "B2", "B3"})
	var admin adminResponse
	c.do("GET", "/api/rooms/"+b.RoomID+"/admin", b.AdminToken, nil, &admin)
	c.expectError("POST", "/api/rooms/"+a.RoomID+"/admin/reset", a.AdminToken,
		map[string]any{"participantId": admin.Participants[0].ID}, http.StatusNotFound, "participant_not_found")
	c.expectError("GET", "/api/rooms/"+a.RoomID+"/admin", b.AdminToken, nil, http.StatusUnauthorized, "invalid_token")
}

func TestAPIFullRoom(t *testing.T) {
	c := newTestClient(t)
	names := make([]string, 10)
	for i := range names {
		names[i] = fmt.Sprintf("Friend %d", i)
	}
	room := c.createRoom("Ten", names)

	giftees := make(map[string]string)
	codes := make(map[string]string)
	for _, n := range names {
		res := c.reveal(room.RoomID, n)
		checkReveal(t, res, n, names)
		if prev, dup := giftees[res.Giftee]; dup {
			t.Fatalf("%s is gifted by both %s and %s", res.Giftee, prev, n)
		}
		giftees[res.Giftee] = n
		if prev, dup := codes[res.CodeText]; dup {
			t.Fatalf("code %s shared by %s and %s", res.CodeText, prev, n)
		}
		codes[res.CodeText] = n
	}
	if len(giftees) != len(names) {
		t.Fatalf("giftees do not cover all participants: %v", giftees)
	}

	var info roomResponse
	c.do("GET", "/api/rooms/"+room.RoomID, "", nil, &info)
	if info.RevealedCount != 10 || info.ParticipantCount != 10 {
		t.Fatalf("counts: %+v", info)
	}
}

func TestAPICreateValidation(t *testing.T) {
	c := newTestClient(t)
	bad := func(body any, code string) {
		t.Helper()
		c.expectError("POST", "/api/rooms", "", body, http.StatusBadRequest, code)
	}
	bad(map[string]any{"participants": []string{"Ana", "ana", "Bruno"}}, "duplicate_name")
	bad(map[string]any{"participants": []string{"José María", "jose  maria", "Bruno"}}, "duplicate_name")
	bad(map[string]any{"participants": []string{"Ana", "Bruno"}}, "invalid_participants")
	bad(map[string]any{}, "invalid_participants")
	tooMany := make([]string, 101)
	for i := range tooMany {
		tooMany[i] = fmt.Sprintf("p%d", i)
	}
	bad(map[string]any{"participants": tooMany}, "invalid_participants")
	bad(map[string]any{"participants": []string{"Ana", "  ", "Bruno"}}, "invalid_name")
	bad(map[string]any{"participants": []string{"Ana", strings.Repeat("x", 41), "Bruno"}}, "invalid_name")
	bad(map[string]any{"name": strings.Repeat("r", 61), "participants": []string{"Ana", "Bruno", "Carla"}}, "invalid_name")
	bad("not an object", "invalid_request")

	room := c.createRoom("", []string{"  Ana ", "Bruno", strings.Repeat("x", 40)})
	res := c.reveal(room.RoomID, "ana")
	if res.Name != "Ana" {
		t.Fatalf("canonical name should be trimmed, got %q", res.Name)
	}
}

func TestAPIBodyLimitAndUnknownRoutes(t *testing.T) {
	c := newTestClient(t)
	big := map[string]any{"participants": []string{strings.Repeat("a", 70<<10), "b", "c"}}
	c.expectError("POST", "/api/rooms", "", big, http.StatusRequestEntityTooLarge, "request_too_large")

	c.expectError("GET", "/api/rooms/abcdefghij", "", nil, http.StatusNotFound, "room_not_found")
	c.expectError("POST", "/api/rooms/abcdefghij/reveal", "", map[string]string{"name": "Ana"}, http.StatusNotFound, "room_not_found")
	c.expectError("POST", "/api/rooms/abcdefghij/me", "", map[string]string{"viewToken": "x"}, http.StatusNotFound, "room_not_found")
	c.expectError("GET", "/api/nope", "", nil, http.StatusNotFound, "not_found")
	c.expectError("DELETE", "/api/rooms/abcdefghij", "", nil, http.StatusNotFound, "not_found")
}

func TestSPAFallback(t *testing.T) {
	c := newTestClient(t)
	for _, p := range []string{"/", "/abcdefghij", "/abcdefghij/admin"} {
		resp, err := http.Get(c.srv.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s: %d", p, resp.StatusCode)
		}
		if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
			t.Fatalf("GET %s: content-type %q", p, ct)
		}
		if cc := resp.Header.Get("Cache-Control"); cc != "no-cache" {
			t.Fatalf("GET %s: cache-control %q", p, cc)
		}
	}
	resp, err := http.Get(c.srv.URL + "/assets/missing-file.js")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("missing asset: %d", resp.StatusCode)
	}
}
