package database

import (
	"github.com/jinzhu/gorm"
	"github.com/spf13/viper"
)

var dbConn *gorm.DB

func GetConnection() (*gorm.DB, error) {
	if dbConn != nil {
		err := dbConn.DB().Ping()
		//ping failed, so this connection as a whole is just shot
		if err == nil {
			return dbConn, nil
		}
	}

	connString := viper.GetString("database")

	var err error
	dbConn, err = gorm.Open("mysql", connString)
	return dbConn, err
}
