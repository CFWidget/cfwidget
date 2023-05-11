package main

import (
	"fmt"
	"github.com/cfwidget/cfwidget/env"
	"github.com/cfwidget/cfwidget/widget"
	"github.com/go-gormigrate/gormigrate/v2"
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

		log.Printf("Starting migrations")
		migrator := gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
			{
				ID: "1682972228",
				Migrate: func(g *gorm.DB) (err error) {
					if !g.Migrator().HasTable("projects") {
						err = g.AutoMigrate(&widget.Project{}, &widget.Author{}, &widget.ProjectLookup{})
						return
					}

					//move old tables away, because they are now considered dead
					err = g.Migrator().RenameTable("projects", "old_projects")
					if err != nil {
						return
					}
					err = g.Migrator().RenameTable("authors", "old_authors")
					if err != nil {
						return
					}

					err = g.AutoMigrate(&widget.Project{}, &widget.Author{}, &widget.ProjectLookup{})
					if err != nil {
						return
					}

					//insert our missing data
					err = g.Exec("INSERT INTO authors (member_id, username, properties, created_at, updated_at) SELECT member_id, username, properties, created_at, updated_at FROM old_authors").Error
					if err != nil {
						return
					}

					err = g.Exec("INSERT INTO projects (id) SELECT DISTINCT curse_id FROM old_projects WHERE properties IS NOT NULL AND STATUS IN (200, 403) AND curse_id IS NOT NULL").Error
					if err != nil {
						return
					}

					err = g.Exec("INSERT INTO project_lookups (path, curse_id) SELECT DISTINCT path, curse_id FROM old_projects").Error
					if err != nil {
						return
					}

					err = g.Exec("UPDATE projects p SET properties = (SELECT properties FROM old_projects op WHERE op.curse_id = p.id AND op.properties IS NOT NULL AND op.STATUS IN (200, 403) ORDER BY id LIMIT 1), STATUS = (SELECT STATUS FROM old_projects op WHERE op.curse_id = p.id AND op.properties IS NOT NULL AND op.STATUS IN (200, 403) ORDER BY id LIMIT 1)").Error
					if err != nil {
						return
					}

					return
				},
				Rollback: func(g *gorm.DB) error {
					//roll back table names, as that is okay
					_ = g.Migrator().DropTable("projects")
					_ = g.Migrator().DropTable("authors")
					_ = g.Migrator().DropTable("project_lookups")
					_ = g.Migrator().RenameTable("old_projects", "projects")
					_ = g.Migrator().RenameTable("authors", "authors")
					return nil
				},
			},
		})

		err = migrator.Migrate()
		if err != nil {
			log.Printf("Error connecting to database: %s", err.Error())
			return nil, err
		}
		log.Printf("Migrations complete")

		_db = db
	}

	return _db, nil
}
