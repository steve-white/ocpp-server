package db

import (
	"database/sql"
	"errors"
	"time"

	log "sw/ocpp/csms/internal/logging"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/mattn/go-sqlite3"
)

var (
	//ctx context.Context
	db *sql.DB
)

func ConnectDb(dbType string, connStr string) error {
	var err error

	db, err = sql.Open(dbType, connStr)

	if err != nil {
		return err
	}

	if err = db.Ping(); err != nil {
		return err
	}
	db.SetConnMaxLifetime(time.Minute * 2)
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(5)

	log.Logger.Info("Connected to: " + dbType)
	return nil
}

func Disconnect() {
	if db != nil {
		db.Close()
	}
}

func CreateDeviceTables() error {
	sql := `
	CREATE TABLE IF NOT EXISTS devices(
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		tenant TEXT NOT NULL,
		guid TEXT NOT NULL,
		networkid TEXT NOT NULL,
		devicetemplateid INTEGER NOT NULL
	);
	`
	_, err := db.Exec(sql)
	if err != nil {
		return err
	}

	sql = `CREATE INDEX IF NOT EXISTS devices_tenant_networkid_IDX ON devices (tenant, networkid);`
	_, err = db.Exec(sql)
	if err != nil {
		return err
	}

	return nil
}

func CreateTables() error {
	sql := `
	CREATE TABLE IF NOT EXISTS transactions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		guid TEXT NOT NULL,
		clientId TEXT NOT NULL,
		timeStarted INTEGER NOT NULL,
		timeEnded INTEGER NULL,
		meterStop FLOAT NULL
	);
	`
	_, err := db.Exec(sql)
	if err != nil {
		return err
	}

	sql = `CREATE INDEX IF NOT EXISTS transaction_clientId_IDX ON transactions (clientId);`
	_, err = db.Exec(sql)
	if err != nil {
		return err
	}

	return nil
}

// Transaction: Id, Guid, ClientId, TimeStarted, TimeEnded, MeterStop
func InsertNextTransaction(clientId string, timeStarted time.Time) (*int64, error) {
	guid := uuid.New().String()
	res, err := db.Exec("INSERT INTO transactions(guid,clientId,timeStarted) VALUES (?,?,?) ", guid, clientId, timeStarted.UnixMilli())
	if err != nil {
		var sqliteErr sqlite3.Error
		if errors.As(err, &sqliteErr) {
			if errors.Is(sqliteErr.ExtendedCode, sqlite3.ErrConstraintUnique) {
				return nil, errors.New("row already exists")
			}
		}
		return nil, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return &id, nil
}
