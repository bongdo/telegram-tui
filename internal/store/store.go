// Package store persists the TUI cache in a local SQLite database. The gotd
// session stays in its own JSON file; this database holds chats, messages,
// media paths and recently used emoji.
package store

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"github.com/bongdo/telegram-tui/internal/domain"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}
	_ = os.Chmod(path, 0o600)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS chats (
  peer_key TEXT PRIMARY KEY,
  position INTEGER NOT NULL DEFAULT 0,
  data TEXT NOT NULL,
  updated_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS messages (
  peer_key TEXT NOT NULL,
  id INTEGER NOT NULL,
  date INTEGER NOT NULL DEFAULT 0,
  data TEXT NOT NULL,
  PRIMARY KEY (peer_key, id)
);
CREATE INDEX IF NOT EXISTS messages_peer_id ON messages(peer_key, id);
CREATE TABLE IF NOT EXISTS recent_emoji (
  emoji TEXT PRIMARY KEY,
  used_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS folders (
  id INTEGER PRIMARY KEY,
  position INTEGER NOT NULL DEFAULT 0,
  data TEXT NOT NULL
);
`)
	return err
}

func (s *Store) SaveChats(chats []domain.Chat) error {
	if s == nil {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM chats`); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO chats(peer_key, position, data, updated_at) VALUES(?, ?, ?, ?)
ON CONFLICT(peer_key) DO UPDATE SET position=excluded.position, data=excluded.data, updated_at=excluded.updated_at`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	now := time.Now().Unix()
	for i, chat := range chats {
		data, err := json.Marshal(chat)
		if err != nil {
			return err
		}
		if _, err := stmt.Exec(chat.PeerKey, i, string(data), now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) LoadChats() ([]domain.Chat, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.Query(`SELECT data FROM chats ORDER BY position`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Chat
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var chat domain.Chat
		if err := json.Unmarshal([]byte(data), &chat); err != nil {
			continue
		}
		chat.HistoryLoaded = false
		out = append(out, chat)
	}
	return out, rows.Err()
}

func (s *Store) SaveFolders(folders []domain.Folder) error {
	if s == nil {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM folders`); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO folders(id, position, data) VALUES(?, ?, ?)
ON CONFLICT(id) DO UPDATE SET position=excluded.position, data=excluded.data`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for i, folder := range folders {
		data, err := json.Marshal(folder)
		if err != nil {
			return err
		}
		if _, err := stmt.Exec(folder.ID, i, string(data)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) LoadFolders() ([]domain.Folder, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.Query(`SELECT data FROM folders ORDER BY position`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Folder
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var folder domain.Folder
		if err := json.Unmarshal([]byte(data), &folder); err != nil {
			continue
		}
		out = append(out, folder)
	}
	return out, rows.Err()
}

func (s *Store) SaveMessages(peerKey string, messages []domain.Message) error {
	if s == nil {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM messages WHERE peer_key = ?`, peerKey); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO messages(peer_key, id, date, data) VALUES(?, ?, ?, ?)
ON CONFLICT(peer_key, id) DO UPDATE SET date=excluded.date, data=excluded.data`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, msg := range messages {
		if msg.Service {
			continue
		}
		data, err := json.Marshal(msg)
		if err != nil {
			return err
		}
		if _, err := stmt.Exec(peerKey, msg.ID, msg.Date, string(data)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) UpsertMessage(peerKey string, msg domain.Message) error {
	if s == nil || msg.Service {
		return nil
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO messages(peer_key, id, date, data) VALUES(?, ?, ?, ?)
ON CONFLICT(peer_key, id) DO UPDATE SET date=excluded.date, data=excluded.data`,
		peerKey, msg.ID, msg.Date, string(data))
	return err
}

func (s *Store) LoadMessages(peerKey string, limit int) ([]domain.Message, error) {
	if s == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(`SELECT data FROM messages WHERE peer_key = ? ORDER BY id DESC LIMIT ?`, peerKey, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Message
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var msg domain.Message
		if err := json.Unmarshal([]byte(data), &msg); err != nil {
			continue
		}
		out = append(out, msg)
	}
	// Rows are newest first; return chronological.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, rows.Err()
}

func (s *Store) DeleteMessages(peerKey string, ids []int) error {
	if s == nil || len(ids) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range ids {
		if _, err := tx.Exec(`DELETE FROM messages WHERE peer_key = ? AND id = ?`, peerKey, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) updateMessage(peerKey string, id int, mutate func(*domain.Message)) error {
	if s == nil {
		return nil
	}
	var data string
	err := s.db.QueryRow(`SELECT data FROM messages WHERE peer_key = ? AND id = ?`, peerKey, id).Scan(&data)
	if err != nil {
		return err
	}
	var msg domain.Message
	if err := json.Unmarshal([]byte(data), &msg); err != nil {
		return err
	}
	mutate(&msg)
	out, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`UPDATE messages SET data = ? WHERE peer_key = ? AND id = ?`, string(out), peerKey, id)
	return err
}

func (s *Store) UpdateText(peerKey string, id int, text string, edited bool) error {
	return s.updateMessage(peerKey, id, func(m *domain.Message) {
		m.Text = text
		m.Edited = edited
	})
}

func (s *Store) UpdateReactions(peerKey string, id int, reactions []domain.Reaction) error {
	return s.updateMessage(peerKey, id, func(m *domain.Message) {
		m.Reactions = reactions
	})
}

func (s *Store) SetMedia(peerKey string, id int, kind, path string) error {
	return s.updateMessage(peerKey, id, func(m *domain.Message) {
		switch kind {
		case "voice":
			if m.Voice != nil {
				m.Voice.File = path
			}
		case "round":
			if m.Round != nil {
				m.Round.File = path
			}
		case "photo":
			if m.Photo != nil {
				m.Photo.File = path
			}
		}
	})
}

func (s *Store) AddRecentEmoji(emoji string) error {
	if s == nil || emoji == "" {
		return nil
	}
	_, err := s.db.Exec(`INSERT INTO recent_emoji(emoji, used_at) VALUES(?, ?)
ON CONFLICT(emoji) DO UPDATE SET used_at=excluded.used_at`, emoji, time.Now().UnixNano())
	return err
}

func (s *Store) RecentEmoji(limit int) ([]string, error) {
	if s == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 16
	}
	rows, err := s.db.Query(`SELECT emoji FROM recent_emoji ORDER BY used_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var emoji string
		if err := rows.Scan(&emoji); err != nil {
			return nil, err
		}
		out = append(out, emoji)
	}
	return out, rows.Err()
}
