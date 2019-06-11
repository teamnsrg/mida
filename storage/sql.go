package storage

import (
	"errors"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"github.com/teamnsrg/mida/jstrace"
)

type SqlCall struct {
	gorm.Model

	CrawlID   int `gorm:"not null"`
	IsolateID int `gorm:"not null"`
	ScriptID  int `gorm: not null`
}

func CreatePostgresConnection(host string, port string, user string, pass string, dbName string) (*gorm.DB, error) {
	db, err := gorm.Open("postgres",
		"host="+host+
			" port="+port+
			" user="+user+
			" dbname="+dbName+
			" password="+pass)
	if err != nil {
		return db, nil
	} else {
		return nil, err
	}
}

func InitializeNewDatabase(name string, db *gorm.DB) (*gorm.DB, error) {

	db = db.Exec("CREATE DATABASE " + name + ";")
	if db.Error != nil {
		return db, errors.New("error creating new database")
	}

	db.CreateTable()

	return db, nil
}

func ClosePostgresConnection(db *gorm.DB) error {
	return db.Close()
}

func StoreJSTraceToDB(db *gorm.DB, trace jstrace.JSTrace) error {
	return nil
}
