package main

import (
	"fmt"
	"github.com/lordralex/cfwidget/env"
	"github.com/lordralex/cfwidget/widget"
	mysql "go.elastic.co/apm/module/apmgormv2/v2/driver/mysql"
	"gorm.io/gorm"
	"log"
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

		dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", env.Get("DB_USER"), env.Get("DB_PASS"), env.Get("DB_HOST"), env.Get("DB_DATABASE"))

		log.Printf("Connecting to database: %s\n", env.Get("DB_HOST"))
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

		if env.GetBool("DB_DEBUG") {
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
