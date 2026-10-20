package data

import (
	"strings"
	"sync"

	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/stockcode"
)

var stockNameCache sync.Map // normalized code -> name

// ResolveStockName 按内部规范码（sh600519 等）解析简称；失败返回空字符串。
func ResolveStockName(code string) string {
	norm := stockcode.Normalize(code)
	if norm == "" {
		return ""
	}
	if v, ok := stockNameCache.Load(norm); ok {
		return v.(string)
	}
	name := lookupStockNameOnce(norm)
	if name != "" {
		stockNameCache.Store(norm, name)
	}
	return name
}

// ResolveStockNames 批量解析，返回 map[normalizedCode]name。
func ResolveStockNames(codes []string) map[string]string {
	out := make(map[string]string)
	need := make([]string, 0, len(codes))
	seen := map[string]struct{}{}
	for _, c := range codes {
		n := stockcode.Normalize(c)
		if n == "" {
			continue
		}
		if v, ok := stockNameCache.Load(n); ok {
			out[n] = v.(string)
			continue
		}
		if _, ok := seen[n]; !ok {
			seen[n] = struct{}{}
			need = append(need, n)
		}
	}
	if len(need) == 0 {
		return out
	}
	batch := lookupStockNamesBatch(need)
	for k, v := range batch {
		out[k] = v
		if v != "" {
			stockNameCache.Store(k, v)
		}
	}
	return out
}

func lookupStockNameOnce(norm string) string {
	m := lookupStockNamesBatch([]string{norm})
	return m[norm]
}

func lookupStockNamesBatch(normalized []string) map[string]string {
	out := make(map[string]string, len(normalized))
	if len(normalized) == 0 || db.Dao == nil {
		return out
	}

	tsSet := make(map[string]struct{})
	tySet := make(map[string]struct{})
	normByTS := map[string]string{}
	normByTy := map[string]string{}
	for _, norm := range normalized {
		ts := strings.ToUpper(stockcode.ToTushare(norm))
		if ts != "" {
			tsSet[ts] = struct{}{}
			normByTS[ts] = norm
		}
		for _, c := range stockcode.StockCodeCandidates(norm) {
			tySet[c] = struct{}{}
			normByTy[c] = norm
		}
	}

	tsList := keysOf(tsSet)
	if len(tsList) > 0 {
		var basics []StockBasic
		_ = db.Dao.Where("ts_code IN ?", tsList).Find(&basics).Error
		for _, b := range basics {
			if b.Name == "" {
				continue
			}
			ts := strings.ToUpper(b.TsCode)
			if norm, ok := normByTS[ts]; ok {
				out[norm] = b.Name
			}
		}
	}

	tyList := keysOf(tySet)
	if len(tyList) > 0 {
		var infos []models.AllStockInfo
		_ = db.Dao.Where("sec_uri_tycode IN ? OR secucode IN ?", tyList, tsList).Find(&infos).Error
		for _, info := range infos {
			name := strings.TrimSpace(info.SECURITYNAMEABBR)
			if name == "" {
				continue
			}
			for _, key := range []string{info.SECURITYCODE, info.SECUCODE} {
				key = strings.TrimSpace(key)
				if key == "" {
					continue
				}
				if norm, ok := normByTy[key]; ok && out[norm] == "" {
					out[norm] = name
				}
				n := stockcode.Normalize(key)
				if n != "" && out[n] == "" {
					out[n] = name
				}
			}
		}
	}
	return out
}

func keysOf(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
