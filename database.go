package main

import (
	"fmt"
	"github.com/lordralex/cfwidget/widget"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"log"
	"os"
	"sync"
	"time"
)

var _db *gorm.DB
var locker sync.Mutex

func GetDatabase() (*gorm.DB, error) {
	if _db == nil {
		locker.Lock()
		defer locker.Unlock()

		if _db != nil {
			return _db, nil
		}

		dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", os.Getenv("DB_USER"), os.Getenv("DB_PASS"), os.Getenv("DB_HOST"), os.Getenv("DB_DATABASE"))

		log.Printf("Connecting to database: %s\n", os.Getenv("DB_HOST"))
		db, err := gorm.Open(mysql.Open(dsn))
		if err != nil {
			log.Printf("Error connecting to database: %s", err.Error())
			return nil, err
		}
		sqlDB, err := db.DB()
		if err != nil {
			log.Printf("Error connecting to database: %s", err.Error())
			return nil, err
		}
		sqlDB.SetMaxIdleConns(10)
		sqlDB.SetMaxOpenConns(100)
		sqlDB.SetConnMaxLifetime(time.Hour)

		if os.Getenv("DEBUG") == "true" {
			db = db.Debug()
		}

		err = db.AutoMigrate(&widget.Project{}, &widget.Author{})
		if err != nil {
			log.Printf("Error connecting to database: %s", err.Error())
			return nil, err
		}
		_db = db
	}

	return _db, nil
}