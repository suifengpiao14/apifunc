package provider

import (
	"database/sql"
	"encoding/json"
)

type DBProvider struct {
	_db *sql.DB
}

func (dbProvider DBProvider) TypeName() string {
	return "DB_Provider"
}

func (dbProvider DBProvider) GetDB() (db *sql.DB) {
	return dbProvider._db
}

type DBConfig struct {
	DSN string `json:"dsn"`
}

var DriverName = "mysql"

func NewDBProvider(config string) (dbProvider *DBProvider, err error) {

	cfg := &DBConfig{}
	err = json.Unmarshal([]byte(config), cfg)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open(DriverName, cfg.DSN)
	if err != nil {
		return nil, err
	}
	dbProvider = &DBProvider{
		_db: db,
	}

	return dbProvider, nil
}
