// khunterreplay 狩猎场全链路四组对照回测与案例回放 CLI。
//
// 用法：
//
//	go run ./backend/cmd/khunterreplay -db data/stock.db -start 2026-01-01 -end 2026-06-30 -all-market -out result.json
//	go run ./backend/cmd/khunterreplay -db data/stock.db -start 2026-01-01 -end 2026-06-30 -cases 688322.SH,300780.SZ,600259.SH,300829.SZ,300969.SZ -variants full -out -
//
// 模式：
//
//	-all-market：本地全市场可研究清单（models.AllStockInfo），不使用当前狩猎池；
//	默认（不带 -all-market）：以 -cases 清单为回测股票池（显式小清单，案例回放覆盖全部代码）。
//
// 输出：FullBacktestResult JSON（四组变体报告 + 守恒计数 + 案例逐日观察/风险/确认/入场/保护/退出轨迹）。
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"go-stock/backend/data/khunter"
	"go-stock/backend/db"
)

func main() {
	dbPath := flag.String("db", "", "SQLite 数据库路径（默认取应用默认库）")
	start := flag.String("start", "", "回测开始日期 YYYY-MM-DD（必填）")
	end := flag.String("end", "", "回测结束日期 YYYY-MM-DD（必填）")
	allMarket := flag.Bool("all-market", false, "使用本地全市场可研究清单（默认使用 -cases 清单）")
	variantsFlag := flag.String("variants", "", "逗号分隔变体子集：baseline,data_only,risk,full（默认全部四组）")
	configPath := flag.String("config", "", "风险控制配置路径（默认 config/khunter_risk_control.json）")
	casesFlag := flag.String("cases", strings.Join(khunter.DefaultCaseReplayCodes, ","), "逗号分隔案例代码（中稀有色固定 600259.SH）")
	out := flag.String("out", "-", "输出文件路径（- 表示 stdout）")
	flag.Parse()

	if *start == "" || *end == "" {
		fmt.Fprintln(os.Stderr, "-start 与 -end 必填（YYYY-MM-DD）")
		os.Exit(2)
	}

	variants := parseList(*variantsFlag)
	var variantEnums []khunter.BacktestVariant
	for _, v := range variants {
		variantEnums = append(variantEnums, khunter.BacktestVariant(v))
	}

	cases := parseList(*casesFlag)
	// 支持 "CODE:YYYY-MM-DD" 形式（案例锚定信号日），只取代码部分
	for i, c := range cases {
		if j := strings.LastIndex(c, ":"); j >= 0 {
			cases[i] = c[:j]
		}
	}
	if len(cases) > 0 {
		khunter.DefaultCaseReplayCodes = cases
	}

	var codes []string
	if !*allMarket {
		codes = cases
		if len(codes) == 0 {
			fmt.Fprintln(os.Stderr, "非全市场模式下必须提供 -cases 清单（或改用 -all-market）")
			os.Exit(2)
		}
	}

	db.Init(*dbPath)
	if err := khunter.EnsureMigrate(); err != nil {
		fmt.Fprintf(os.Stderr, "迁移失败: %v\n", err)
		os.Exit(1)
	}

	res, err := khunter.RunFullBacktest(context.Background(), khunter.BacktestRequest{
		StartDate:  *start,
		EndDate:    *end,
		Codes:      codes,
		Variants:   variantEnums,
		ConfigPath: *configPath,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "回测失败: %v\n", err)
		os.Exit(1)
	}

	blob, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "序列化失败: %v\n", err)
		os.Exit(1)
	}
	if *out == "-" {
		fmt.Println(string(blob))
		return
	}
	if err := os.WriteFile(*out, blob, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "写入 %s 失败: %v\n", *out, err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "已写出 %s（%d 字节）\n", *out, len(blob))
}

func parseList(s string) []string {
	out := make([]string, 0, 8)
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}
