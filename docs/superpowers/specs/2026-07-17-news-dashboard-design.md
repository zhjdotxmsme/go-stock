# 投资资讯看板设计（v1）

> 在 go-stock 中增加赛道分类新闻看板 + 个股关联消息功能。
> 参考项目：https://github.com/simonlin1212/investment-news
> 日期：2026-07-17

---

## 1. 概述

### 1.1 目标

- 侧边栏新增「投资资讯」顶级菜单，打开赛道分类新闻看板
- 12 大赛道分类（AI/半导体/机器人/新能源车/能源/医药/航天/安全/科技/消费电子/财经/热点）
- 每赛道顶部 3-5 条「今日要点」AI 摘要
- 下方按时间线排列详细新闻
- 个股详情页增加关联新闻 Tab

### 1.2 核心设计原则

- **复用现有基础设施**：不复写新闻抓取，基于已有 MarketNewsApi（财联社/东方财富/华尔街见闻）
- **赛道映射**：关键词 + AI 归类，每个赛道映射到 A 股板块
- **轻量 AI 摘要**：复用现有 OpenAI Stream / 情感分析能力
- **可扩展赛道**：赛道配置化，支持新增/修改

---

## 2. 后端设计

### 2.1 新增/修改文件

| 文件 | 改动类型 | 说明 |
|---|---|---|
| `backend/data/market_news_api.go` | 修改 | 新增 `GetNewsBySector` 方法 |
| `backend/data/news_sectors.go` | **新增** | 赛道定义与配置 |
| `backend/data/tool_agent_extra.go` | 修改 | 新增 `HandleGetNewsBySector` |
| `main.go` (App) | 修改 | 暴露 `GetNewsBySector` `GetStockNews` 绑定 |

### 2.2 赛道配置

```go
// news_sectors.go
type Sector struct {
    ID          string   `json:"id"`
    Name        string   `json:"name"`
    Keywords    []string `json:"keywords"`    // 匹配关键词
    StockSector []string `json:"stockSector"` // 对应A股板块代码
    Icon        string   `json:"icon"`
}

var NewsSectors = []Sector{
    {ID: "ai",         Name: "AI/大模型",   Keywords: []string{"AI","大模型","人工智能","GPT","LLM"}, StockSector: []string{"BK1131"}, Icon: "sparkles"},
    {ID: "semi",       Name: "半导体/芯片",  Keywords: []string{"芯片","半导体","晶圆","光刻"}, StockSector: []string{"BK1036"}, Icon: "hardware-chip"},
    {ID: "robot",      Name: "机器人/自动化",Keywords: []string{"机器人","自动化","工业母机"}, StockSector: []string{"BK1109"}, Icon: "robot"},
    {ID: "nev",        Name: "新能源车",     Keywords: []string{"新能源车","电动汽车","锂电","充电"}, StockSector: []string{"BK0900"}, Icon: "car"},
    {ID: "energy",     Name: "能源/新能源",  Keywords: []string{"光伏","风电","储能","氢能","新能源"}, StockSector: []string{"BK0497"}, Icon: "flash"},
    {ID: "medical",    Name: "生物医药",     Keywords: []string{"医药","生物","创新药","疫苗"}, StockSector: []string{"BK1014"}, Icon: "medkit"},
    {ID: "space",      Name: "航天/太空",    Keywords: []string{"航天","卫星","火箭","低空经济"}, StockSector: []string{"BK0721"}, Icon: "rocket"},
    {ID: "security",   Name: "网络安全",     Keywords: []string{"网络安全","信息安全","数据安全"}, StockSector: []string{"BK1002"}, Icon: "shield"},
    {ID: "tech",       Name: "科技/互联网",  Keywords: []string{"云计算","SaaS","数字经济","信创"}, StockSector: []string{"BK1030"}, Icon: "globe"},
    {ID: "consumer",   Name: "消费电子",     Keywords: []string{"消费电子","手机","可穿戴","VR"}, StockSector: []string{"BK1087"}, Icon: "phone-portrait"},
    {ID: "macro",      Name: "财经/宏观",    Keywords: []string{"央行","利率","GDP","CPI","美联储"}, StockSector: []string{}, Icon: "trending-up"},
    {ID: "hot",        Name: "热点事件",     Keywords: []string{}, StockSector: []string{}, Icon: "flame"},
}
```

### 2.3 MarketNewsApi 扩展

```go
// GetNewsBySector 按赛道获取新闻，返回结构化新闻列表
func (m *MarketNewsApi) GetNewsBySector(sectorID string, limit int) (*SectorNewsResponse, error)

// SectorNewsResponse 赛道新闻响应
type SectorNewsResponse struct {
    SectorID    string        `json:"sectorId"`
    SectorName  string        `json:"sectorName"`
    Highlights []NewsItem     `json:"highlights"` // 今日要点(AI摘要)
    News       []NewsItem     `json:"news"`       // 时间线新闻
}

// GetStockRelatedNews 获取个股关联新闻
func (m *MarketNewsApi) GetStockRelatedNews(code string, limit int) ([]NewsItem, error)
```

**数据流**：
1. `GetNewsBySector` 调用 `GetNewsList`/`TelegraphList` 获取原始新闻
2. 按赛道关键词过滤分类
3. 对分类后的新闻调用 AI 生成摘要（复用 `OpenAiStream`）
4. 返回结构化数据

### 2.4 个股关联新闻

```go
// GetStockRelatedNews 通过东方财富搜索接口获取个股相关新闻
func (m *MarketNewsApi) GetStockRelatedNews(code string, limit int) ([]NewsItem, error) {
    // 使用 EmAPI.FinanceSearch 或 IwencaiAPI.SearchNews
    // 按股票代码/名称搜索相关新闻
    // 返回匹配结果
}
```

---

## 3. 前端设计

### 3.1 路由与菜单

**路由注册** (`frontend/src/router/index.js`)：
```js
{
    path: '/news',
    name: 'news',
    component: () => import('../components/NewsPage.vue')
}
```

**App.vue 菜单**：在 `研究中心` 上方新增顶级菜单：
```js
{
    label: () => h(RouterLink, { to: { name: 'news' } }, { default: () => '投资资讯' }),
    key: 'news',
    icon: renderIcon(NewspaperOutline),
}
```

### 3.2 NewsPage.vue 看板页面

**布局**：
```
┌─────────────────────────────────────────────┐
│  [AI/大模型] [半导体] [机器人] [新能源] ...  │  ← 赛道 Tab 导航
├─────────────────────────────────────────────┤
│  赛道名: AI/大模型                          │
│  ┌───────────────────────────────────────┐  │
│  │ 📌 今日要点                              │  │
│  │  • OpenAI发布... (AI摘要)  [原文]        │  │
│  │  • 国内大模型...  (AI摘要)  [原文]        │  │
│  │  • 英伟达财报...  (AI摘要)  [原文]        │  │
│  └───────────────────────────────────────┘  │
│  ┌───────────────────────────────────────┐  │
│  │ 时间线新闻                              │  │
│  │ [15:30] 标题1...        来源   [原文]    │  │
│  │ [14:20] 标题2...  关联: 科大讯飞 [原文]  │  │
│  │ [13:10] 标题3...        来源   [原文]    │  │
│  └───────────────────────────────────────┘  │
│  加载更多 ↓                                  │
└─────────────────────────────────────────────┘
```

**组件结构**：
- `NewsPage.vue` — 主页面，赛道 Tab 切换
- `SectorHighlights.vue` — （内联）今日要点区域
- `NewsTimeline.vue` — （内联）新闻时间线列表

**数据源**：调用 `GetNewsBySector(sectorId)` Wails 绑定

**交互**：
- Tab 切换时加载对应赛道新闻
- 支持下拉刷新（`n-spin`）
- 新闻卡片可点击展开详情或跳转原文
- 关联股票可点击跳转股票页面

### 3.3 个股关联新闻

**stock.vue 改动**：在个股详情 Tab 栏新增「关联资讯」Tab

```
┌─ 行情 ─┬─ K线 ─┬─ 资金流 ─┬─ 关联资讯 ─┐
│                                     │
│  关联新闻列表（按时间倒序）           │
│  ┌─────────────────────────────┐  │
│  │ 标题...  来源   15分钟前  原文│  │
│  │ 摘要内容...                   │  │
│  └─────────────────────────────┘  │
│  ┌─────────────────────────────┐  │
│  │ 标题...  来源   1小时前   原文│  │
│  └─────────────────────────────┘  │
└─────────────────────────────────┘
```

**组件**：`StockNews.vue`（新增），嵌入 stock.vue

### 3.4 主题适配

- 支持深色/浅色主题（继承 `enableDarkTheme`）
- Naive UI 组件风格一致（`n-tabs`, `n-card`, `n-spin`）

---

## 4. 实施步骤

### 第一阶段：基础设施

1. 创建 `news_sectors.go` — 赛道定义与配置
2. `MarketNewsApi` 扩展 — `GetNewsBySector` + `GetStockRelatedNews`
3. Wails 绑定注册

### 第二阶段：赛道看板

4. 路由注册 + App.vue 菜单
5. `NewsPage.vue` 看板页面
6. 今日要点 AI 摘要集成

### 第三阶段：个股关联

7. `StockNews.vue` 组件
8. stock.vue 集成关联新闻 Tab

### 第四阶段：打磨

9. 加载态/空态/错误态处理
10. 深色主题适配
11. 数据缓存与刷新

---

## 5. 文件清单

| 文件 | 改动 |
|---|---|
| `backend/data/news_sectors.go` | **新增** — 赛道配置 |
| `backend/data/market_news_api.go` | 修改 — 新增 `GetNewsBySector` `GetStockRelatedNews` |
| `backend/data/tool_agent_extra.go` | 修改 — 新增处理方法 |
| `main.go` | 修改 — 暴露新绑定 |
| `frontend/src/components/NewsPage.vue` | **新增** — 赛道看板 |
| `frontend/src/components/StockNews.vue` | **新增** — 个股关联新闻 |
| `frontend/src/components/stock.vue` | 修改 — 新增关联资讯 Tab |
| `frontend/src/App.vue` | 修改 — 菜单 + 路由 |
| `frontend/src/router/index.js` | 修改 — 注册 /news 路由 |

---

## 6. 验收标准

- [ ] 侧边栏「投资资讯」菜单可见，点击打开赛道看板
- [ ] 12 个赛道 Tab 可切换，每赛道显示对应新闻
- [ ] 每个赛道顶部显示 3-5 条今日要点（AI 摘要）
- [ ] 新闻列表按时间线排列，显示来源和原文链接
- [ ] 新闻卡片中关联股票可点击跳转
- [ ] 个股详情页「关联资讯」Tab 显示相关新闻
- [ ] 深色/浅色主题适配
- [ ] 加载态/空数据/错误状态正确处理
