package util

import (
	"bbutil_cli/common"
	"fmt"
	"time"

	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var db *gorm.DB

const (
	SQL_LEVEL_SILENT = "Slient"
	SQL_LEVEL_ERROR  = "Error"
	SQL_LEVEL_WARN   = "Warn"
	SQL_LEVEL_INFO   = "Info"
)

func DatabaseInit() {
	common.Logger.Info("start database pool init")
	databaseType := viper.GetString("dataSource.type")
	databaseLevel := viper.GetString("dataSource.level")
	var sqlLevel = 0
	if databaseLevel == SQL_LEVEL_SILENT {
		sqlLevel = 1
	} else if databaseLevel == SQL_LEVEL_ERROR {
		sqlLevel = 2
	} else if databaseLevel == SQL_LEVEL_WARN {
		sqlLevel = 3
	} else if databaseLevel == SQL_LEVEL_INFO {
		sqlLevel = 4
	} else {
		common.Logger.Fatalf("The sql log level named %s does not exist", databaseLevel)
	}
	common.Logger.Infof("sql log level is %s", databaseLevel)

	// mysql
	if databaseType == "mysql" {

		databaseName := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=True&charset=utf8mb4&loc=Local",
			viper.GetString("dataSource.username"),
			viper.GetString("dataSource.password"),
			viper.GetString("dataSource.host"),
			viper.GetInt("dataSource.port"),
			viper.GetString("dataSource.database"),
		)
		conn, err := gorm.Open(mysql.Open(databaseName), &gorm.Config{
			Logger:                 logger.Default.LogMode(logger.LogLevel(sqlLevel)),
			SkipDefaultTransaction: true,
		})
		if err != nil {
			panic(fmt.Errorf("database source type: %s init err: %s", "mysql", err))
		}
		pool, err := conn.DB()
		if err != nil {
			panic(fmt.Errorf("database pool type: %s init err: %s", "mysql", err))
		}
		pool.SetMaxIdleConns(viper.GetInt("databasePool.maxIdleConns"))
		pool.SetMaxOpenConns(viper.GetInt("databasePool.maxOpenConns"))
		connMaxLifeTime := viper.GetInt("databasePool.connMaxLifetime")
		pool.SetConnMaxLifetime(time.Second * time.Duration(connMaxLifeTime))
		db = conn
	} else if databaseType == "postgres" {
		databaseName := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=disable  TimeZone=Asia/Shanghai",
			viper.GetString("dataSource.host"),
			viper.GetString("dataSource.username"),
			viper.GetString("dataSource.password"),
			viper.GetString("dataSource.database"),
			viper.GetInt("dataSource.port"),
		)
		conn, err := gorm.Open(postgres.Open(databaseName), &gorm.Config{
			Logger:                 logger.Default.LogMode(logger.LogLevel(sqlLevel)),
			SkipDefaultTransaction: true,
		})
		if err != nil {
			panic(fmt.Errorf("database source type: %s init err: %s", "postgres", err))
		}
		pool, err := conn.DB()
		if err != nil {
			panic(fmt.Errorf("database pool type: %s init err: %s", "postgres", err))
		}
		pool.SetMaxIdleConns(viper.GetInt("databasePool.maxIdleConns"))
		pool.SetMaxOpenConns(viper.GetInt("databasePool.maxOpenConns"))
		connMaxLifeTime := viper.GetInt("databasePool.connMaxLifetime")
		pool.SetConnMaxLifetime(time.Second * time.Duration(connMaxLifeTime))
		db = conn
	} else {
		databaseName := viper.GetString("dataSource.database")
		conn, err := gorm.Open(sqlite.Open(databaseName), &gorm.Config{
			Logger:                 logger.Default.LogMode(logger.LogLevel(sqlLevel)),
			SkipDefaultTransaction: true,
		})
		if err != nil {
			panic(fmt.Errorf("database source type: %s init err: %s", "sqlite", err))
		}
		pool, err := conn.DB()
		if err != nil {
			panic(fmt.Errorf("database pool type: %s init err: %s", "sqlite", err))
		}
		pool.SetMaxIdleConns(viper.GetInt("databasePool.maxIdleConns"))
		pool.SetMaxOpenConns(viper.GetInt("databasePool.maxOpenConns"))
		connMaxLifeTime := viper.GetInt("databasePool.connMaxLifetime")
		pool.SetConnMaxLifetime(time.Second * time.Duration(connMaxLifeTime))
		db = conn
	}
	common.Logger.Info("success database pool init")

}

func GetDB() *gorm.DB {
	sqlDB, err := db.DB()
	if err != nil {
		DatabaseInit()
	}

	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		DatabaseInit()
	}

	return db
}
