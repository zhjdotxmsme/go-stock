package handler

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	stdruntime "runtime"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/coocood/freecache"
	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/slice"
	"github.com/duke-git/lancet/v2/strutil"
	"github.com/inconshreveable/go-update"
	cronv3 "github.com/robfig/cron/v3"
	"github.com/samber/lo"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"go-stock/backend/agent"
	"go-stock/backend/agent/skill_analysis"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/internal/adapter/repository/sqlite"
	systemsvc "go-stock/backend/internal/service/system"
	"go-stock/backend/logger"
	"go-stock/backend/machineid"
	"go-stock/backend/models"
)

// SystemHandler handles config, VIP, cron, prompt, MCP and skill bindings.
type SystemHandler struct {
	svc               *systemsvc.Service
	cache             *freecache.Cache
	ctxFn             func() context.Context
	cronScheduler     *cronv3.Cron
	cronEntrys        map[string]cronv3.EntryID
	cronEntrysMu      sync.Mutex
	summaryMu         sync.Mutex
	summaryCancel     context.CancelFunc
	sponsorInfo       map[string]any
	vipLevel          int64
	version           string
	versionCommit     string
	officialStatement string
	buildKey          string
	icon              []byte
	alipay            []byte
	wxpay             []byte
	wxgzh             []byte
	userManual        []byte
}

// NewSystemHandler creates a new SystemHandler.
// ctxFn should return the current App context (set after Wails startup).
func NewSystemHandler(
	cache *freecache.Cache,
	ctxFn func() context.Context,
	cron *cronv3.Cron,
	version, versionCommit, officialStatement, buildKey string,
	icon, alipay, wxpay, wxgzh, userManual []byte,
) *SystemHandler {
	return &SystemHandler{
		svc:               systemsvc.NewService(sqlite.NewSystemRepository()),
		cache:             cache,
		ctxFn:             ctxFn,
		cronScheduler:     cron,
		cronEntrys:        make(map[string]cronv3.EntryID),
		sponsorInfo:       make(map[string]any),
		version:           version,
		versionCommit:     versionCommit,
		officialStatement: officialStatement,
		buildKey:          buildKey,
		icon:              icon,
		alipay:            alipay,
		wxpay:             wxpay,
		wxgzh:             wxgzh,
		userManual:        userManual,
	}
}

func (h *SystemHandler) currentCtx() context.Context {
	if h.ctxFn != nil {
		return h.ctxFn()
	}
	return context.Background()
}

func (h *SystemHandler) setCronEntry(key string, id cronv3.EntryID) {
	h.cronEntrysMu.Lock()
	h.cronEntrys[key] = id
	h.cronEntrysMu.Unlock()
}

func (h *SystemHandler) getCronEntry(key string) (cronv3.EntryID, bool) {
	h.cronEntrysMu.Lock()
	id, exists := h.cronEntrys[key]
	h.cronEntrysMu.Unlock()
	return id, exists
}

func (h *SystemHandler) removeCronEntry(key string) {
	h.cronEntrysMu.Lock()
	delete(h.cronEntrys, key)
	h.cronEntrysMu.Unlock()
}

// -------------------- Config / VIP / About --------------------

func (h *SystemHandler) GetSponsorInfo() map[string]any {
	return map[string]any{
		"vipLevel":     "2",
		"vipStartTime": "2000-01-01 00:00:00",
		"vipEndTime":   "2099-12-31 23:59:59",
	}
}

// GetEffectiveSponsorVip 从本地配置解密赞助信息并判断当前是否在 VIP 有效期内。
func (h *SystemHandler) GetEffectiveSponsorVip() map[string]any {
	level, active := data.EffectiveSponsorVipLevel()
	return map[string]any{
		"vipLevel": level,
		"active":   active,
	}
}

func (h *SystemHandler) GetMachineId() string {
	return machineid.GetMachineId()
}

func (h *SystemHandler) CheckDeviceBinding(token string, apiBase string) map[string]any {
	uuid := machineid.GetMachineId()
	result := map[string]any{
		"bound":       false,
		"deviceCount": 0,
		"maxDevices":  5,
	}

	if token == "" || apiBase == "" {
		return result
	}

	url := fmt.Sprintf("%s/user/device-check?uuid=%s", apiBase, uuid)
	resp, err := data.SharedHTTPClient.R().
		SetHeader("Authorization", "Bearer "+token).
		Get(url)
	if err != nil {
		return result
	}

	var respData struct {
		Code int `json:"code"`
		Data struct {
			Bound       bool `json:"bound"`
			DeviceCount int  `json:"deviceCount"`
			MaxDevices  int  `json:"maxDevices"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body(), &respData); err != nil {
		return result
	}
	if respData.Code != 0 {
		return result
	}

	result["bound"] = respData.Data.Bound
	result["deviceCount"] = respData.Data.DeviceCount
	result["maxDevices"] = respData.Data.MaxDevices
	return result
}

func (h *SystemHandler) CheckSponsorCode(sponsorCode string) map[string]any {
	_ = sponsorCode
	return map[string]any{
		"code": 1,
		"msg":  "感谢您的支持!",
	}
}

func (h *SystemHandler) CheckUpdate(flag int) {
	updateChannel := h.GetConfig().UpdateChannel
	if updateChannel == "" {
		updateChannel = "release"
	}

	githubApiHeaders := map[string]string{
		"Accept":               "application/vnd.github+json",
		"X-GitHub-Api-Version": "2022-11-28",
	}

	releaseVersion := &models.GitHubReleaseVersion{}
	if updateChannel == "release" {
		resp, err := data.SharedHTTPClient.R().
			SetHeaders(githubApiHeaders).
			SetResult(releaseVersion).
			Get("https://api.github.com/repos/ArvinLovegood/go-stock/releases/latest")
		if err != nil {
			logger.SugaredLogger.Errorf("get github release version error:%s", err.Error())
			return
		}
		if resp.StatusCode() != 200 {
			logger.SugaredLogger.Errorf("get github release version failed, status:%d", resp.StatusCode())
			return
		}
	} else {
		var releases []models.GitHubReleaseVersion
		resp, err := data.SharedHTTPClient.R().
			SetHeaders(githubApiHeaders).
			SetResult(&releases).
			Get("https://api.github.com/repos/ArvinLovegood/go-stock/releases")
		if err != nil {
			logger.SugaredLogger.Errorf("get github releases error:%s", err.Error())
			return
		}
		if resp.StatusCode() != 200 {
			logger.SugaredLogger.Errorf("get github releases failed, status:%d", resp.StatusCode())
			return
		}
		if len(releases) == 0 {
			logger.SugaredLogger.Errorf("no releases found")
			return
		}
		if updateChannel == "pre" {
			for _, r := range releases {
				if !r.Draft {
					releaseVersion = &r
					break
				}
			}
			if releaseVersion.TagName == "" {
				releaseVersion = &releases[0]
			}
		} else {
			releaseVersion = &releases[0]
		}
	}

	// VIP 策略已移除：外媒新闻同步对全部用户开放（原为 VIP2+ 专属）。
	go h.syncNews()

	if releaseVersion.TagName != h.version {
		tag := &models.Tag{}
		tagResp, tagErr := data.SharedHTTPClient.R().
			SetHeaders(githubApiHeaders).
			SetResult(tag).
			Get("https://api.github.com/repos/ArvinLovegood/go-stock/git/ref/tags/" + releaseVersion.TagName)
		if tagErr == nil && tagResp.StatusCode() == 200 && tag.Object.Url != "" {
			releaseVersion.Tag = *tag
			commit := &models.Commit{}
			commitResp, commitErr := data.SharedHTTPClient.R().
				SetHeaders(githubApiHeaders).
				SetResult(commit).
				Get(tag.Object.Url)
			if commitErr == nil && commitResp.StatusCode() == 200 {
				releaseVersion.Commit = *commit
			}
		}

		commitMessage := releaseVersion.Body
		if releaseVersion.Commit.Message != "" {
			commitMessage = releaseVersion.Commit.Message
		}

		downloadUrl := ""
		assetName := ""
		if h.isWindows() {
			if h.isArm64() {
				assetName = "go-stock-windows-arm64.exe"
			} else {
				assetName = "go-stock-windows-amd64.exe"
			}
		} else if h.isMacOS() {
			assetName = "go-stock-darwin-universal"
		} else if h.isLinux() {
			assetName = "go-stock-linux-amd64"
		}

		for _, asset := range releaseVersion.Assets {
			if asset.Name == assetName {
				downloadUrl = asset.BrowserDownloadUrl
				break
			}
		}

		if downloadUrl == "" {
			downloadUrl = fmt.Sprintf("https://github.com/ArvinLovegood/go-stock/releases/download/%s/%s", releaseVersion.TagName, assetName)
		}

		originalDownloadUrl := downloadUrl
		mirrorDownloadUrl := "https://gh.927223.xyz/" + originalDownloadUrl
		manualDownloadTip := fmt.Sprintf("\n手动下载链接(加速镜像): %s\n手动下载链接(原始地址): %s\n下载后请替换当前程序文件即可完成更新。", mirrorDownloadUrl, originalDownloadUrl)

		go wailsruntime.EventsEmit(h.currentCtx(), "newsPush", map[string]any{
			"time":    "发现新版本：" + releaseVersion.TagName,
			"isRed":   true,
			"source":  "go-stock",
			"content": commitMessage + "\n正在下载新版本，请耐心等待...",
		})

		tmpFile, err := os.CreateTemp("", "go-stock-update-*.tmp")
		if err != nil {
			logger.SugaredLogger.Errorf("create temp file error: %s", err.Error())
			go wailsruntime.EventsEmit(h.currentCtx(), "newsPush", map[string]any{
				"time":    "新版本：" + releaseVersion.TagName,
				"isRed":   true,
				"source":  "go-stock",
				"content": commitMessage + "\n新版本下载失败(无法创建临时文件)。" + manualDownloadTip,
			})
			return
		}
		tmpPath := tmpFile.Name()
		tmpFile.Close()
		defer os.Remove(tmpPath)

		downloadClient := data.CreateDownloadClient()

		downloadUrls := []string{mirrorDownloadUrl, downloadUrl}
		var downloadSuccess bool
		for _, url := range downloadUrls {
			_, err = downloadClient.R().
				SetHeader("User-Agent", "go-stock-updater").
				SetOutput(tmpPath).
				Get(url)
			if err != nil {
				logger.SugaredLogger.Warnf("download from %s error: %s, trying next...", url, err.Error())
				continue
			}
			fileInfo, statErr := os.Stat(tmpPath)
			if statErr != nil || fileInfo.Size() < 1024*500 {
				logger.SugaredLogger.Warnf("download from %s file size invalid, trying next...", url)
				continue
			}
			downloadSuccess = true
			break
		}

		if !downloadSuccess {
			go wailsruntime.EventsEmit(h.currentCtx(), "newsPush", map[string]any{
				"time":    "新版本：" + releaseVersion.TagName,
				"isRed":   true,
				"source":  "go-stock",
				"content": commitMessage + "\n新版本自动下载失败，请手动下载更新。" + manualDownloadTip,
			})
			return
		}

		body, err := os.ReadFile(tmpPath)
		if err != nil {
			logger.SugaredLogger.Errorf("read downloaded file error: %s", err.Error())
			go wailsruntime.EventsEmit(h.currentCtx(), "newsPush", map[string]any{
				"time":    "新版本：" + releaseVersion.TagName,
				"isRed":   true,
				"source":  "go-stock",
				"content": commitMessage + "\n新版本下载失败(无法读取临时文件)。" + manualDownloadTip,
			})
			return
		}

		err = update.Apply(bytes.NewReader(body), update.Options{})
		if err != nil {
			logger.SugaredLogger.Error("更新失败: ", err.Error())
			if !h.isRunningAsAdmin() {
				go wailsruntime.EventsEmit(h.currentCtx(), "updateNeedAdmin", map[string]any{
					"version": releaseVersion.TagName,
					"message": commitMessage,
				})
			} else {
				go wailsruntime.EventsEmit(h.currentCtx(), "updateVersion", releaseVersion)
			}
			return
		}
		go wailsruntime.EventsEmit(h.currentCtx(), "newsPush", map[string]any{
			"time":    "新版本：" + releaseVersion.TagName,
			"isRed":   true,
			"source":  "go-stock",
			"content": "版本更新完成,下次重启软件生效.",
		})
	} else {
		if flag == 1 {
			go wailsruntime.EventsEmit(h.currentCtx(), "newsPush", map[string]any{
				"time":    "当前版本：" + h.version,
				"isRed":   true,
				"source":  "go-stock",
				"content": "当前版本无更新",
			})
		}
	}
}

func (h *SystemHandler) syncNews() {
	defer panicHandler()
	client := data.SharedHTTPClient
	url := fmt.Sprintf("http://go-stock.sparkmemory.top:16666/FinancialNews/json?since=%d", time.Now().Add(-24*time.Hour).Unix())
	resp, err := client.R().SetDoNotParseResponse(true).Get(url)
	body := resp.RawBody()
	defer body.Close()
	if err != nil {
		logger.SugaredLogger.Errorf("syncNews error:%s", err.Error())
	}
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		news := &models.NtfyNews{}
		err := json.Unmarshal(scanner.Bytes(), news)
		if err != nil {
			return
		}
		dataTime := time.UnixMilli(int64(news.Time * 1000))

		if slice.ContainAny(news.Tags, []string{"外媒资讯", "财联社电报", "新浪财经", "外媒简讯", "外媒"}) {
			isRed := false
			if slice.Contain(news.Tags, "rotating_light") {
				isRed = true
			}
			telegraph := &models.Telegraph{
				Title:           news.Title,
				Content:         news.Message,
				DataTime:        &dataTime,
				IsRed:           isRed,
				Time:            dataTime.Format("15:04:05"),
				Source:          getSource(news.Tags),
				SentimentResult: data.AnalyzeSentiment(news.Message).Description,
			}
			cnt := int64(0)
			if telegraph.Title == "" {
				db.Dao.Model(telegraph).Where("content=?", telegraph.Content).Count(&cnt)
			} else {
				db.Dao.Model(telegraph).Where("title=?", telegraph.Title).Count(&cnt)
			}
			if cnt == 0 {
				db.Dao.Model(telegraph).Create(&telegraph)
				if time.Now().Sub(dataTime) < 5*time.Minute {
					h.newsPush(&[]models.Telegraph{*telegraph})
				}
				tags := slice.Filter(news.Tags, func(index int, item string) bool {
					return !(item == "rotating_light" || item == "loudspeaker")
				})
				for _, subject := range tags {
					tag := &models.Tags{
						Name: subject,
						Type: "subject",
					}
					db.Dao.Model(tag).Where("name=? and type=?", subject, "subject").FirstOrCreate(&tag)
					db.Dao.Model(models.TelegraphTags{}).Where("telegraph_id=? and tag_id=?", telegraph.ID, tag.ID).FirstOrCreate(&models.TelegraphTags{
						TelegraphId: telegraph.ID,
						TagId:       tag.ID,
					})
				}
			}
		}
	}
}

func (h *SystemHandler) newsPush(news *[]models.Telegraph) {
	follows := data.NewStockDataApi().GetFollowList(0)
	stockNames := slice.Map(*follows, func(index int, item data.FollowedStock) string {
		return item.Name
	})

	for _, telegraph := range *news {
		if h.GetConfig().EnableOnlyPushRedNews {
			if telegraph.IsRed || strutil.ContainsAny(telegraph.Content, stockNames) {
				go wailsruntime.EventsEmit(h.currentCtx(), "newsPush", telegraph)
			}
		} else {
			go wailsruntime.EventsEmit(h.currentCtx(), "newsPush", telegraph)
		}
	}
}

func getSource(tags []string) string {
	if slice.ContainAny(tags, []string{"外媒简讯", "外媒资讯", "外媒"}) {
		return "外媒"
	}
	if slice.Contain(tags, "财联社电报") {
		return "财联社电报"
	}
	if slice.Contain(tags, "新浪财经") {
		return "新浪财经"
	}
	return ""
}

func (h *SystemHandler) GetVersionInfo() *models.VersionInfo {
	return &models.VersionInfo{
		Version:           h.version,
		Icon:              getImageBase(h.icon),
		Alipay:            getImageBase(h.alipay),
		Wxpay:             getImageBase(h.wxpay),
		Wxgzh:             getImageBase(h.wxgzh),
		Content:           h.versionCommit,
		OfficialStatement: h.officialStatement,
	}
}

func (h *SystemHandler) GetUserManual() string {
	return string(h.userManual)
}

// OpenURL 跨平台打开默认浏览器
func (h *SystemHandler) OpenURL(url string) {
	wailsruntime.BrowserOpenURL(h.currentCtx(), url)
}

// GetTimezone 返回应用使用的时区信息（固定东八区）
func (h *SystemHandler) GetTimezone() map[string]any {
	return map[string]any{
		"offset":   8 * 60 * 60,
		"location": "Asia/Shanghai",
	}
}

func getImageBase(bytes []byte) string {
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(bytes)
}

func (h *SystemHandler) UpdateConfig(settingConfig *data.SettingConfig) string {
	if settingConfig.RefreshInterval > 0 {
		if entryID, exists := h.getCronEntry("MonitorStockPrices"); exists {
			h.cronScheduler.Remove(entryID)
		}
		id, _ := h.cronScheduler.AddFunc(fmt.Sprintf("@every %ds", settingConfig.RefreshInterval), func() {
			h.monitorStockPrices()
		})
		h.setCronEntry("MonitorStockPrices", id)
	}

	return data.UpdateConfig(settingConfig)
}

func (h *SystemHandler) GetConfig() *data.SettingConfig {
	return data.GetSettingConfig()
}

func (h *SystemHandler) ExportConfig() string {
	config := data.NewSettingsApi().Export()
	file, err := wailsruntime.SaveFileDialog(h.currentCtx(), wailsruntime.SaveDialogOptions{
		Title:                "导出配置文件",
		CanCreateDirectories: true,
		DefaultFilename:      "config.json",
	})
	if err != nil {
		logger.SugaredLogger.Errorf("导出配置文件失败:%s", err.Error())
		return err.Error()
	}
	err = os.WriteFile(file, []byte(config), os.ModePerm)
	if err != nil {
		logger.SugaredLogger.Errorf("导出配置文件失败:%s", err.Error())
		return err.Error()
	}
	return "导出成功:" + file
}

func (h *SystemHandler) GetAiConfigs() []*data.AIConfig {
	return data.GetSettingConfig().AiConfigs
}

func (h *SystemHandler) FetchAiModels(baseUrl, apiKey string) []string {
	baseUrl = strutil.Trim(baseUrl)
	apiKey = strutil.Trim(apiKey)
	if baseUrl == "" || apiKey == "" {
		return []string{}
	}

	type modelItem struct {
		ID string `json:"id"`
	}
	var respData struct {
		Data []modelItem `json:"data"`
	}

	client := data.SharedHTTPClient
	client.SetBaseURL(baseUrl)

	resp, err := client.R().
		SetHeader("Authorization", "Bearer "+apiKey).
		SetHeader("Content-Type", "application/json").
		SetResult(&respData).
		Get("/models")
	if err != nil {
		logger.SugaredLogger.Errorf("FetchAiModels error: %v", err)
		return []string{}
	}
	if resp.IsError() {
		logger.SugaredLogger.Errorf("FetchAiModels http error: %s", resp.Status())
		return []string{}
	}

	modelsList := make([]string, 0, len(respData.Data))
	for _, m := range respData.Data {
		if strings.TrimSpace(m.ID) != "" {
			modelsList = append(modelsList, m.ID)
		}
	}
	return modelsList
}

type AiModelInfo struct {
	ModelName string `json:"modelName"`
	MaxTokens int    `json:"maxTokens"`
	Source    string `json:"source"`
}

func (h *SystemHandler) FetchAiModelInfo(baseUrl, apiKey, modelName string) *AiModelInfo {
	baseUrl = strutil.Trim(baseUrl)
	modelName = strutil.Trim(modelName)
	if baseUrl == "" || modelName == "" {
		return nil
	}

	info := &AiModelInfo{
		ModelName: modelName,
		MaxTokens: 0,
		Source:    "",
	}

	if apiKey != "" {
		type modelDetail struct {
			ID             string `json:"id"`
			MaxContextLen  int    `json:"max_context_length"`
			ContextLength  int    `json:"context_length"`
			MaxOutputTok   int    `json:"max_output_tokens"`
			MaxTokensField int    `json:"max_tokens"`
		}
		var detail modelDetail

		client := data.SharedHTTPClient
		client.SetBaseURL(baseUrl)

		resp, err := client.R().
			SetHeader("Authorization", "Bearer "+apiKey).
			SetHeader("Content-Type", "application/json").
			SetResult(&detail).
			Get("/models/" + modelName)

		if err == nil && !resp.IsError() && detail.ID != "" {
			if detail.MaxContextLen > 0 {
				info.MaxTokens = detail.MaxContextLen
				info.Source = "api"
			} else if detail.ContextLength > 0 {
				info.MaxTokens = detail.ContextLength
				info.Source = "api"
			} else if detail.MaxOutputTok > 0 {
				info.MaxTokens = detail.MaxOutputTok
				info.Source = "api"
			} else if detail.MaxTokensField > 0 {
				info.MaxTokens = detail.MaxTokensField
				info.Source = "api"
			}
		}
	}

	if info.MaxTokens == 0 {
		if maxTokens := getBuiltinModelMaxTokens(modelName); maxTokens > 0 {
			info.MaxTokens = maxTokens
			info.Source = "builtin"
		}
	}

	return info
}

func getBuiltinModelMaxTokens(modelName string) int {
	modelTokenMap := map[string]int{
		"deepseek-chat":        65536,
		"deepseek-reasoner":    65536,
		"deepseek-coder":       16384,
		"deepseek-v3":          65536,
		"deepseek-r1":          65536,
		"gpt-4o":               16384,
		"gpt-4o-mini":          16384,
		"gpt-4o-2024-05-13":    4096,
		"gpt-4-turbo":          4096,
		"gpt-4-turbo-preview":  4096,
		"gpt-4":                8192,
		"gpt-4-32k":            32768,
		"gpt-3.5-turbo":        4096,
		"gpt-3.5-turbo-16k":    16384,
		"gpt-4.1":              32768,
		"gpt-4.1-mini":         32768,
		"gpt-4.1-nano":         32768,
		"o1":                   100000,
		"o1-mini":              65536,
		"o1-preview":           32768,
		"o3-mini":              100000,
		"o4-mini":              100000,
		"claude-3-5-sonnet":    8192,
		"claude-3-5-haiku":     8192,
		"claude-3-opus":        4096,
		"claude-3-sonnet":      4096,
		"claude-3-haiku":       4096,
		"glm-4":                8192,
		"glm-4-plus":           4096,
		"glm-4-air":            4096,
		"glm-4-flash":          4096,
		"glm-4-long":           4096,
		"chatglm-turbo":        4096,
		"moonshot-v1-8k":       8192,
		"moonshot-v1-32k":      32768,
		"moonshot-v1-128k":     131072,
		"qwen-turbo":           8192,
		"qwen-plus":            131072,
		"qwen-max":             8192,
		"qwen-long":            65536,
		"qwen2.5-72b-instruct": 32768,
		"hunyuan-lite":         4096,
		"hunyuan-standard":     4096,
		"hunyuan-pro":          4096,
		"hunyuan-turbo":        4096,
		"spark-lite":           4096,
		"spark-pro":            4096,
		"spark-max":            4096,
		"spark-4.0-ultra":      4096,
		"yi-light":             16384,
		"yi-large":             16384,
		"yi-medium":            16384,
		"yi-spark":             16384,
		"yi-vision":            16384,
		"abab6.5-chat":         8192,
		"abab6.5s-chat":        8192,
		"abab5.5-chat":         4096,
		"baichuan2-turbo":      4096,
		"baichuan2-53b":        4096,
		"ernie-4.0":            4096,
		"ernie-3.5":            4096,
		"ernie-speed":          4096,
		"ernie-lite":           4096,
	}

	if maxTokens, ok := modelTokenMap[modelName]; ok {
		return maxTokens
	}

	for prefix, maxTokens := range map[string]int{
		"deepseek":      65536,
		"gpt-4o":        16384,
		"gpt-4-turbo":   4096,
		"gpt-4-":        8192,
		"gpt-3.5":       4096,
		"gpt-4.1":       32768,
		"o1-":           65536,
		"o3-":           100000,
		"o4-":           100000,
		"claude-3":      8192,
		"glm-4":         8192,
		"chatglm":       4096,
		"moonshot-v1":   8192,
		"qwen-":         8192,
		"qwen2":         32768,
		"hunyuan-":      4096,
		"spark-":        4096,
		"yi-":           16384,
		"abab":          8192,
		"baichuan":      4096,
		"ernie-":        4096,
		"llama-3":       8192,
		"llama3":        8192,
		"mistral-":      8192,
		"mixtral-":      32768,
		"codestral-":    32768,
		"gemini-1.5":    8192,
		"gemini-2":      8192,
		"command-r":     4096,
		"Qwen/Qwen":     32768,
		"deepseek-ai/":  65536,
		"meta-llama/":   8192,
		"mistralai/":    32768,
		"Pro/deepseek-": 65536,
		"Pro/qwen-":     32768,
	} {
		if strings.HasPrefix(modelName, prefix) {
			return maxTokens
		}
	}

	return 0
}

func (h *SystemHandler) GetAiAssistantSession(sessionId string) (*models.AiAssistantSessionResp, error) {
	resp, err := h.svc.GetAiAssistantSession(context.Background(), sessionId)
	if err != nil {
		return nil, err
	}
	return sqlite.AiAssistantSessionRespFromDomain(resp), nil
}

func (h *SystemHandler) SaveAiAssistantSession(sessionId string, messages []models.AiAssistantMessage) error {
	return h.svc.SaveAiAssistantSession(context.Background(), sessionId, sqlite.AiAssistantMessagesToDomain(messages))
}

// -------------------- Cron --------------------

func (h *SystemHandler) InitCronTasks() {
	cronApi := agent.NewCronTaskApi()
	if !cronApi.ExistsByTaskType("stock_change_save") {
		task := &models.CronTask{
			Name:        "异动数据保存",
			CronExpr:    "0 */1 * * * *",
			TaskType:    "stock_change_save",
			Enable:      true,
			Status:      "active",
			Description: "每分钟自动保存A股异动数据（火箭发射、快速反弹、大笔买入、封涨停板等），交易时间外自动跳过",
		}
		err := cronApi.Create(task)
		if err != nil {
			logger.SugaredLogger.Errorf("自动创建异动数据保存任务失败：%v", err)
		} else {
			logger.SugaredLogger.Info("已自动创建异动数据保存定时任务")
		}
	}
	tasks := cronApi.GetAll()
	if len(tasks) == 0 {
		return
	}
	for _, t := range tasks {
		taskCopy := t
		entryID, err := h.cronScheduler.AddFunc(taskCopy.CronExpr, func() {
			err := agent.NewCronTaskApi().ExecuteTask(h.currentCtx(), &taskCopy)
			if err != nil {
				logger.SugaredLogger.Errorf("启动任务失败：%v %s", err, taskCopy.Name)
				return
			}
		})
		if err != nil {
			logger.SugaredLogger.Errorf("自动创建定时任务失败：%v %s", err, taskCopy.Name)
			continue
		}
		h.setCronEntry(convertor.ToString(taskCopy.ID)+"_"+taskCopy.Name, entryID)
	}
}

func (h *SystemHandler) AbortSummaryStockNews() {
	h.summaryMu.Lock()
	defer h.summaryMu.Unlock()
	if h.summaryCancel != nil {
		h.summaryCancel()
		h.summaryCancel = nil
	}
}

func (h *SystemHandler) CreateCronTask(task *models.CronTask) string {
	dtask := sqlite.CronTaskToDomain(task)
	err := h.svc.CreateCronTask(context.Background(), dtask)
	if err != nil {
		return fmt.Sprintf("创建失败：%v", err)
	}
	*task = *sqlite.CronTaskFromDomain(dtask)
	taskCopy := *task
	entryID, err := h.cronScheduler.AddFunc(taskCopy.CronExpr, func() {
		err := agent.NewCronTaskApi().ExecuteTask(h.currentCtx(), &taskCopy)
		if err != nil {
			logger.SugaredLogger.Errorf("执行任务失败：%v %s", err, taskCopy.Name)
			return
		}
	})
	h.setCronEntry(convertor.ToString(task.ID)+"_"+task.Name, entryID)
	if err != nil {
		return "任务创建成功,但定时失败"
	}
	return "创建成功"
}

func (h *SystemHandler) UpdateCronTask(task *models.CronTask) string {
	err := h.svc.UpdateCronTask(context.Background(), sqlite.CronTaskToDomain(task))
	if err != nil {
		return fmt.Sprintf("更新失败：%v", err)
	}
	if entryID, exists := h.getCronEntry(convertor.ToString(task.ID) + "_" + task.Name); exists {
		h.cronScheduler.Remove(entryID)
	}
	taskCopy := *task
	entryID, err := h.cronScheduler.AddFunc(taskCopy.CronExpr, func() {
		err := agent.NewCronTaskApi().ExecuteTask(h.currentCtx(), &taskCopy)
		if err != nil {
			logger.SugaredLogger.Errorf("执行任务失败：%v %s", err, taskCopy.Name)
			return
		}
	})
	h.setCronEntry(convertor.ToString(task.ID)+"_"+task.Name, entryID)
	if err != nil {
		return fmt.Sprintf("更新失败：%v", err)
	}
	return "更新成功"
}

func (h *SystemHandler) DeleteCronTask(id uint) string {
	err := h.svc.DeleteCronTask(context.Background(), id)
	task, err := h.svc.GetCronTaskByID(context.Background(), id)
	if err == nil {
		if entryID, exists := h.getCronEntry(convertor.ToString(id) + "_" + task.Name); exists {
			h.cronScheduler.Remove(entryID)
		}
	}
	if err != nil {
		return fmt.Sprintf("删除失败：%v", err)
	}
	return "删除成功"
}

func (h *SystemHandler) GetCronTaskByID(id uint) *models.CronTask {
	task, err := h.svc.GetCronTaskByID(context.Background(), id)
	if err != nil {
		return nil
	}
	return sqlite.CronTaskFromDomain(task)
}

func (h *SystemHandler) GetCronTaskList(query *models.CronTaskQuery) *models.CronTaskPageResp {
	resp, err := h.svc.GetCronTaskList(context.Background(), sqlite.CronTaskQueryToDomain(query))
	if err != nil {
		return nil
	}
	return sqlite.CronTaskPageRespFromDomain(resp)
}

func (h *SystemHandler) EnableCronTask(id uint, enable bool) string {
	err := h.svc.EnableCronTask(context.Background(), id, enable)
	dtask, err := h.svc.GetCronTaskByID(context.Background(), id)
	if err == nil {
		task := sqlite.CronTaskFromDomain(dtask)
		if entryID, exists := h.getCronEntry(convertor.ToString(id) + "_" + task.Name); exists {
			h.cronScheduler.Remove(entryID)
		}
		if enable {
			taskCopy := *task
			entryID, err := h.cronScheduler.AddFunc(taskCopy.CronExpr, func() {
				err := agent.NewCronTaskApi().ExecuteTask(h.currentCtx(), &taskCopy)
				if err != nil {
					logger.SugaredLogger.Errorf("%s 执行任务失败：%v", taskCopy.Name, err)
					return
				}
			})
			h.setCronEntry(convertor.ToString(id)+"_"+task.Name, entryID)
			if err != nil {
				return "操作成功,但定时失败"
			}
		}
	}
	if err != nil {
		return fmt.Sprintf("操作失败：%v", err)
	}
	return "操作成功"
}

func (h *SystemHandler) ExecuteCronTaskNow(id uint) string {
	task, err := agent.NewCronTaskApi().GetByID(id)
	if err != nil {
		return fmt.Sprintf("任务不存在：%v", err)
	}

	go func() {
		err := agent.NewCronTaskApi().ExecuteTask(h.currentCtx(), task)
		if err != nil {
			logger.SugaredLogger.Errorf("执行任务失败：%v %s", err, task.Name)
		}
	}()

	return "任务执行中"
}

func (h *SystemHandler) GetCronTaskTypes() []lo.Tuple2[string, string] {
	return agent.NewCronTaskApi().GetTaskTypes()
}

func (h *SystemHandler) ValidateCronExpr(expr string) string {
	err := agent.NewCronTaskApi().ValidateCronExpr(expr)
	if err != nil {
		return fmt.Sprintf("无效表达式：%v", err)
	}
	return "有效表达式"
}

func (h *SystemHandler) SearchCronTasks(keyword string) []models.CronTask {
	tasks, _ := h.svc.SearchCronTasks(context.Background(), keyword)
	return sqlite.CronTaskListFromDomain(tasks)
}

func (h *SystemHandler) CalculateNextRunTime(cron string) string {
	nextRunTime := agent.NewCronTaskApi().CalculateNextRunTime(cron)
	return nextRunTime.Format("2006-01-02 15:04:05")
}

func (h *SystemHandler) CalculateNextRunTimes(cron string, count int) []string {
	times := agent.NewCronTaskApi().CalculateNextRunTimes(cron, count)
	result := make([]string, 0, len(times))
	for _, t := range times {
		result = append(result, t.Format("2006-01-02 15:04:05"))
	}
	return result
}

// -------------------- MCP / Skills --------------------

func (h *SystemHandler) CreateMCPServer(server *models.MCPServer) string {
	err := h.svc.CreateMCPServer(context.Background(), sqlite.MCPServerToDomain(server))
	if err != nil {
		logger.SugaredLogger.Errorf("创建MCP服务器失败: %v", err)
		return "创建失败: " + err.Error()
	}
	return "创建成功"
}

func (h *SystemHandler) UpdateMCPServer(server *models.MCPServer) string {
	err := h.svc.UpdateMCPServer(context.Background(), sqlite.MCPServerToDomain(server))
	if err != nil {
		logger.SugaredLogger.Errorf("更新MCP服务器失败: %v", err)
		return "更新失败: " + err.Error()
	}
	return "更新成功"
}

func (h *SystemHandler) DeleteMCPServer(id uint) string {
	err := h.svc.DeleteMCPServer(context.Background(), id)
	if err != nil {
		logger.SugaredLogger.Errorf("删除MCP服务器失败: %v", err)
		return "删除失败: " + err.Error()
	}
	return "删除成功"
}

func (h *SystemHandler) GetMCPServerByID(id uint) *models.MCPServer {
	server, err := h.svc.GetMCPServerByID(context.Background(), id)
	if err != nil {
		logger.SugaredLogger.Errorf("获取MCP服务器失败: %v", err)
		return nil
	}
	return sqlite.MCPServerFromDomain(server)
}

func (h *SystemHandler) GetMCPServerList(query *models.MCPServerQuery) *models.MCPServerPageResp {
	resp, err := h.svc.GetMCPServerList(context.Background(), sqlite.MCPServerQueryToDomain(query))
	if err != nil {
		return nil
	}
	return sqlite.MCPServerPageRespFromDomain(resp)
}

func (h *SystemHandler) EnableMCPServer(id uint, enable bool) string {
	err := h.svc.EnableMCPServer(context.Background(), id, enable)
	if err != nil {
		logger.SugaredLogger.Errorf("启用/禁用MCP服务器失败: %v", err)
		return "操作失败: " + err.Error()
	}
	if enable {
		return "已启用"
	}
	return "已禁用"
}

func (h *SystemHandler) TestMCPServer(id uint) string {
	result, err := data.NewMCPServerApi().TestConnection(id)
	if err != nil {
		logger.SugaredLogger.Errorf("测试MCP服务器连接失败: %v", err)
		return "测试失败: " + err.Error()
	}
	return result
}

func (h *SystemHandler) GetMCPToolsByServerID(serverID uint) []models.MCPServerTool {
	tools, _ := h.svc.GetMCPToolsByServerID(context.Background(), serverID)
	return sqlite.MCPServerToolListFromDomain(tools)
}

func (h *SystemHandler) GetAllMCPTools() []models.MCPServerTool {
	tools, _ := h.svc.GetAllMCPTools(context.Background())
	return sqlite.MCPServerToolListFromDomain(tools)
}

func (h *SystemHandler) CreateSkill(skill *models.Skill) string {
	err := h.svc.CreateSkill(context.Background(), sqlite.SkillToDomain(skill))
	if err != nil {
		logger.SugaredLogger.Errorf("创建技能失败: %v", err)
		return "创建失败: " + err.Error()
	}
	return "创建成功"
}

func (h *SystemHandler) UpdateSkill(skill *models.Skill) string {
	err := h.svc.UpdateSkill(context.Background(), sqlite.SkillToDomain(skill))
	if err != nil {
		logger.SugaredLogger.Errorf("更新技能失败: %v", err)
		return "更新失败: " + err.Error()
	}
	return "更新成功"
}

func (h *SystemHandler) DeleteSkill(id uint) string {
	err := h.svc.DeleteSkill(context.Background(), id)
	if err != nil {
		logger.SugaredLogger.Errorf("删除技能失败: %v", err)
		return "删除失败: " + err.Error()
	}
	return "删除成功"
}

func (h *SystemHandler) GetSkillByID(id uint) *models.Skill {
	skill, err := h.svc.GetSkillByID(context.Background(), id)
	if err != nil {
		logger.SugaredLogger.Errorf("获取技能失败: %v", err)
		return nil
	}
	return sqlite.SkillFromDomain(skill)
}

func (h *SystemHandler) GetSkillList(query *models.SkillQuery) *models.SkillPageResp {
	resp, err := h.svc.GetSkillList(context.Background(), sqlite.SkillQueryToDomain(query))
	if err != nil {
		return nil
	}
	return sqlite.SkillPageRespFromDomain(resp)
}

func (h *SystemHandler) EnableSkill(id uint, enable bool) string {
	err := h.svc.EnableSkill(context.Background(), id, enable)
	if err != nil {
		logger.SugaredLogger.Errorf("启用/禁用技能失败: %v", err)
		return "操作失败: " + err.Error()
	}
	if enable {
		return "已启用"
	}
	return "已禁用"
}

func (h *SystemHandler) GetAllSkills() []models.Skill {
	skills, _ := h.svc.GetAllSkills(context.Background())
	return sqlite.SkillListFromDomain(skills)
}

type einoLLMClient struct {
	model model.ToolCallingChatModel
}

func (c *einoLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	msg, err := c.model.Generate(ctx, []*schema.Message{
		{Role: schema.User, Content: prompt},
	})
	if err != nil {
		return "", err
	}
	if msg == nil {
		return "", fmt.Errorf("no response from LLM")
	}
	return msg.Content, nil
}

func (h *SystemHandler) GenerateSkillFromURL(url string) (*models.Skill, float64, error) {
	configs := data.GetSettingConfig().AiConfigs
	if len(configs) == 0 {
		return nil, 0, fmt.Errorf("请先配置 AI 模型")
	}
	cfg := configs[0]
	chatModel, err := agent.CreateChatModel(h.currentCtx(), *cfg)
	if err != nil {
		return nil, 0, fmt.Errorf("创建 AI 模型失败: %s", err.Error())
	}
	llm := &einoLLMClient{model: chatModel}
	skill, confidence, err := skill_analysis.GenerateSkillFromURL(h.currentCtx(), url, llm)
	if err != nil {
		return nil, 0, fmt.Errorf("生成失败: %s", err.Error())
	}
	if skill == nil {
		return nil, 0, fmt.Errorf("生成失败：未获取到有效结果")
	}
	err = data.NewSkillApi().Create(skill)
	if err != nil {
		return nil, 0, fmt.Errorf("保存技能失败: %s", err.Error())
	}
	return skill, confidence, nil
}

func (h *SystemHandler) AnalyzeSkillEffectiveness(id uint) string {
	skill, err := data.NewSkillApi().GetByID(id)
	if err != nil {
		return "未找到指定技能"
	}
	var totalUsage int64
	db.Dao.Model(&models.SkillUsageRecord{}).Where("skill_id = ?", id).Count(&totalUsage)
	return fmt.Sprintf(`技能: %s
分类: %s
总使用次数: %d
平均分: %.2f
置信度: %.2f
来源: %s
版本: %d`,
		skill.Name, skill.Category, totalUsage, skill.AvgScore, skill.Confidence, skill.Source, skill.Version)
}

// -------------------- Internal helpers --------------------

func (h *SystemHandler) monitorStockPrices() {
	isAStockOpen := isTradingTime(time.Now())
	isHKStockOpen := isHKTradingTime(time.Now())
	isUSStockOpen := isUSTradingTime(time.Now())

	if !isAStockOpen && !isHKStockOpen && !isUSStockOpen {
		return
	}

	dest := &[]data.FollowedStock{}
	db.Dao.Model(&data.FollowedStock{}).Find(dest)
	total := float64(0)

	stockCodes := make([]string, 0)
	for _, follow := range *dest {
		if strutil.HasPrefixAny(follow.StockCode, []string{"SZ", "SH", "sh", "sz"}) && !isTradingTime(time.Now()) {
			continue
		}
		if strutil.HasPrefixAny(follow.StockCode, []string{"hk", "HK"}) && !isHKTradingTime(time.Now()) {
			continue
		}
		if strutil.HasPrefixAny(follow.StockCode, []string{"us", "US", "gb_"}) && !isUSTradingTime(time.Now()) {
			continue
		}
		stockCodes = append(stockCodes, follow.StockCode)
	}

	stockDatas, err := data.NewStockDataApi().GetStockCodeRealTimeData(stockCodes...)
	if err != nil || stockDatas == nil {
		return
	}
	for _, stockInfo := range *stockDatas {
		if strutil.HasPrefixAny(stockInfo.Code, []string{"SZ", "SH", "sh", "sz"}) && !isTradingTime(time.Now()) {
			continue
		}
		if strutil.HasPrefixAny(stockInfo.Code, []string{"hk", "HK"}) && !isHKTradingTime(time.Now()) {
			continue
		}
		if strutil.HasPrefixAny(stockInfo.Code, []string{"us", "US", "gb_"}) && !isUSTradingTime(time.Now()) {
			continue
		}
		total += stockInfo.ProfitAmountToday
		price, _ := convertor.ToFloat(stockInfo.Price)
		if stockInfo.PrePrice != price {
			go wailsruntime.EventsEmit(h.currentCtx(), "stock_price", stockInfo)
		}
	}

	go wailsruntime.EventsEmit(h.currentCtx(), "realtime_profit", fmt.Sprintf("  %.2f", total))
}

func panicHandler() {
	if r := recover(); r != nil {
		fmt.Printf("Recovered from panic: %v\n", r)
	}
}

func (h *SystemHandler) isWindows() bool {
	return stdruntime.GOOS == "windows"
}

func (h *SystemHandler) isLinux() bool {
	return stdruntime.GOOS == "linux"
}

func (h *SystemHandler) isMacOS() bool {
	return stdruntime.GOOS == "darwin"
}

func (h *SystemHandler) isArm64() bool {
	return stdruntime.GOARCH == "arm64"
}

func (h *SystemHandler) isRunningAsAdmin() bool {
	return isRunningAsAdmin()
}
