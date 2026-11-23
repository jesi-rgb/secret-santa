package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

var (
	ErrRoomNotFound        = errors.New("room not found")
	ErrNameNotFound        = errors.New("name not found")
	ErrAlreadyRevealed     = errors.New("already revealed")
	ErrInvalidToken        = errors.New("invalid token")
	ErrParticipantNotFound = errors.New("participant not found")
)

var schema = []string{
	`CREATE TABLE IF NOT EXISTS rooms (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		admin_token TEXT NOT NULL,
		secret BLOB NOT NULL,
		created_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS participants (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		room_id TEXT NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
		name TEXT NOT NULL,
		norm_name TEXT NOT NULL,
		giftee_id INTEGER REFERENCES participants(id),
		revealed_at TEXT NULL,
		view_token TEXT NULL,
		UNIQUE(room_id, norm_name)
	)`,
	`CREATE INDEX IF NOT EXISTS participants_view_token ON participants(view_token)`,
}

type Store struct {
	db *sql.DB
}

// OpenStore opens (creating if needed) the SQLite database at path.
func OpenStore(path string) (*Store, error) {
	dsn := "file:" + path +
		"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			db.Close()
			return nil, fmt.Errorf("apply schema: %w", err)
		}
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// CreateRoom persists a room and its participants in one transaction.
// gifteeIdx[i] is the index (into names) of participant i's giftee.
// It returns the generated room ID, retrying on ID collisions.
func (s *Store) CreateRoom(ctx context.Context, name, adminToken string, secret []byte, names, norms []string, gifteeIdx []int, createdAt time.Time) (string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	created := createdAt.UTC().Format(time.RFC3339)
	var roomID string
	for attempt := 0; ; attempt++ {
		if attempt == 20 {
			return "", errors.New("could not allocate a unique room id")
		}
		id, err := newRoomID()
		if err != nil {
			return "", err
		}
		res, err := tx.ExecContext(ctx,
			`INSERT INTO rooms (id, name, admin_token, secret, created_at) VALUES (?, ?, ?, ?, ?)
			 ON CONFLICT(id) DO NOTHING`,
			id, name, adminToken, secret, created)
		if err != nil {
			return "", err
		}
		if n, err := res.RowsAffected(); err != nil {
			return "", err
		} else if n == 1 {
			roomID = id
			break
		}
	}

	ids := make([]int64, len(names))
	for i := range names {
		res, err := tx.ExecContext(ctx,
			`INSERT INTO participants (room_id, name, norm_name) VALUES (?, ?, ?)`,
			roomID, names[i], norms[i])
		if err != nil {
			return "", err
		}
		if ids[i], err = res.LastInsertId(); err != nil {
			return "", err
		}
	}
	for i, g := range gifteeIdx {
		if _, err := tx.ExecContext(ctx,
			`UPDATE participants SET giftee_id = ? WHERE id = ?`, ids[g], ids[i]); err != nil {
			return "", err
		}
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return roomID, nil
}

type RoomSummary struct {
	ID               string
	Name             string
	CreatedAt        string
	ParticipantCount int
	RevealedCount    int
	Participants     []PublicParticipant
}

// PublicParticipant is what anyone with the room link may see: the name and
// whether it is already locked. Never the draw.
type PublicParticipant struct {
	Name     string
	Revealed bool
}

func (s *Store) RoomSummary(ctx context.Context, roomID string) (RoomSummary, error) {
	r := RoomSummary{ID: roomID}
	err := s.db.QueryRowContext(ctx,
		`SELECT name, created_at FROM rooms WHERE id = ?`, roomID).
		Scan(&r.Name, &r.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return r, ErrRoomNotFound
	}
	if err != nil {
		return r, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT name, revealed_at IS NOT NULL FROM participants
		 WHERE room_id = ? ORDER BY norm_name`, roomID)
	if err != nil {
		return r, err
	}
	defer rows.Close()
	for rows.Next() {
		var p PublicParticipant
		if err := rows.Scan(&p.Name, &p.Revealed); err != nil {
			return r, err
		}
		r.Participants = append(r.Participants, p)
		r.ParticipantCount++
		if p.Revealed {
			r.RevealedCount++
		}
	}
	return r, rows.Err()
}

// Reveal is what a participant learns about their draw.
type Reveal struct {
	Name       string
	Giftee     string
	GifteeNorm string
	Secret     []byte
	ViewToken  string
}

func (s *Store) roomSecret(ctx context.Context, roomID string) ([]byte, error) {
	var secret []byte
	err := s.db.QueryRowContext(ctx, `SELECT secret FROM rooms WHERE id = ?`, roomID).Scan(&secret)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrRoomNotFound
	}
	return secret, err
}

// RevealByName locks the participant with the given normalized name and
// stores viewToken. Only the first caller succeeds; later ones get
// ErrAlreadyRevealed.
func (s *Store) RevealByName(ctx context.Context, roomID, normName, viewToken string, now time.Time) (Reveal, error) {
	secret, err := s.roomSecret(ctx, roomID)
	if err != nil {
		return Reveal{}, err
	}
	var id int64
	rv := Reveal{Secret: secret, ViewToken: viewToken}
	err = s.db.QueryRowContext(ctx,
		`SELECT p.id, p.name, g.name, g.norm_name
		 FROM participants p JOIN participants g ON g.id = p.giftee_id
		 WHERE p.room_id = ? AND p.norm_name = ?`, roomID, normName).
		Scan(&id, &rv.Name, &rv.Giftee, &rv.GifteeNorm)
	if errors.Is(err, sql.ErrNoRows) {
		return Reveal{}, ErrNameNotFound
	}
	if err != nil {
		return Reveal{}, err
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE participants SET revealed_at = ?, view_token = ?
		 WHERE id = ? AND revealed_at IS NULL`,
		now.UTC().Format(time.RFC3339), viewToken, id)
	if err != nil {
		return Reveal{}, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return Reveal{}, err
	}
	if n == 0 {
		return Reveal{}, ErrAlreadyRevealed
	}
	return rv, nil
}

// RevealByToken returns the reveal previously granted with viewToken.
func (s *Store) RevealByToken(ctx context.Context, roomID, viewToken string) (Reveal, error) {
	secret, err := s.roomSecret(ctx, roomID)
	if err != nil {
		return Reveal{}, err
	}
	if viewToken == "" {
		return Reveal{}, ErrInvalidToken
	}
	rv := Reveal{Secret: secret, ViewToken: viewToken}
	err = s.db.QueryRowContext(ctx,
		`SELECT p.name, g.name, g.norm_name
		 FROM participants p JOIN participants g ON g.id = p.giftee_id
		 WHERE p.room_id = ? AND p.view_token = ? AND p.revealed_at IS NOT NULL`,
		roomID, viewToken).
		Scan(&rv.Name, &rv.Giftee, &rv.GifteeNorm)
	if errors.Is(err, sql.ErrNoRows) {
		return Reveal{}, ErrInvalidToken
	}
	if err != nil {
		return Reveal{}, err
	}
	return rv, nil
}

type AdminParticipant struct {
	ID         int64
	Name       string
	RevealedAt sql.NullString
}

type AdminRoom struct {
	ID           string
	Name         string
	CreatedAt    string
	AdminToken   string
	Participants []AdminParticipant
}

// AdminRoom loads the room with its admin token and participant reveal states.
// Callers must verify the token before exposing anything.
func (s *Store) AdminRoom(ctx context.Context, roomID string) (AdminRoom, error) {
	a := AdminRoom{ID: roomID}
	err := s.db.QueryRowContext(ctx,
		`SELECT name, created_at, admin_token FROM rooms WHERE id = ?`, roomID).
		Scan(&a.Name, &a.CreatedAt, &a.AdminToken)
	if errors.Is(err, sql.ErrNoRows) {
		return a, ErrRoomNotFound
	}
	if err != nil {
		return a, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, revealed_at FROM participants WHERE room_id = ? ORDER BY id`, roomID)
	if err != nil {
		return a, err
	}
	defer rows.Close()
	for rows.Next() {
		var p AdminParticipant
		if err := rows.Scan(&p.ID, &p.Name, &p.RevealedAt); err != nil {
			return a, err
		}
		a.Participants = append(a.Participants, p)
	}
	return a, rows.Err()
}

// AdminToken returns the room's admin token.
func (s *Store) AdminToken(ctx context.Context, roomID string) (string, error) {
	var tok string
	err := s.db.QueryRowContext(ctx, `SELECT admin_token FROM rooms WHERE id = ?`, roomID).Scan(&tok)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrRoomNotFound
	}
	return tok, err
}

// ResetReveal unlocks a participant so their name can be revealed again.
// The draw is unchanged.
func (s *Store) ResetReveal(ctx context.Context, roomID string, participantID int64) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE participants SET revealed_at = NULL, view_token = NULL
		 WHERE id = ? AND room_id = ?`, participantID, roomID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrParticipantNotFound
	}
	return nil
}
