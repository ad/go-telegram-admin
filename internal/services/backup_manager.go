package services

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/ad/go-telegram-admin/internal/db"
	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
)

type BackupManager struct {
	bot    *bot.Bot
	dbPath string
	queue  *db.DBQueue
}

func NewBackupManager(b *bot.Bot, dbPath string, queue *db.DBQueue) *BackupManager {
	return &BackupManager{
		bot:    b,
		dbPath: dbPath,
		queue:  queue,
	}
}

func (bm *BackupManager) CreateBackup() (string, error) {
	if bm.dbPath == ":memory:" || strings.HasPrefix(bm.dbPath, "file::memory:") {
		sqlDump, err := bm.GenerateSQLDumpGo(bm.dbPath)
		if err != nil {
			return "", fmt.Errorf("failed to create backup: %w", err)
		}
		return sqlDump, nil
	}

	sqlDump, err := bm.GenerateSQLDumpCLI(bm.dbPath)
	if err != nil {
		sqlDump, err = bm.GenerateSQLDumpGo(bm.dbPath)
		if err != nil {
			return "", fmt.Errorf("failed to create backup: %w", err)
		}
	}
	return sqlDump, nil
}

func (bm *BackupManager) GenerateSQLDumpCLI(dbPath string) (string, error) {
	cmd := exec.Command("sqlite3", dbPath, ".dump")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("sqlite3 command failed: %w", err)
	}
	return string(output), nil
}

func (bm *BackupManager) GenerateSQLDumpGo(dbPath string) (string, error) {
	var dump strings.Builder

	dump.WriteString("BEGIN TRANSACTION;\n")

	rows, err := bm.queue.DB().Query("SELECT name, sql FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		return "", fmt.Errorf("failed to query tables: %w", err)
	}
	defer rows.Close()

	var tables []struct {
		name string
		sql  string
	}

	for rows.Next() {
		var name, createSQL string
		if err := rows.Scan(&name, &createSQL); err != nil {
			return "", fmt.Errorf("failed to scan table info: %w", err)
		}
		tables = append(tables, struct {
			name string
			sql  string
		}{name, createSQL})
	}

	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("error iterating tables: %w", err)
	}

	for _, table := range tables {
		dump.WriteString(table.sql)
		dump.WriteString(";\n")

		dataRows, err := bm.queue.DB().Query(fmt.Sprintf("SELECT * FROM %s", table.name))
		if err != nil {
			return "", fmt.Errorf("failed to query table %s: %w", table.name, err)
		}

		columns, err := dataRows.Columns()
		if err != nil {
			dataRows.Close()
			return "", fmt.Errorf("failed to get columns for table %s: %w", table.name, err)
		}

		for dataRows.Next() {
			values := make([]interface{}, len(columns))
			valuePtrs := make([]interface{}, len(columns))
			for i := range values {
				valuePtrs[i] = &values[i]
			}

			if err := dataRows.Scan(valuePtrs...); err != nil {
				dataRows.Close()
				return "", fmt.Errorf("failed to scan row in table %s: %w", table.name, err)
			}

			dump.WriteString(fmt.Sprintf("INSERT INTO %s VALUES (", table.name))
			for i, val := range values {
				if i > 0 {
					dump.WriteString(", ")
				}
				if val == nil {
					dump.WriteString("NULL")
				} else {
					switch v := val.(type) {
					case []byte:
						dump.WriteString(fmt.Sprintf("'%s'", strings.ReplaceAll(string(v), "'", "''")))
					case string:
						dump.WriteString(fmt.Sprintf("'%s'", strings.ReplaceAll(v, "'", "''")))
					case int64, float64, bool:
						dump.WriteString(fmt.Sprintf("%v", v))
					default:
						dump.WriteString(fmt.Sprintf("'%v'", v))
					}
				}
			}
			dump.WriteString(");\n")
		}

		dataRows.Close()

		if err := dataRows.Err(); err != nil {
			return "", fmt.Errorf("error iterating rows in table %s: %w", table.name, err)
		}
	}

	dump.WriteString("COMMIT;\n")

	return dump.String(), nil
}

func (bm *BackupManager) SendBackupToAdmin(ctx context.Context, adminID int64, sqlDump string) error {
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("backup_%s.sql", timestamp)

	tmpFile, err := os.CreateTemp("", filename)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(sqlDump); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write backup to file: %w", err)
	}
	tmpFile.Close()

	file, err := os.Open(tmpFile.Name())
	if err != nil {
		return fmt.Errorf("failed to open backup file: %w", err)
	}
	defer file.Close()

	_, err = bm.bot.SendDocument(ctx, &bot.SendDocumentParams{
		ChatID: adminID,
		Document: &tgmodels.InputFileUpload{
			Filename: filename,
			Data:     file,
		},
		Caption: fmt.Sprintf("✅ Бэкап базы данных создан: %s", time.Now().Format("2006-01-02 15:04:05")),
	})

	if err != nil {
		return fmt.Errorf("failed to send backup file: %w", err)
	}

	return nil
}
