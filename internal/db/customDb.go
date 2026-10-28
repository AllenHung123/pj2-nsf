package db

import (
	"database/sql"
	"errors"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

type customDb struct {
	db *sql.DB
}

func newCustomDb(dbPath string) (*customDb, error) {
	// 開啟 SQLite 資料庫
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open SQLite: %w", err)
	}

	// 建立 services 資料表
	_, err = conn.Exec(`
        CREATE TABLE IF NOT EXISTS services (
            key TEXT PRIMARY KEY,
            value TEXT NOT NULL
        )
    `)
	if err != nil {
		closeErr := conn.Close()
		return nil, errors.Join(
			fmt.Errorf("create table: %w", err),
			closeErr,
		)
	}

	return &customDb{db: conn}, nil
}

func (c *customDb) Save(key, value string) error {
	_, err := c.db.Exec(
		"INSERT OR REPLACE INTO services (key, value) VALUES (?, ?)",
		key, value,
	)
	return err
}
func (c *customDb) Load(key string) (string, error) {
	var value string

	err := c.db.QueryRow(
		"SELECT value FROM services WHERE key = ?",
		key,
	).Scan(&value)

	if err == sql.ErrNoRows {
		return "", nil
	}

	if err != nil {
		return "", err
	}

	return value, nil
}
func (c *customDb) Exist(key string) (bool, error) {
	var count int

	err := c.db.QueryRow(
		"SELECT COUNT(*) FROM services WHERE key = ?",
		key,
	).Scan(&count)

	if err != nil {
		return false, err
	}

	return count > 0, nil
}
func (c *customDb) Release() error {
	if c.db != nil {
		return c.db.Close()
	}

	return nil
}
func (c *customDb) SaveAll(data map[string]string) error {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	for key, value := range data {
		_, err := tx.Exec(
			"INSERT OR REPLACE INTO services (key, value) VALUES (?, ?)",
			key, value,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}
func (c *customDb) LoadAll() (result map[string]string, err error) {
	result = make(map[string]string)

	rows, err := c.db.Query(
		"SELECT key, value FROM services",
	)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()

	for rows.Next() {
		var key, value string

		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}

		result[key] = value
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}
func (c *customDb) Remove(key string) error {
	_, err := c.db.Exec(
		"DELETE FROM services WHERE key = ?",
		key,
	)
	return err
}
func (c *customDb) RemoveAll() error {
	_, err := c.db.Exec(
		"DELETE FROM services",
	)
	return err
}
