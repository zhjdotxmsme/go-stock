package db

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"go-stock/backend/apppath"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var Dao *gorm.DB

func Init(sqlitePath string) {
	dbLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second * 3,
			Colorful:                  false,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      false,
			LogLevel:                  logger.Silent,
		},
	)
	var openDb *gorm.DB
	var err error
	if sqlitePath == "" {
		sqlitePath = filepath.ToSlash(filepath.Join(apppath.DataDir(), "stock.db")) +
			"?_busy_timeout=10000&_journal_mode=WAL&_synchronous=NORMAL&_cache_size=-524288"
	}
	openDb, err = gorm.Open(sqlite.Open(sqlitePath), &gorm.Config{
		Logger:                                   dbLogger,
		DisableForeignKeyConstraintWhenMigrating: true,
		SkipDefaultTransaction:                   true,
		PrepareStmt:                              true,
	})

	if err != nil {
		log.Fatalf("db connection error is %s", err.Error())
	}

	// 兜底：确保 busy_timeout / WAL / synchronous 生效（不同驱动/DSN 参数支持可能存在差异）
	_ = openDb.Exec("PRAGMA busy_timeout=10000").Error
	_ = openDb.Exec("PRAGMA journal_mode=WAL").Error
	_ = openDb.Exec("PRAGMA synchronous=NORMAL").Error

	dbCon, err := openDb.DB()
	if err != nil {
		log.Fatalf("openDb.DB error is  %s", err.Error())
	}
	// SQLite 写入是串行锁模型：连接开太多会放大锁竞争导致 SQLITE_BUSY
	dbCon.SetMaxIdleConns(1)
	dbCon.SetMaxOpenConns(5)
	dbCon.SetConnMaxLifetime(time.Hour)
	Dao = openDb
	AutoMigrate()
}

// Close 优雅关闭数据库：先 TRUNCATE checkpoint 合并 WAL 并截断，再关闭连接池。
// timeout 限制总时长（checkpoint 可能需要把 GB 级 WAL 写回主库），超时后放弃等待，
// 由调用方随后的 os.Exit / 进程退出兜底（SQLite checkpoint 本身崩溃安全）。
//
// 背景：直接 os.Exit(0) 跳过收尾会留下数 GB 的脏 WAL，新进程启动时要全量扫描
// 恢复（纯 Go 驱动），卡在 db.Init 数十秒无窗口。
func Close(timeout time.Duration) {
	if Dao == nil {
		return
	}
	sqlDB, err := Dao.DB()
	if err != nil {
		return
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		// 走原生 *sql.DB 执行，绕过 GORM 预编译语句缓存
		if _, err := sqlDB.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
			log.Printf("db close: wal_checkpoint: %v", err)
		}
		if err := sqlDB.Close(); err != nil {
			log.Printf("db close: %v", err)
		}
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		log.Printf("db close: timeout after %v, skip graceful close", timeout)
	}
}
