// Package apppath 统一解析运行时可写目录。
//
// 背景：数据库与日志曾使用相对于进程工作目录(cwd)的路径。从快捷方式（"起始位置"
// 非程序目录）、计划任务或其他目录启动进程时，数据会散落到错误位置，甚至因
// 目录无法创建而启动失败（macOS LaunchServices 场景下 cwd 固定为只读的 /）。
//
// 解析优先级（"老位置优先"，老用户数据路径逐字不变）：
//  1. 环境变量 GO_STOCK_ROOT_DIR 显式覆盖（测试或指定部署目录）；
//  2. 可执行文件所在目录——Windows 双击启动时与旧的 cwd 相对行为等价，零迁移；
//  3. 可执行文件路径获取失败时降级到当前工作目录。
package apppath

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	baseOnce sync.Once
	baseDir  string

	dataOnce sync.Once
	dataDir  string

	logsOnce sync.Once
	logsDir  string
)

// BaseDir 返回运行时数据根目录（不带尾分隔符）。
func BaseDir() string {
	baseOnce.Do(func() {
		if env := strings.TrimSpace(os.Getenv("GO_STOCK_ROOT_DIR")); env != "" {
			baseDir = env
			return
		}
		if exePath, err := os.Executable(); err == nil && exePath != "" {
			baseDir = filepath.Dir(exePath)
			return
		}
		wd, err := os.Getwd()
		if err != nil || wd == "" {
			wd = "."
		}
		baseDir = wd
	})
	return baseDir
}

// DataDir 返回数据目录（<BaseDir>/data），已确保存在；创建失败时退回 BaseDir。
func DataDir() string {
	dataOnce.Do(func() { dataDir = ensureDir("data") })
	return dataDir
}

// LogsDir 返回日志目录（<BaseDir>/logs），已确保存在；创建失败时退回 BaseDir。
func LogsDir() string {
	logsOnce.Do(func() { logsDir = ensureDir("logs") })
	return logsDir
}

func ensureDir(name string) string {
	dir := filepath.Join(BaseDir(), name)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		// 目录创建失败无法在本包记日志（logger 依赖本包，防循环导入），
		// 退回基础目录保证路径可用；真实错误由调用方的写失败/GORM 报错暴露。
		return BaseDir()
	}
	return dir
}
