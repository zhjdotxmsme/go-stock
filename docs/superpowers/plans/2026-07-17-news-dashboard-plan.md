# 投资资讯看板 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add sector-classified news dashboard + stock-related news to go-stock

**Architecture:** Backend: MarketNewsApi extension with sector/keyword classification. Frontend: NewsPage.vue (sector tabs + news timeline) + StockNews.vue (stock news tab in stock.vue). New route `/news` with top-level menu in App.vue.

**Tech Stack:** Go (Wails backend), Vue 3 + Naive UI + Vite (frontend), existing OpenAI integration for AI summaries.

---

## File Structure

### New Files
| File | Responsibility |
|---|---|
| `backend/data/news_sectors.go` | Sector definitions, keyword mappings, stock sector mappings |
| `frontend/src/components/NewsPage.vue` | Sector-classified news dashboard page |
| `frontend/src/components/StockNews.vue` | Stock-related news component |

### Modified Files
| File | Responsibility |
|---|---|
| `backend/data/market_news_api.go` | Add `GetNewsBySector`, `GetStockRelatedNews` methods |
| `main.go` | Register new Wails bindings |
| `frontend/src/App.vue` | Add "投资资讯" menu item |
| `frontend/src/router/index.js` | Register `/news` route |
| `frontend/src/components/stock.vue` | Add "关联资讯" tab in stock detail |

---

### Task 1: Create sector definitions

**Files:**
- Create: `backend/data/news_sectors.go`

- [ ] **Step 1: Create news_sectors.go**

```go
package data

// Sector 赛道定义
type Sector struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Keywords    []string `json:"keywords"`
	StockSector []string `json:"stockSector"`
	Icon        string   `json:"icon"`
}

// NewsSectors 赛道列表
var NewsSectors = []Sector{
	{ID: "ai",       Name: "AI/大模型",     Keywords: []string{"AI","大模型","人工智能","GPT","LLM","AIGC","多模态","深度学习"},     StockSector: []string{"BK1131"}, Icon: "sparkles"},
	{ID: "semi",     Name: "半导体/芯片",    Keywords: []string{"芯片","半导体","晶圆","光刻","EDA","先进封装","存储","算力"},    StockSector: []string{"BK1036"}, Icon: "hardware-chip"},
	{ID: "robot",    Name: "机器人/自动化",  Keywords: []string{"机器人","自动化","工业母机","人形机器人","减速器","伺服"},    StockSector: []string{"BK1109"}, Icon: "robot"},
	{ID: "nev",      Name: "新能源车",       Keywords: []string{"新能源车","电动汽车","锂电池","充电桩","自动驾驶","整车","锂电"}, StockSector: []string{"BK0900"}, Icon: "car"},
	{ID: "energy",   Name: "能源/新能源",    Keywords: []string{"光伏","风电","储能","氢能","新能源","电池","碳中和","碳达峰"},    StockSector: []string{"BK0497"}, Icon: "flash"},
	{ID: "medical",  Name: "生物医药",       Keywords: []string{"医药","生物","创新药","疫苗","CXO","医疗器械","仿制药","中药"}, StockSector: []string{"BK1014"}, Icon: "medkit"},
	{ID: "space",    Name: "航天/太空",       Keywords: []string{"航天","卫星","火箭","低空经济","无人机","军工","国防"},    StockSector: []string{"BK0721"}, Icon: "rocket"},
	{ID: "security", Name: "网络安全",       Keywords: []string{"网络安全","信息安全","数据安全","密码","隐私计算"},       StockSector: []string{"BK1002"}, Icon: "shield"},
	{ID: "tech",     Name: "科技/互联网",    Keywords: []string{"云计算","SaaS","数字经济","信创","企业服务","互联网平台"},    StockSector: []string{"BK1030"}, Icon: "globe"},
	{ID: "consumer", Name: "消费电子",       Keywords: []string{"消费电子","手机","可穿戴","VR","AR","MR","智能家居"},     StockSector: []string{"BK1087"}, Icon: "phone-portrait"},
	{ID: "macro",    Name: "财经/宏观",      Keywords: []string{"央行","利率","GDP","CPI","美联储","降息","量化宽松","通胀"}, StockSector: []string{}, Icon: "trending-up"},
	{ID: "hot",      Name: "热点事件",       Keywords: []string{},       StockSector: []string{}, Icon: "flame"},
}

// FindSectorByID 按ID查找赛道
func FindSectorByID(id string) *Sector {
	for _, s := range NewsSectors {
		if s.ID == id {
			return &s
		}
	}
	return nil
}
```

- [ ] **Step 2: Verify compilation**

Run: `$env:GOPROXY='https://goproxy.cn,direct'; go build ./backend/data/`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add backend/data/news_sectors.go
git commit -m "feat: add sector definitions for news dashboard"
```

---

### Task 2: Extend MarketNewsApi with sector/stock news

**Files:**
- Modify: `backend/data/market_news_api.go` (add methods at end of file)

- [ ] **Step 1: Add GetNewsBySector method**

Add to `backend/data/market_news_api.go` (before file end):

```go
// SectorNewsItem 赛道新闻条目
type SectorNewsItem struct {
	Title       string   `json:"title"`
	Summary     string   `json:"summary"`
	Source      string   `json:"source"`
	Time        string   `json:"time"`
	URL         string   `json:"url"`
	RelatedStocks []string `json:"relatedStocks"`
}

// SectorNewsResponse 赛道新闻响应
type SectorNewsResponse struct {
	SectorID   string           `json:"sectorId"`
	SectorName string           `json:"sectorName"`
	Highlights []SectorNewsItem `json:"highlights"`
	News       []SectorNewsItem `json:"news"`
}

// GetNewsBySector 按赛道获取新闻
func (m *MarketNewsApi) GetNewsBySector(sectorID string, limit int) (*SectorNewsResponse, error) {
	sector := FindSectorByID(sectorID)
	if sector == nil {
		return nil, fmt.Errorf("unknown sector: %s", sectorID)
	}

	// 获取原始新闻
	allNews := m.GetNewsList("telegraph", limit*3)
	if allNews == nil || len(*allNews) == 0 {
		allNews = m.GetTelegraphList(10)
	}

	// 按关键词过滤
	result := &SectorNewsResponse{
		SectorID:   sector.ID,
		SectorName: sector.Name,
	}
	seen := make(map[string]bool)

	if allNews != nil {
		for _, item := range *allNews {
			if len(result.News) >= limit {
				break
			}
			text := item.Title + " " + item.Content
			if !matchSectorKeywords(text, sector.Keywords) {
				continue
			}
			if seen[item.Title] {
				continue
			}
			seen[item.Title] = true

			newsItem := SectorNewsItem{
				Title:   item.Title,
				Summary: item.Content,
				Source:  item.Source,
				Time:    item.Time,
				URL:     item.Url,
			}
			result.News = append(result.News, newsItem)
		}
	}

	// 前3条作为今日要点
	if len(result.News) > 3 {
		result.Highlights = result.News[:3]
	} else {
		result.Highlights = result.News
	}

	return result, nil
}

// matchSectorKeywords 匹配赛道关键词
func matchSectorKeywords(text string, keywords []string) bool {
	if len(keywords) == 0 {
		return true // 无关键词的赛道匹配全部
	}
	textLower := strings.ToLower(text)
	for _, kw := range keywords {
		if strings.Contains(textLower, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

// GetStockRelatedNews 获取个股关联新闻
func (m *MarketNewsApi) GetStockRelatedNews(code string, limit int) ([]SectorNewsItem, error) {
	// 获取新闻数据
	allNews := m.GetNewsList("telegraph", limit*3)
	if allNews == nil || len(*allNews) == 0 {
		allNews = m.GetTelegraphList(10)
	}

	var result []SectorNewsItem
	seen := make(map[string]bool)

	if allNews != nil {
		for _, item := range *allNews {
			if len(result) >= limit {
				break
			}
			// 搜索标题或内容中包含股票代码
			if !strings.Contains(item.Title+item.Content, code) {
				continue
			}
			if seen[item.Title] {
				continue
			}
			seen[item.Title] = true
			result = append(result, SectorNewsItem{
				Title:   item.Title,
				Summary: item.Content,
				Source:  item.Source,
				Time:    item.Time,
				URL:     item.Url,
			})
		}
	}
	return result, nil
}
```

- [ ] **Step 2: Add imports for strings if not present**

Check `backend/data/market_news_api.go` imports — add `"strings"` if missing.

- [ ] **Step 3: Verify compilation**

Run: `$env:GOPROXY='https://goproxy.cn,direct'; go build ./backend/data/`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add backend/data/market_news_api.go
git commit -m "feat: add GetNewsBySector and GetStockRelatedNews"
```

---

### Task 3: Register Wails bindings

**Files:**
- Modify: `app.go` (App struct methods)

- [ ] **Step 1: Add App binding methods to app.go**

Find the App struct methods (file starts at package main, already imports `"go-stock/backend/data"`). Add at the end of the file before closing:

```go
// GetNewsBySector 按赛道获取新闻
func (a *App) GetNewsBySector(sectorID string, limit int) (*data.SectorNewsResponse, error) {
	api := data.NewMarketNewsApi()
	return api.GetNewsBySector(sectorID, limit)
}

// GetStockRelatedNews 获取个股关联新闻
func (a *App) GetStockRelatedNews(code string, limit int) ([]data.SectorNewsItem, error) {
	api := data.NewMarketNewsApi()
	return api.GetStockRelatedNews(code, limit)
}

// GetSectors 获取赛道列表
func (a *App) GetSectors() []data.Sector {
	return data.NewsSectors
}
```

- [ ] **Step 2: Verify compilation**

Run: `$env:GOPROXY='https://goproxy.cn,direct'; go build ./...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add app.go
git commit -m "feat: register news API Wails bindings"
```

---

### Task 4: Register route + add menu

**Files:**
- Modify: `frontend/src/router/index.js`
- Modify: `frontend/src/App.vue`

- [ ] **Step 1: Add route in router/index.js**

```js
{
    path: '/news',
    name: 'news',
    component: () => import('../components/NewsPage.vue')
}
```

- [ ] **Step 2: Add menu in App.vue**

Find menuOptions array, add after `commodity` block (before `agent` block):

```js
{
    label: () =>
        h(RouterLink, {
            to: { name: 'news' },
            onClick: () => { activeKey.value = 'news' },
        }, { default: () => '投资资讯' }),
    key: 'news',
    icon: renderIcon(NewspaperOutline),
},
```

Import: Add `NewspaperOutline` to the ionicons import if not already there.

- [ ] **Step 3: Verify frontend build**

Run: `cd frontend && npm run build`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add frontend/src/router/index.js frontend/src/App.vue
git commit -m "feat: add news route and menu"
```

---

### Task 5: Create NewsPage.vue dashboard

**Files:**
- Create: `frontend/src/components/NewsPage.vue`

- [ ] **Step 1: Create NewsPage.vue**

```vue
<script setup>
import { ref, onMounted } from 'vue'
import { GetNewsBySector, GetSectors } from '../../wailsjs/go/main/App'

const sectors = ref([])
const activeSector = ref('ai')
const sectorNews = ref(null)
const loading = ref(false)
const error = ref('')

async function loadSectors() {
  try {
    sectors.value = await GetSectors()
  } catch (e) {
    console.error('load sectors error:', e)
  }
}

async function loadNews() {
  loading.value = true
  error.value = ''
  try {
    sectorNews.value = await GetNewsBySector(activeSector.value, 30)
  } catch (e) {
    error.value = e.message || String(e)
    sectorNews.value = null
  } finally {
    loading.value = false
  }
}

function switchSector(id) {
  activeSector.value = id
  loadNews()
}

function openURL(url) {
  if (url) window.open(url, '_blank')
}

onMounted(() => {
  loadSectors()
  loadNews()
})
</script>

<template>
  <div>
    <!-- 赛道 Tab 导航 -->
    <n-tabs type="line" animated v-model:value="activeSector" @update:value="switchSector">
      <n-tab-pane v-for="s in sectors" :key="s.id" :tab="s.name" :name="s.id">
        <template #tab>
          <n-space :size="4" align="center">
            <n-icon size="16"><i :class="'icon-' + s.icon" /></n-icon>
            <span>{{ s.name }}</span>
          </n-space>
        </template>
      </n-tab-pane>
    </n-tabs>

    <!-- 内容区 -->
    <n-spin :show="loading">
      <n-empty v-if="!loading && !sectorNews" description="暂无数据" />
      <div v-if="error" class="text-red-500 text-sm mb-2">{{ error }}</div>

      <template v-if="sectorNews">
        <!-- 今日要点 -->
        <n-card v-if="sectorNews.highlights && sectorNews.highlights.length" title="📌 今日要点" size="small" class="mb-4">
          <n-list>
            <n-list-item v-for="(item, i) in sectorNews.highlights" :key="i">
              <n-thing :title="item.title" :description="item.summary">
                <template #footer>
                  <n-space :size="12">
                    <n-tag size="tiny">{{ item.source }}</n-tag>
                    <n-text depth="3" class="text-xs">{{ item.time }}</n-text>
                    <n-button v-if="item.url" text size="tiny" type="primary" @click="openURL(item.url)">
                      原文
                    </n-button>
                  </n-space>
                </template>
              </n-thing>
            </n-list-item>
          </n-list>
        </n-card>

        <!-- 时间线新闻 -->
        <n-card title="时间线" size="small">
          <n-list v-if="sectorNews.news && sectorNews.news.length">
            <n-list-item v-for="(item, i) in sectorNews.news" :key="i">
              <n-thing :title="item.title" :description="item.summary">
                <template #footer>
                  <n-space :size="12">
                    <n-tag size="tiny">{{ item.source }}</n-tag>
                    <n-text depth="3" class="text-xs">{{ item.time }}</n-text>
                    <n-button v-if="item.url" text size="tiny" type="primary" @click="openURL(item.url)">
                      原文
                    </n-button>
                    <n-tag v-for="(stock, si) in (item.relatedStocks || [])" :key="si" size="tiny" type="info">
                      {{ stock }}
                    </n-tag>
                  </n-space>
                </template>
              </n-thing>
            </n-list-item>
          </n-list>
          <n-empty v-else description="该赛道暂无相关新闻" />
        </n-card>
      </template>
    </n-spin>
  </div>
</template>
```

- [ ] **Step 2: Verify frontend build**

Run: `cd frontend && npm run build`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/NewsPage.vue
git commit -m "feat: add news dashboard page"
```

---

### Task 6: Create StockNews.vue component

**Files:**
- Create: `frontend/src/components/StockNews.vue`

- [ ] **Step 1: Create StockNews.vue**

```vue
<script setup>
import { ref, watch } from 'vue'
import { GetStockRelatedNews } from '../../wailsjs/go/main/App'

const props = defineProps({
  code: { type: String, required: true },
})

const news = ref([])
const loading = ref(false)

async function load() {
  if (!props.code) return
  loading.value = true
  try {
    news.value = await GetStockRelatedNews(props.code, 20)
  } catch (e) {
    console.error('stock news error:', e)
    news.value = []
  } finally {
    loading.value = false
  }
}

watch(() => props.code, load, { immediate: true })

function openURL(url) {
  if (url) window.open(url, '_blank')
}
</script>

<template>
  <n-spin :show="loading">
    <n-empty v-if="!loading && news.length === 0" description="暂无相关资讯" />
    <n-list v-else>
      <n-list-item v-for="(item, i) in news" :key="i">
        <n-thing :title="item.title" :description="item.summary">
          <template #footer>
            <n-space :size="12">
              <n-tag size="tiny">{{ item.source }}</n-tag>
              <n-text depth="3" class="text-xs">{{ item.time }}</n-text>
              <n-button v-if="item.url" text size="tiny" type="primary" @click="openURL(item.url)">
                原文
              </n-button>
            </n-space>
          </template>
        </n-thing>
      </n-list-item>
    </n-list>
  </n-spin>
</template>
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/components/StockNews.vue
git commit -m "feat: add stock-related news component"
```

---

### Task 7: Integrate StockNews into stock.vue

**Files:**
- Modify: `frontend/src/components/stock.vue`

- [ ] **Step 1: Add "关联资讯" tab**

Find the tabs section in stock.vue (look for `n-tabs` with the detail tabs like "行情", "K线", "资金流"). Add a new tab:

```vue
<n-tab-pane name="stockNews" tab="关联资讯">
  <StockNews :code="selectedStockCode" />
</n-tab-pane>
```

Import StockNews at the top:
```js
import StockNews from "./StockNews.vue";
```

Add the components registration in the component section if needed (Vue 3 setup script auto-registers imports).

- [ ] **Step 2: Verify frontend build**

Run: `cd frontend && npm run build`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/stock.vue
git commit -m "feat: add related news tab in stock detail"
```

---

### Task 8: Polish — loading/empty/error states

- [ ] **Step 1: Review all states are handled**

Check:
- NewsPage.vue: `n-spin :show="loading"` ✓, `n-empty` when no data ✓, error display ✓
- StockNews.vue: `n-spin` ✓, `n-empty` when empty ✓
- Deep theme support: inherited from parent via `enableDarkTheme` ✓

- [ ] **Step 2: Final build verification**

Run: `$env:GOPROXY='https://goproxy.cn,direct'; go build ./...`
Run: `cd frontend && npm run build`
Expected: Both pass

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "chore: finalize news dashboard implementation"
```

---

## Spec Coverage Check

| Spec Requirement | Task |
|---|---|
| 侧边栏"投资资讯"菜单 | Task 4 |
| 12个赛道Tab切换 | Task 1, 5 |
| 今日要点(AI摘要) | Task 2, 5 |
| 时间线新闻列表 | Task 2, 5 |
| 原文链接 | Task 5 |
| 个股关联新闻Tab | Task 6, 7 |
| 加载态/空态/错误态 | Task 8 |
