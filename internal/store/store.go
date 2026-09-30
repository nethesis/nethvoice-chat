// Package store is the gateway's SQLite: push registrations and a few settings.
// Messages are never stored here; the archive is ejabberd's MAM.
package store

import (
	"database/sql"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct{ db *sql.DB }

// WebSub is a browser Web Push subscription bound to an XMPP push node.
type WebSub struct {
	Node, Username, Endpoint, P256dh, Auth string
}

// PNMToken is an Acrobits device, as reported by push_token_report, bound to a node.
type PNMToken struct {
	Node, Username, Selector, Token, AppID string
}

func Open(dir string) (*Store, error) {
	db, err := sql.Open("sqlite", filepath.Join(dir, "chat-gw.db")+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	for _, q := range []string{
		`CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS web_subs (node TEXT PRIMARY KEY, username TEXT NOT NULL, endpoint TEXT NOT NULL, p256dh TEXT NOT NULL, auth TEXT NOT NULL, created INTEGER NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS pnm_tokens (node TEXT PRIMARY KEY, username TEXT NOT NULL, selector TEXT NOT NULL, token TEXT NOT NULL, app_id TEXT NOT NULL, created INTEGER NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS web_subs_user ON web_subs(username)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS web_subs_endpoint ON web_subs(endpoint)`,
		`CREATE INDEX IF NOT EXISTS pnm_tokens_user ON pnm_tokens(username)`,
		`CREATE INDEX IF NOT EXISTS pnm_tokens_account ON pnm_tokens(username, selector)`,
	} {
		if _, err := db.Exec(q); err != nil {
			return nil, err
		}
	}
	// Columns added later: an older database gets them; on a current one the statement fails harmlessly.
	for _, q := range []string{
		`ALTER TABLE pnm_tokens ADD COLUMN last_seen INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE pnm_tokens ADD COLUMN pending_since INTEGER NOT NULL DEFAULT 0`,
	} {
		_, _ = db.Exec(q)
	}
	return &Store{db}, nil
}

func (s *Store) Meta(key string) (string, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return v, err
}

func (s *Store) SetMeta(key, value string) error {
	_, err := s.db.Exec(`INSERT INTO meta(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// PutWebSub stores a browser subscription: same endpoint and user keep the node,
// a new user on the same browser gets a new one.
func (s *Store) PutWebSub(w WebSub) (node string, err error) {
	var owner string
	err = s.db.QueryRow(`SELECT node, username FROM web_subs WHERE endpoint = ?`, w.Endpoint).Scan(&node, &owner)
	switch {
	case err == nil && owner == w.Username:
		_, err = s.db.Exec(`UPDATE web_subs SET p256dh = ?, auth = ? WHERE node = ?`, w.P256dh, w.Auth, node)
		return node, err
	case err == nil:
		if _, err := s.db.Exec(`DELETE FROM web_subs WHERE node = ?`, node); err != nil {
			return "", err
		}
	case err != sql.ErrNoRows:
		return "", err
	}
	_, err = s.db.Exec(`INSERT INTO web_subs(node, username, endpoint, p256dh, auth, created) VALUES(?,?,?,?,?,?)`,
		w.Node, w.Username, w.Endpoint, w.P256dh, w.Auth, time.Now().Unix())
	return w.Node, err
}

func (s *Store) WebSub(node string) (*WebSub, error) {
	w := &WebSub{}
	err := s.db.QueryRow(`SELECT node, username, endpoint, p256dh, auth FROM web_subs WHERE node = ?`, node).Scan(&w.Node, &w.Username, &w.Endpoint, &w.P256dh, &w.Auth)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return w, err
}

func (s *Store) DeleteWebSub(node, username string) error {
	_, err := s.db.Exec(`DELETE FROM web_subs WHERE node = ? AND (username = ? OR ? = '')`, node, username, username)
	return err
}

// PutPNM stores an Acrobits device; a known account and selector keep their (random) node.
func (s *Store) PutPNM(t PNMToken) (node string, err error) {
	err = s.db.QueryRow(`SELECT node FROM pnm_tokens WHERE username = ? AND selector = ?`, t.Username, t.Selector).Scan(&node)
	if err == nil {
		_, err = s.db.Exec(`UPDATE pnm_tokens SET token = ?, app_id = ? WHERE node = ?`, t.Token, t.AppID, node)
		return node, err
	}
	if err != sql.ErrNoRows {
		return "", err
	}
	_, err = s.db.Exec(`INSERT INTO pnm_tokens(node, username, selector, token, app_id, created) VALUES(?,?,?,?,?,?)`,
		t.Node, t.Username, t.Selector, t.Token, t.AppID, time.Now().Unix())
	return t.Node, err
}

func (s *Store) PNM(node string) (*PNMToken, error) {
	t := &PNMToken{}
	err := s.db.QueryRow(`SELECT node, username, selector, token, app_id FROM pnm_tokens WHERE node = ?`, node).Scan(&t.Node, &t.Username, &t.Selector, &t.Token, &t.AppID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return t, err
}

func (s *Store) DeletePNM(node string) error {
	_, err := s.db.Exec(`DELETE FROM pnm_tokens WHERE node = ?`, node)
	return err
}

// Seen records that the app of this account called the gateway: it is logged in and answering.
func (s *Store) Seen(username string) {
	_, _ = s.db.Exec(`UPDATE pnm_tokens SET last_seen = ?, pending_since = 0 WHERE username = ?`, time.Now().Unix(), username)
}

// Pushed records a push the phone has not answered yet; the first unanswered one counts.
func (s *Store) Pushed(node string) {
	_, _ = s.db.Exec(`UPDATE pnm_tokens SET pending_since = ? WHERE node = ? AND pending_since = 0`, time.Now().Unix(), node)
}

// PNMUsers: accounts with an app seen in the last week and no push unanswered for 3 minutes
// (a logged-in app fetches right after a push; the app never reports a logout).
func (s *Store) PNMUsers() ([]string, error) {
	now := time.Now().Unix()
	rows, err := s.db.Query(`SELECT DISTINCT username FROM pnm_tokens
		WHERE MAX(last_seen, created) > ? AND (pending_since = 0 OR pending_since > ?)`, now-7*24*3600, now-3*60)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []string
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}
