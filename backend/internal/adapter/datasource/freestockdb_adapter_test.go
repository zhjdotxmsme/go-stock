package datasource

import (
	"testing"

	"go-stock/backend/data/datasource/freestockdb"
	portds "go-stock/backend/internal/port/datasource"
)

// TestFreestockdbAdapterMeta 验证适配器元信息透传与端口实现。
// freestockdb.Provider 是具体结构体且依赖运行期 Manager/引擎，
// 数据方法（GetKLine/GetQuote/GetSectorData）与 Available 需引擎就绪，
// 无法在单测环境构造；此处用零值 Provider 覆盖不触碰内部状态的
// Name/Priority 透传，端口接口实现由 freestockdb_adapter.go 顶部的
// 编译期断言保证。
func TestFreestockdbAdapterMeta(t *testing.T) {
	var _ portds.SectorProvider = (*FreestockdbProvider)(nil)
	var _ portds.QuoteProvider = (*FreestockdbProvider)(nil)
	var _ portds.KLineProvider = (*FreestockdbProvider)(nil)

	a := NewFreestockdbProvider(&freestockdb.Provider{})
	if a.Name() != "freestockdb" {
		t.Errorf("Name()=%q, want freestockdb", a.Name())
	}
	if a.Priority() != 1 {
		t.Errorf("Priority()=%d, want 1", a.Priority())
	}
}
