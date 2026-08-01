package freestockdb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// 板块类别（对齐 zhibiao.py CATEGORY_MAP）。
const (
	CategoryConcept = 0 // 概念
	CategorySW1     = 1 // 申万一级
	CategorySW2     = 2 // 申万二级
	CategorySW3     = 3 // 申万三级
)

var categoryNames = map[int]string{
	CategoryConcept: "概念",
	CategorySW1:     "申万一级",
	CategorySW2:     "申万二级",
	CategorySW3:     "申万三级",
}

// Board 对应服务端 "板块" 记录。
type Board struct {
	Code     string   `json:"code"`
	Name     string   `json:"name"`
	Source   string   `json:"source"`
	Type     string   `json:"type"`
	Group    string   `json:"group"`
	Category string   `json:"category"`
	Symbols  []string `json:"symbols"`
}

// BoardIndex 板块四向内存索引（对照 zhibiao.py BoardIndex）。
type BoardIndex struct {
	mu         sync.RWMutex
	byCode     map[string]*Board
	byStock    map[string][]*Board
	byName     map[string][]*Board // key = category + "_" + name
	byCategory map[string][]*Board
}

func NewBoardIndex() *BoardIndex {
	return &BoardIndex{
		byCode:     map[string]*Board{},
		byStock:    map[string][]*Board{},
		byName:     map[string][]*Board{},
		byCategory: map[string][]*Board{},
	}
}

// stockCode 统一为 6 位数字代码（去掉 .SH/.SZ 等后缀）。
func stockCode(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "."); i >= 0 {
		s = s[:i]
	}
	return s
}

// Load 拉取 "板块*" 全量并重建索引。
func (bi *BoardIndex) Load(ctx context.Context, c *Client) error {
	raw, err := c.Get(ctx, "板块*")
	if err != nil {
		return err
	}
	vals, err := decodeValues(raw)
	if err != nil {
		return err
	}
	nb := NewBoardIndex()
	for _, v := range vals {
		var b Board
		if err := json.Unmarshal(v, &b); err != nil {
			continue
		}
		if b.Code == "" || b.Name == "" || b.Category == "" {
			continue
		}
		syms := make([]string, 0, len(b.Symbols))
		for _, s := range b.Symbols {
			if sc := stockCode(s); sc != "" {
				syms = append(syms, sc)
			}
		}
		b.Symbols = syms
		bb := b
		nb.byCode[b.Code] = &bb
		nb.byName[b.Category+"_"+b.Name] = append(nb.byName[b.Category+"_"+b.Name], &bb)
		nb.byCategory[b.Category] = append(nb.byCategory[b.Category], &bb)
		for _, s := range b.Symbols {
			nb.byStock[s] = append(nb.byStock[s], &bb)
		}
	}
	bi.mu.Lock()
	bi.byCode, bi.byStock, bi.byName, bi.byCategory = nb.byCode, nb.byStock, nb.byName, nb.byCategory
	bi.mu.Unlock()
	return nil
}

// OfStock 查股票所属板块；category 传 -1 表示不限类别。
func (bi *BoardIndex) OfStock(code string, category int) []*Board {
	bi.mu.RLock()
	defer bi.mu.RUnlock()
	items := bi.byStock[stockCode(code)]
	if category < 0 {
		return items
	}
	cat := categoryNames[category]
	out := make([]*Board, 0, len(items))
	for _, b := range items {
		if b.Category == cat {
			out = append(out, b)
		}
	}
	return out
}

// SymbolsOfBoard 查板块成分股（6 位代码）。
func (bi *BoardIndex) SymbolsOfBoard(name string, category int) ([]string, error) {
	bi.mu.RLock()
	defer bi.mu.RUnlock()
	cat, ok := categoryNames[category]
	if !ok {
		return nil, fmt.Errorf("unknown category %d", category)
	}
	items := bi.byName[cat+"_"+name]
	if len(items) == 0 {
		return nil, fmt.Errorf("board %q not found in %s", name, cat)
	}
	return items[0].Symbols, nil
}

// SearchName 按名称子串模糊匹配；category 传 -1 表示不限类别。
func (bi *BoardIndex) SearchName(keyword string, category int) []*Board {
	bi.mu.RLock()
	defer bi.mu.RUnlock()
	var pool []*Board
	if category < 0 {
		for _, items := range bi.byCategory {
			pool = append(pool, items...)
		}
	} else {
		pool = bi.byCategory[categoryNames[category]]
	}
	out := make([]*Board, 0)
	for _, b := range pool {
		if strings.Contains(b.Name, keyword) {
			out = append(out, b)
		}
	}
	return out
}
