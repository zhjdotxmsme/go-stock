package data

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go-stock/backend/logger"
	"go-stock/backend/util"
	"io"
	"strings"
	"sync"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// 本文件实现 FetchMarketSnapshot 的降级数据源：
// 东方财富 push2 接口在部分网络环境下会被重置（EOF），此时用
// 腾讯行情（指数报价）+ 新浪 Market_Center（全市场涨跌幅）补齐数据。

// fetchTencentIndexQuotes 通过腾讯行情接口获取指数报价（不含分市场涨跌家数）。
// codes 形如 ["sh000001", "sz399001"]。
func fetchTencentIndexQuotes(codes []string) ([]EMIndexQuote, error) {
	if len(codes) == 0 {
		return nil, fmt.Errorf("no index codes")
	}
	url := "http://qt.gtimg.cn/q=" + strings.Join(codes, ",")
	resp, err := SharedHTTPClient.R().
		SetHeader("User-Agent", util.GetUserAgent()).
		SetHeader("Referer", "https://gu.qq.com/").
		Get(url)
	if err != nil {
		return nil, fmt.Errorf("tencent index quotes request: %w", err)
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("tencent index quotes HTTP %d", resp.StatusCode())
	}
	// 腾讯行情接口返回 GBK 编码文本
	gbkReader := transform.NewReader(bytes.NewReader(resp.Body()), simplifiedchinese.GBK.NewDecoder())
	textBytes, err := io.ReadAll(gbkReader)
	if err != nil {
		return nil, fmt.Errorf("tencent index quotes decode: %w", err)
	}

	var quotes []EMIndexQuote
	for _, line := range strings.Split(string(textBytes), "\n") {
		line = strings.TrimSpace(line)
		// 形如 v_sh000001="1~上证指数~000001~3934.40~...";
		eq := strings.Index(line, "=")
		if eq <= 0 {
			continue
		}
		raw := strings.Trim(strings.TrimSpace(line[eq+1:]), `";`)
		fields := strings.Split(raw, "~")
		if len(fields) < 33 {
			continue
		}
		var pct, change float64
		fmt.Sscanf(fields[32], "%f", &pct)
		fmt.Sscanf(fields[31], "%f", &change)
		var price float64
		fmt.Sscanf(fields[3], "%f", &price)
		quotes = append(quotes, EMIndexQuote{
			Code:      "sh" + fields[2], // 与东财路径保持一致的历史代码格式（sz399001 也写作 sh399001）
			Name:      fields[1],
			Price:     price,
			ChangePct: pct,
			Change:    change,
		})
	}
	if len(quotes) == 0 {
		return nil, fmt.Errorf("tencent index quotes: no parseable rows")
	}
	return quotes, nil
}

type sinaMarketItem struct {
	Symbol        string      `json:"symbol"`
	ChangePercent interface{} `json:"changepercent"`
}

const sinaMarketPageSize = 100 // 新浪该接口单页上限为 100（实测传更大的 num 也只返回 100）

// fetchSinaStockCount 获取某节点股票总数（如 hs_a）。失败不影响主流程（走顺序翻页兜底）。
func fetchSinaStockCount(node string) (int, error) {
	url := fmt.Sprintf("http://vip.stock.finance.sina.com.cn/quotes_service/api/json_v2.php/Market_Center.getHQNodeStockCount?node=%s", node)
	resp, err := SharedHTTPClient.R().
		SetHeader("User-Agent", util.GetUserAgent()).
		SetHeader("Referer", "https://finance.sina.com.cn").
		Get(url)
	if err != nil {
		return 0, err
	}
	// 返回形如 "5560"（带引号的数字字符串）
	raw := strings.Trim(strings.TrimSpace(string(resp.Body())), `"`)
	var n int
	if _, err := fmt.Sscanf(raw, "%d", &n); err != nil || n <= 0 {
		return 0, fmt.Errorf("parse count %q failed", raw)
	}
	return n, nil
}

// fetchSinaMarketPage 拉取一页全市场股票涨跌幅
func fetchSinaMarketPage(node string, page int) ([]sinaMarketItem, error) {
	url := fmt.Sprintf("http://vip.stock.finance.sina.com.cn/quotes_service/api/json_v2.php/Market_Center.getHQNodeData?page=%d&num=%d&sort=changepercent&asc=0&node=%s",
		page, sinaMarketPageSize, node)
	resp, err := SharedHTTPClient.R().
		SetHeader("User-Agent", util.GetUserAgent()).
		SetHeader("Referer", "https://finance.sina.com.cn").
		Get(url)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode())
	}
	body := resp.Body()
	if len(body) == 0 || string(body) == "null" {
		return nil, nil
	}
	var items []sinaMarketItem
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	return items, nil
}

// fetchSinaAllAStockChanges 拉取全部沪深 A 股（hs_a 含北交所，后续按前缀剔除）涨跌幅。
// 优先按总数并发翻页；拿不到总数则顺序翻页直到短页。
func fetchSinaAllAStockChanges() ([]sinaMarketItem, error) {
	total, err := fetchSinaStockCount("hs_a")
	if err != nil || total <= 0 {
		logger.SugaredLogger.Warnf("新浪涨跌分布：获取总数失败(%v)，退化为顺序翻页", err)
		var all []sinaMarketItem
		for page := 1; page <= 80; page++ {
			items, err := fetchSinaMarketPage("hs_a", page)
			if err != nil {
				return nil, err
			}
			all = append(all, items...)
			if len(items) < sinaMarketPageSize {
				break
			}
		}
		return all, nil
	}

	pages := (total + sinaMarketPageSize - 1) / sinaMarketPageSize
	results := make([][]sinaMarketItem, pages+1) // 1-based page index
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	var mu sync.Mutex
	failedPages := 0

	for p := 1; p <= pages; p++ {
		wg.Add(1)
		go func(page int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			items, err := fetchSinaMarketPage("hs_a", page)
			if err != nil {
				logger.SugaredLogger.Warnf("新浪涨跌分布：第 %d 页失败: %v", page, err)
				mu.Lock()
				failedPages++
				mu.Unlock()
				return
			}
			results[page] = items
		}(p)
	}
	wg.Wait()

	var all []sinaMarketItem
	for p := 1; p <= pages; p++ {
		all = append(all, results[p]...)
	}
	// 缺页超过一半则认为数据不可信
	if len(all) < total/2 {
		return nil, fmt.Errorf("sina breadth incomplete: got %d/%d stocks (%d pages failed)", len(all), total, failedPages)
	}
	if failedPages > 0 {
		logger.SugaredLogger.Warnf("新浪涨跌分布：%d 页失败，使用不完整数据 (%d/%d 只股票)", failedPages, len(all), total)
	}
	return all, nil
}

// bucketPercent 与东财路径相同的涨跌幅分桶逻辑
func bucketPercent(dis *EMUpDownDis, pct float64) {
	dis.AverageRise += pct
	switch {
	case pct >= 9.8:
		dis.LimitUp++
		dis.RiseCount++
	case pct >= 8:
		dis.Up10++
		dis.RiseCount++
	case pct >= 6:
		dis.Up8++
		dis.RiseCount++
	case pct >= 4:
		dis.Up6++
		dis.RiseCount++
	case pct >= 2:
		dis.Up4++
		dis.RiseCount++
	case pct > 0:
		dis.Up2++
		dis.RiseCount++
	case pct == 0:
		dis.FlatCount++
	case pct > -2:
		dis.Down2++
		dis.FallCount++
	case pct > -4:
		dis.Down4++
		dis.FallCount++
	case pct > -6:
		dis.Down6++
		dis.FallCount++
	case pct > -8:
		dis.Down8++
		dis.FallCount++
	case pct > -9.8:
		dis.Down10++
		dis.FallCount++
	default:
		dis.LimitDown++
		dis.FallCount++
	}
}

// fetchMarketSnapshotFallback 东财不可用时的降级实现：
// 指数报价走腾讯，涨跌分布走新浪全市场分页统计（按代码前缀归属沪/深，北交所剔除以保持与东财口径一致）。
func fetchMarketSnapshotFallback() (*EMMarketSnapshot, error) {
	snap := &EMMarketSnapshot{Source: "腾讯+新浪（东财接口不可用，已降级）"}

	quotes, err := fetchTencentIndexQuotes([]string{"sh000001", "sz399001"})
	if err != nil {
		return nil, fmt.Errorf("fallback index quotes: %w", err)
	}

	items, err := fetchSinaAllAStockChanges()
	if err != nil {
		return nil, fmt.Errorf("fallback breadth: %w", err)
	}

	// 按交易所归属统计（用于指数行的上涨/下跌/平盘家数）
	exchCount := map[string]*struct{ up, down, flat int }{
		"sh000001": {},
		"sh399001": {}, // 深市行沿用东财路径的历史代码写法
	}

	for _, item := range items {
		sym := strings.ToLower(strings.TrimSpace(item.Symbol))
		// 东财 clist 口径不含北交所，这里同样剔除
		if strings.HasPrefix(sym, "bj") {
			continue
		}
		pct := toFloat64(item.ChangePercent)
		bucketPercent(&snap.UpDownDis, pct)

		var key string
		switch {
		case strings.HasPrefix(sym, "sh"):
			key = "sh000001"
		case strings.HasPrefix(sym, "sz"):
			key = "sh399001"
		}
		if c, ok := exchCount[key]; ok {
			switch {
			case pct > 0:
				c.up++
			case pct < 0:
				c.down++
			default:
				c.flat++
			}
		}
	}

	totalStocks := snap.UpDownDis.RiseCount + snap.UpDownDis.FallCount + snap.UpDownDis.FlatCount
	if totalStocks > 0 {
		snap.UpDownDis.AverageRise /= float64(totalStocks)
	}

	for i := range quotes {
		if c, ok := exchCount[quotes[i].Code]; ok {
			quotes[i].UpCount = c.up
			quotes[i].DownCount = c.down
			quotes[i].FlatCount = c.flat
		}
	}
	snap.IndexQuotes = quotes
	return snap, nil
}
