package db

import (
	"database/sql"
	"fmt"

	"github.com/sirupsen/logrus"
)

type DbClient struct {
	db *sql.DB
}

// 创建表
func createTable(db *sql.DB) error {
	createTableSQL := `
        CREATE TABLE IF NOT EXISTS domains (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            original TEXT,
            target TEXT
        );
    `
	_, err := db.Exec(createTableSQL)
	if err != nil {
		return fmt.Errorf("无法创建表：", err)
	}
	logrus.Info("表创建成功")
	return nil
}

func NewDbClient() (*DbClient, error) {
	db, err := sql.Open("sqlite3", "./database.db")
	if err != nil {
		return nil, fmt.Errorf("无法打开数据库连接：", err)
	}

	client := &DbClient{
		db: db,
	}
	client.Init()
	return client, nil
}

func executeDatabaseQuery(db *sql.DB, query string, args ...interface{}) error {
	stmt, err := db.Prepare(query)
	if err != nil {
		return fmt.Errorf("failed to prepare query: %v", err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(args...)
	if err != nil {
		return fmt.Errorf("failed to execute query: %v", err)
	}

	return nil
}

func queryDatabase(db *sql.DB, query string, args ...interface{}) (*sql.Row, error) {
	stmt, err := db.Prepare(query)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare query: %v", err)
	}
	defer stmt.Close()

	return stmt.QueryRow(args...), nil
}

func (client *DbClient) UpdateDomain(original string, target string) error {
	insertDataSQL := "INSERT INTO domains (original, target) VALUES (?, ?);"

	err := executeDatabaseQuery(client.db, insertDataSQL, original, target)
	if err != nil {
		return fmt.Errorf("can't update domain, err: %v", err)
	}
	return nil
}

func (client *DbClient) GetOriginalUrl(hash string) (string, error) {
	var original string
	queryDataSQL := "SELECT original FROM domains WHERE target = ?"

	row, err := queryDatabase(client.db, queryDataSQL, hash)
	if err != nil {
		return "", err
	}

	err = row.Scan(&original)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("cannot find original domain of %s", hash)
		}
		return "", fmt.Errorf("query failed: %v", err)
	}

	return original, nil
}

func (client *DbClient) GetShortedHash(originalUrl string) (string, error) {
	var hash string
	queryDataSQL := "SELECT target FROM domains WHERE original = ?"

	row, err := queryDatabase(client.db, queryDataSQL, originalUrl)
	if err != nil {
		return "", err
	}

	err = row.Scan(&hash)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("cannot find shorted hash of %s", originalUrl)
		}
		return "", fmt.Errorf("query failed: %v", err)
	}

	return hash, nil
}

func (client *DbClient) Init() error {
	return createTable(client.db)
}

func (client *DbClient) Close() error {
	return client.db.Close()
}
