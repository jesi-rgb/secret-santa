package main

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	maxBodyBytes       = 64 << 10
	minParticipants    = 3
	maxParticipants    = 100
	maxParticipantName = 40
	maxRoomName        = 60
)

type api struct {
	store *Store
}

// newHandler returns the full HTTP handler: JSON API under /api/ and the
// embedded SPA everywhere else.
func newHandler(store *Store) http.Handler {
	a := &api{store: store}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/rooms", a.createRoom)
	mux.HandleFunc("GET /api/rooms/{roomId}", a.getRoom)
	mux.HandleFunc("POST /api/rooms/{roomId}/reveal", a.reveal)
	mux.HandleFunc("POST /api/rooms/{roomId}/me", a.me)
	mux.HandleFunc("GET /api/rooms/{roomId}/admin", a.admin)
	mux.HandleFunc("POST /api/rooms/{roomId}/admin/reset", a.adminReset)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "Unknown API endpoint.")
	})
	mux.Handle("/", staticHandler())
	return mux
}

type errorBody struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorBody{Error: code, Message: message})
}

func internalError(w http.ResponseWriter, err error) {
	log.Printf("internal error: %v", err)
	writeError(w, http.StatusInternalServerError, "internal_error", "Something went wrong. Please try again.")
}

// decodeJSON reads exactly one JSON value from a size-limited body.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	err := dec.Decode(dst)
	if err == nil {
		if dec.Decode(&struct{}{}) != io.EOF {
			err = errors.New("trailing data")
		}
	}
	if err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			writeError(w, http.StatusRequestEntityTooLarge, "request_too_large", "Request body is too large.")
		} else {
			writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be a valid JSON object.")
		}
		return false
	}
	return true
}

func hasControl(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

type createRoomRequest struct {
	Name         string   `json:"name"`
	Participants []string `json:"participants"`
	// Exclusions lists pairs of participant names that must not gift each
	// other, in either direction (e.g. partners or siblings).
	Exclusions [][]string `json:"exclusions"`
}

type createRoomResponse struct {
	RoomID     string `json:"roomId"`
	AdminToken string `json:"adminToken"`
}

func (a *api) createRoom(w http.ResponseWriter, r *http.Request) {
	var req createRoomRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	roomName := strings.TrimSpace(req.Name)
	if !utf8.ValidString(roomName) || utf8.RuneCountInString(roomName) > maxRoomName || hasControl(roomName) {
		writeError(w, http.StatusBadRequest, "invalid_name", "Room name must be at most 60 characters.")
		return
	}
	if len(req.Participants) < minParticipants || len(req.Participants) > maxParticipants {
		writeError(w, http.StatusBadRequest, "invalid_participants", "A room needs between 3 and 100 participants.")
		return
	}
	names := make([]string, len(req.Participants))
	norms := make([]string, len(req.Participants))
	seen := make(map[string]string, len(req.Participants))
	for i, raw := range req.Participants {
		name := strings.TrimSpace(raw)
		n := utf8.RuneCountInString(name)
		if !utf8.ValidString(name) || n < 1 || n > maxParticipantName || hasControl(name) {
			writeError(w, http.StatusBadRequest, "invalid_name", "Each participant name must be 1 to 40 characters.")
			return
		}
		norm := normalizeName(name)
		if norm == "" {
			writeError(w, http.StatusBadRequest, "invalid_name", "Participant name \""+name+"\" is not valid.")
			return
		}
		if prev, dup := seen[norm]; dup {
			writeError(w, http.StatusBadRequest, "duplicate_name",
				"Names \""+prev+"\" and \""+name+"\" are the same; make them distinct.")
			return
		}
		seen[norm] = name
		names[i], norms[i] = name, norm
	}

	var excluded [][]bool
	if len(req.Exclusions) > 0 {
		index := make(map[string]int, len(norms))
		for i, n := range norms {
			index[n] = i
		}
		excluded = make([][]bool, len(names))
		for i := range excluded {
			excluded[i] = make([]bool, len(names))
		}
		for _, pair := range req.Exclusions {
			if len(pair) != 2 {
				writeError(w, http.StatusBadRequest, "invalid_exclusion", "Each exclusion must be a pair of two names.")
				return
			}
			a, okA := index[normalizeName(pair[0])]
			b, okB := index[normalizeName(pair[1])]
			if !okA || !okB || a == b {
				writeError(w, http.StatusBadRequest, "invalid_exclusion",
					"Each exclusion must name two different participants.")
				return
			}
			excluded[a][b], excluded[b][a] = true, true
		}
	}

	perm, err := drawAssignment(len(names), excluded)
	if errors.Is(err, ErrImpossibleDraw) {
		writeError(w, http.StatusBadRequest, "impossible_exclusions",
			"No draw satisfies these exclusions; remove some of them.")
		return
	}
	if err != nil {
		internalError(w, err)
		return
	}
	secret, err := newRoomSecret(norms)
	if err != nil {
		internalError(w, err)
		return
	}
	adminToken, err := newToken()
	if err != nil {
		internalError(w, err)
		return
	}
	roomID, err := a.store.CreateRoom(r.Context(), roomName, adminToken, secret, names, norms, perm, time.Now())
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, createRoomResponse{RoomID: roomID, AdminToken: adminToken})
}

type roomResponse struct {
	RoomID           string                  `json:"roomId"`
	Name             string                  `json:"name"`
	CreatedAt        string                  `json:"createdAt"`
	ParticipantCount int                     `json:"participantCount"`
	RevealedCount    int                     `json:"revealedCount"`
	Participants     []publicParticipantJSON `json:"participants"`
}

type publicParticipantJSON struct {
	Name     string `json:"name"`
	Revealed bool   `json:"revealed"`
}

func (a *api) getRoom(w http.ResponseWriter, r *http.Request) {
	s, err := a.store.RoomSummary(r.Context(), r.PathValue("roomId"))
	if a.storeError(w, err) {
		return
	}
	participants := make([]publicParticipantJSON, len(s.Participants))
	for i, p := range s.Participants {
		participants[i] = publicParticipantJSON{Name: p.Name, Revealed: p.Revealed}
	}
	writeJSON(w, http.StatusOK, roomResponse{
		RoomID:           s.ID,
		Name:             s.Name,
		CreatedAt:        s.CreatedAt,
		ParticipantCount: s.ParticipantCount,
		RevealedCount:    s.RevealedCount,
		Participants:     participants,
	})
}

type revealResult struct {
	Name      string   `json:"name"`
	Giftee    string   `json:"giftee"`
	Code      []string `json:"code"`
	CodeText  string   `json:"codeText"`
	ViewToken string   `json:"viewToken"`
}

func toRevealResult(rv Reveal) revealResult {
	code := emojiCode(rv.Secret, rv.GifteeNorm)
	return revealResult{
		Name:      rv.Name,
		Giftee:    rv.Giftee,
		Code:      code,
		CodeText:  strings.Join(code, ""),
		ViewToken: rv.ViewToken,
	}
}

func (a *api) reveal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	norm := normalizeName(req.Name)
	if norm == "" {
		writeError(w, http.StatusBadRequest, "invalid_name", "Please enter your name.")
		return
	}
	viewToken, err := newToken()
	if err != nil {
		internalError(w, err)
		return
	}
	rv, err := a.store.RevealByName(r.Context(), r.PathValue("roomId"), norm, viewToken, time.Now())
	if a.storeError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, toRevealResult(rv))
}

func (a *api) me(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ViewToken string `json:"viewToken"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	rv, err := a.store.RevealByToken(r.Context(), r.PathValue("roomId"), req.ViewToken)
	if a.storeError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, toRevealResult(rv))
}

type adminParticipant struct {
	ID         int64   `json:"id"`
	Name       string  `json:"name"`
	Revealed   bool    `json:"revealed"`
	RevealedAt *string `json:"revealedAt"`
}

type adminResponse struct {
	RoomID       string             `json:"roomId"`
	Name         string             `json:"name"`
	CreatedAt    string             `json:"createdAt"`
	Participants []adminParticipant `json:"participants"`
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(h[len(prefix):])
}

func tokenMatches(given, want string) bool {
	return given != "" && subtle.ConstantTimeCompare([]byte(given), []byte(want)) == 1
}

func writeInvalidToken(w http.ResponseWriter) {
	writeError(w, http.StatusUnauthorized, "invalid_token", "Missing or invalid admin token.")
}

func (a *api) admin(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r)
	if token == "" {
		writeInvalidToken(w)
		return
	}
	room, err := a.store.AdminRoom(r.Context(), r.PathValue("roomId"))
	if a.storeError(w, err) {
		return
	}
	if !tokenMatches(token, room.AdminToken) {
		writeInvalidToken(w)
		return
	}
	resp := adminResponse{
		RoomID:       room.ID,
		Name:         room.Name,
		CreatedAt:    room.CreatedAt,
		Participants: make([]adminParticipant, 0, len(room.Participants)),
	}
	for _, p := range room.Participants {
		ap := adminParticipant{ID: p.ID, Name: p.Name, Revealed: p.RevealedAt.Valid}
		if p.RevealedAt.Valid {
			at := p.RevealedAt.String
			ap.RevealedAt = &at
		}
		resp.Participants = append(resp.Participants, ap)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (a *api) adminReset(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r)
	if token == "" {
		writeInvalidToken(w)
		return
	}
	roomID := r.PathValue("roomId")
	want, err := a.store.AdminToken(r.Context(), roomID)
	if a.storeError(w, err) {
		return
	}
	if !tokenMatches(token, want) {
		writeInvalidToken(w)
		return
	}
	var req struct {
		ParticipantID *int64 `json:"participantId"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.ParticipantID == nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "participantId is required.")
		return
	}
	if a.storeError(w, a.store.ResetReveal(r.Context(), roomID, *req.ParticipantID)) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// storeError writes the HTTP response for a store error and reports whether
// it did so.
func (a *api) storeError(w http.ResponseWriter, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, ErrRoomNotFound):
		writeError(w, http.StatusNotFound, "room_not_found", "This room does not exist.")
	case errors.Is(err, ErrNameNotFound):
		writeError(w, http.StatusNotFound, "name_not_found", "That name is not in this room. Check the spelling.")
	case errors.Is(err, ErrAlreadyRevealed):
		writeError(w, http.StatusConflict, "already_revealed", "This name has already been revealed.")
	case errors.Is(err, ErrInvalidToken):
		writeError(w, http.StatusUnauthorized, "invalid_token", "This link is no longer valid.")
	case errors.Is(err, ErrParticipantNotFound):
		writeError(w, http.StatusNotFound, "participant_not_found", "No such participant in this room.")
	default:
		internalError(w, err)
	}
	return true
}
