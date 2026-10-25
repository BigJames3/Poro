// Package cursor encodes the opaque pagination cursors of the feeds.
package cursor

import (
	"encoding/base64"
	"errors"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrInvalid is returned for a cursor this service did not issue.
var ErrInvalid = errors.New("invalid cursor")

// Time is a keyset position on (published_at DESC, video_id DESC).
type Time struct {
	At      time.Time
	VideoID uuid.UUID
}

// Score is a keyset position on (trending_score DESC, video_id DESC).
type Score struct {
	Score   float64
	VideoID uuid.UUID
}

// Session is a position in a For You session.
type Session struct {
	ID     string
	Offset int
}

const (
	kindTime    = "t"
	kindScore   = "s"
	kindSession = "f"
)

// scoped marks the cursors of a client that declared photo posts (ADR-0020).
// A cursor is only accepted with the capabilities it was issued for: replayed
// without the header, a photo cursor is invalid and never served. Video-only
// cursors keep their original form, so cursors issued before photo posts
// stay valid.
func scoped(kind string, photoPosts bool) string {
	if photoPosts {
		return kind + "p"
	}
	return kind
}

func encode(kind, value, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(kind + "|" + value + "|" + id))
}

func decode(s, kind string) (string, string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return "", "", ErrInvalid
	}
	parts := strings.Split(string(raw), "|")
	if len(parts) != 3 || parts[0] != kind {
		return "", "", ErrInvalid
	}
	return parts[1], parts[2], nil
}

// EncodeTime packs a time position.
func EncodeTime(p Time, photoPosts bool) string {
	return encode(scoped(kindTime, photoPosts), p.At.UTC().Format(time.RFC3339Nano), p.VideoID.String())
}

// DecodeTime unpacks a time position; "" is the first page and returns nil.
func DecodeTime(s string, photoPosts bool) (*Time, error) {
	if s == "" {
		return nil, nil
	}
	value, id, err := decode(s, scoped(kindTime, photoPosts))
	if err != nil {
		return nil, err
	}
	at, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil, ErrInvalid
	}
	vid, err := uuid.Parse(id)
	if err != nil {
		return nil, ErrInvalid
	}
	return &Time{At: at, VideoID: vid}, nil
}

// EncodeScore packs a score position. The float round-trips exactly.
func EncodeScore(p Score, photoPosts bool) string {
	return encode(scoped(kindScore, photoPosts), strconv.FormatFloat(p.Score, 'g', -1, 64), p.VideoID.String())
}

// DecodeScore unpacks a score position; "" is the first page and returns nil.
func DecodeScore(s string, photoPosts bool) (*Score, error) {
	if s == "" {
		return nil, nil
	}
	value, id, err := decode(s, scoped(kindScore, photoPosts))
	if err != nil {
		return nil, err
	}
	score, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(score) || math.IsInf(score, 0) {
		return nil, ErrInvalid
	}
	vid, err := uuid.Parse(id)
	if err != nil {
		return nil, ErrInvalid
	}
	return &Score{Score: score, VideoID: vid}, nil
}

// EncodeSession packs a For You position.
func EncodeSession(p Session, photoPosts bool) string {
	return encode(scoped(kindSession, photoPosts), strconv.Itoa(p.Offset), p.ID)
}

// DecodeSession unpacks a For You position; "" starts a new session and returns nil.
func DecodeSession(s string, photoPosts bool) (*Session, error) {
	if s == "" {
		return nil, nil
	}
	value, id, err := decode(s, scoped(kindSession, photoPosts))
	if err != nil {
		return nil, err
	}
	offset, err := strconv.Atoi(value)
	if err != nil || offset < 0 || id == "" {
		return nil, ErrInvalid
	}
	return &Session{ID: id, Offset: offset}, nil
}
