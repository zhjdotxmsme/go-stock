package freestockdb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

const boardFixture = `[
	{"code":"300843.TI","name":"5G","source":"ths","type":"concept","group":"概念板块列表","category":"概念","symbols":["000016.SZ","000049.SZ","600633.SH"]},
	{"code":"801760.SL","name":"传媒","source":"sw","type":"sw_1","group":"申万行业指数列表","category":"申万一级","symbols":["600633.SH"]},
	{"code":"888888.TI","name":"5G消息","source":"ths","type":"concept","group":"概念板块列表","category":"概念","symbols":["000016.SZ"]}
]`

func loadFixture(t *testing.T) *BoardIndex {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(boardFixture))
	}))
	defer srv.Close()
	bi := NewBoardIndex()
	if err := bi.Load(context.Background(), NewClient(srv.URL[len("http://"):])); err != nil {
		t.Fatalf("Load: %v", err)
	}
	return bi
}

func TestBoardOfStock(t *testing.T) {
	bi := loadFixture(t)
	got := bi.OfStock("600633", CategoryConcept)
	if len(got) != 1 || got[0].Name != "5G" {
		t.Fatalf("OfStock concept = %+v", got)
	}
	got = bi.OfStock("600633.SH", CategorySW1) // 带后缀代码也应命中
	if len(got) != 1 || got[0].Name != "传媒" {
		t.Fatalf("OfStock sw1 = %+v", got)
	}
}

func TestBoardSymbolsOf(t *testing.T) {
	bi := loadFixture(t)
	syms, err := bi.SymbolsOfBoard("5G", CategoryConcept)
	if err != nil || len(syms) != 3 || syms[0] != "000016" {
		t.Fatalf("symbols = %v err=%v", syms, err)
	}
}

func TestBoardSearchName(t *testing.T) {
	bi := loadFixture(t)
	got := bi.SearchName("5G", CategoryConcept)
	if len(got) != 2 { // "5G" 与 "5G消息"
		t.Fatalf("search = %+v", got)
	}
}
