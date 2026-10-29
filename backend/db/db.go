package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"go-stock/backend/apppath"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var Dao *gorm.DB

// dbFilePath 实际使用的数据库文件路径（无 DSN 参数），供 VACUUM 等维护操作定位文件
var dbFilePath string

// vacuumRunning 防止并发触发 VACUUM：VACUUM 全程独占写锁，并发执行只会相互等待并失败
var vacuumRunning atomic.Bool

// resolveDBPath 去掉 DSN 查询参数只保留文件路径；空值取默认路径。
func resolveDBPath(path string) string {
	if path == "" {
		path = filepath.ToSlash(filepath.Join(apppath.DataDir(), "stock.db"))
	}
	if i := strings.Index(path, "?"); i >= 0 {
		path = path[:i]
	}
	return path
}

// defaultDBPath 默认数据库文件路径（无 DSN 参数）。
func defaultDBPath() string {
	return filepath.ToSlash(filepath.Join(apppath.DataDir(), "stock.db"))
}

// dsnForPath 为裸文件路径拼装完整 DSN。
// busy_timeout 等为连接级参数，必须走 DSN 才能对连接池中每个新建连接生效。
func dsnForPath(path string) string {
	dsn := path + "?_busy_timeout=10000&_journal_mode=WAL&_synchronous=NORMAL&_cache_size=-524288"
	// WAL 下 deferred（驱动默认）事务若"先读后写"，升级写锁时会立即返回 busy 而不排队等待，
	// 造成偶发写失败；immediate 在 BEGIN 时就取写锁，把冲突交给 busy_timeout 排队。
	// 项目现有事务均为"写在前"，此项属防御性配置，也覆盖未来可能新增的"先查后写"事务。
	if !strings.Contains(dsn, "_txlock") {
		dsn += "&_txlock=immediate"
	}
	return dsn
}

// applyPragmas 兜底确保 busy_timeout / WAL / synchronous 生效
//（不同驱动/DSN 参数支持可能存在差异）。
func applyPragmas(d *gorm.DB) {
	_ = d.Exec("PRAGMA busy_timeout=10000").Error
	_ = d.Exec("PRAGMA journal_mode=WAL").Error
	_ = d.Exec("PRAGMA synchronous=NORMAL").Error
}

// configurePool 设置连接池：SQLite 写入是串行锁模型，连接开太多会放大锁竞争导致 SQLITE_BUSY。
func configurePool(d *gorm.DB) error {
	sqlDB, err := d.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetMaxOpenConns(5)
	sqlDB.SetConnMaxLifetime(time.Hour)
	return nil
}

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
	if sqlitePath == "" {
		sqlitePath = defaultDBPath()
	}
	openDb, err := gorm.Open(sqlite.Open(dsnForPath(sqlitePath)), &gorm.Config{
		Logger:                                   dbLogger,
		DisableForeignKeyConstraintWhenMigrating: true,
		SkipDefaultTransaction:                   true,
		PrepareStmt:                              true,
	})

	if err != nil {
		log.Fatalf("db connection error is %s", err.Error())
	}
	dbFilePath = resolveDBPath(sqlitePath)
	applyPragmas(openDb)
	if err := configurePool(openDb); err != nil {
		log.Fatalf("openDb.DB error is  %s", err.Error())
	}
	Dao = openDb
	AutoMigrate()
}

// VacuumResult 数据库压缩结果，字节数便于前端自行换算展示
type VacuumResult struct {
	FilePath    string  `json:"filePath"`    // 数据库文件路径
	BeforeBytes int64   `json:"beforeBytes"` // 压缩前文件大小
	AfterBytes  int64   `json:"afterBytes"`  // 压缩后文件大小
	FreedBytes  int64   `json:"freedBytes"`  // 释放的空间
	DurationSec float64 `json:"durationSec"` // 本次耗时（秒）
}

// Vacuum 执行 SQLite VACUUM：重建数据库文件，回收 DELETE/清理后未归还操作系统的空闲页。
//
// 为什么需要：SQLite 的 DELETE 只把页面标记为空闲并放入空闲链表，文件大小不会缩小。
// 板块/概念资金流向、快讯等高频写入表长期清理后，文件会远大于实际数据量，
// 查询要扫描的页面数、磁盘 IO 与 WAL checkpoint 耗时都会成倍增加，进而拖慢其它查询。
//
// 调用方需注意：
//  1. VACUUM 全程独占写锁，期间所有写入排队等待，务必在非交易时段执行；
//  2. 需要约等于当前库大小的临时磁盘空间（内部会重建出一份完整副本），空间不足会失败；
//  3. 耗时与库大小成正比，数 GB 的库可能持续数分钟。
func Vacuum() (*VacuumResult, error) {
	if !vacuumRunning.CompareAndSwap(false, true) {
		return nil, fmt.Errorf("数据库压缩正在进行中，请等待本次执行完成")
	}
	defer vacuumRunning.Store(false)

	if Dao == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}
	path := dbFilePath
	if path == "" {
		return nil, fmt.Errorf("数据库文件路径未知")
	}
	start := time.Now()

	// 关键前置步骤：排空连接池中的空闲连接。
	// SQLite 只有在没有其它连接持读标记时，checkpoint 才能截断主库文件；
	// 池中空闲连接会持有读标记，导致出现"页数已被 VACUUM 压缩、但磁盘文件大小不变"的现象。
	if sqlDB, dbErr := Dao.DB(); dbErr == nil {
		sqlDB.SetMaxIdleConns(0)
		// 给在途查询留出归还连接的时间，否则该连接仍会挡住文件截断
		time.Sleep(300 * time.Millisecond)
		defer configurePool(Dao)
	}

	// 用独立连接执行，绕开 GORM 的预编译语句缓存：
	// VACUUM 会重建整个库，连接上已缓存的语句会全部失效。
	conn, err := sql.Open("sqlite", dsnForPath(path))
	if err != nil {
		return nil, fmt.Errorf("打开数据库连接失败: %w", err)
	}
	defer func() { _ = conn.Close() }()
	conn.SetMaxOpenConns(1)

	// 先把 WAL 内容合并回主库：一是让大小统计真实反映数据量，
	// 二是避免后续 VACUUM 与未合并的 WAL 争抢资源。
	if _, err := conn.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		log.Printf("vacuum: pre wal_checkpoint failed: %v", err)
	}

	before := fileSizeOrZero(path)
	// 覆盖 DSN 中的 busy_timeout(10s)：压缩期间定时采集等写入者会排队，
	// 需要更长的等待窗口，否则 VACUUM 容易因取不到锁而失败。
	if _, err := conn.Exec("PRAGMA busy_timeout=120000"); err != nil {
		log.Printf("vacuum: set busy_timeout failed: %v", err)
	}

	if _, err := conn.Exec("VACUUM"); err != nil {
		return nil, fmt.Errorf("VACUUM 执行失败（可能有写入尚未释放，或磁盘空间不足）: %w", err)
	}
	// WAL 模式下 VACUUM 重建后的页面先写入 WAL，主库文件的收缩要到 checkpoint 时才落地，
	// 因此必须再 checkpoint 一次，文件大小才真正反映压缩结果（否则会看到"没变小"甚至变大）。
	if _, err := conn.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		log.Printf("vacuum: post wal_checkpoint failed: %v", err)
	}

	after := fileSizeOrZero(path)
	return &VacuumResult{
		FilePath:    path,
		BeforeBytes: before,
		AfterBytes:  after,
		FreedBytes:  before - after,
		DurationSec: time.Since(start).Seconds(),
	}, nil
}

func fileSizeOrZero(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return fi.Size()
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
