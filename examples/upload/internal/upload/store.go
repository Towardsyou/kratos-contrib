package upload

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// Store persists uploads in SQLite.
type Store struct {
	db *sql.DB
}

func OpenStore(ctx context.Context, path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.ExecContext(ctx, `
		PRAGMA busy_timeout = 5000;
		CREATE TABLE IF NOT EXISTS uploads (
			id INTEGER PRIMARY KEY,
			object_key TEXT NOT NULL UNIQUE,
			owner TEXT NOT NULL,
			filename TEXT NOT NULL,
			expected_size INTEGER NOT NULL,
			status TEXT NOT NULL CHECK (status IN ('pending', 'verified', 'rejected')),
			size INTEGER NOT NULL DEFAULT 0,
			etag TEXT NOT NULL DEFAULT ''
		);
		CREATE INDEX IF NOT EXISTS uploads_list ON uploads (owner, status, id DESC);
	`)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initialize upload store: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

const recordColumns = "object_key, filename, expected_size, status, size, etag"

func (s *Store) create(ctx context.Context, user User, record Record) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO uploads (object_key, owner, filename, expected_size, status)
		VALUES (?, ?, ?, ?, 'pending')
	`, record.Key, user.ID.String(), record.Filename, record.ExpectedSize)
	return err
}

func (s *Store) find(ctx context.Context, user User, key uuid.UUID) (Record, error) {
	return scanRecord(s.db.QueryRowContext(ctx, `SELECT `+recordColumns+`
		FROM uploads WHERE owner = ? AND object_key = ?
	`, user.ID.String(), key.String()))
}

func (s *Store) finish(ctx context.Context, user User, key uuid.UUID, record Record) (Record, error) {
	saved, err := scanRecord(s.db.QueryRowContext(ctx, `
		UPDATE uploads SET status = ?, size = ?, etag = ?
		WHERE owner = ? AND object_key = ? AND status = 'pending'
		RETURNING `+recordColumns,
		record.Status, record.Size, record.ETag, user.ID.String(), key.String()))
	if errors.Is(err, sql.ErrNoRows) {
		return s.find(ctx, user, key)
	}
	return saved, err
}

func (s *Store) list(ctx context.Context, user User) ([]Record, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+recordColumns+`
		FROM uploads WHERE owner = ? AND status = 'verified' ORDER BY id DESC
	`, user.ID.String())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var records []Record
	for rows.Next() {
		record, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func scanRecord(row interface{ Scan(...any) error }) (Record, error) {
	var record Record
	err := row.Scan(&record.Key, &record.Filename, &record.ExpectedSize, &record.Status, &record.Size, &record.ETag)
	return record, err
}
