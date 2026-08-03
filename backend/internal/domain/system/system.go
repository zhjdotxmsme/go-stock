package system

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/plugin/soft_delete"
)

// VersionInfo 版本信息
type VersionInfo struct {
	gorm.Model
	Version           string                `json:"version"`
	Content           string                `json:"content"`
	Icon              string                `json:"icon"`
	Alipay            string                `json:"alipay"`
	Wxpay             string                `json:"wxpay"`
	Wxgzh             string                `json:"wxgzh"`
	BuildTimeStamp    int64                 `json:"buildTimeStamp"`
	OfficialStatement string                `json:"officialStatement"`
	IsDel             soft_delete.DeletedAt `gorm:"softDelete:flag"`
}

func (VersionInfo) TableName() string {
	return "version_info"
}

// GitHubReleaseVersion GitHub 发布版本
type GitHubReleaseVersion struct {
	Url       string `json:"url"`
	AssetsUrl string `json:"assets_url"`
	UploadUrl string `json:"upload_url"`
	HtmlUrl   string `json:"html_url"`
	Id        int    `json:"id"`
	Author    struct {
		Login             string `json:"login"`
		Id                int    `json:"id"`
		NodeId            string `json:"node_id"`
		AvatarUrl         string `json:"avatar_url"`
		GravatarId        string `json:"gravatar_id"`
		Url               string `json:"url"`
		HtmlUrl           string `json:"html_url"`
		FollowersUrl      string `json:"followers_url"`
		FollowingUrl      string `json:"following_url"`
		GistsUrl          string `json:"gists_url"`
		StarredUrl        string `json:"starred_url"`
		SubscriptionsUrl  string `json:"subscriptions_url"`
		OrganizationsUrl  string `json:"organizations_url"`
		ReposUrl          string `json:"repos_url"`
		EventsUrl         string `json:"events_url"`
		ReceivedEventsUrl string `json:"received_events_url"`
		Type              string `json:"type"`
		UserViewType      string `json:"user_view_type"`
		SiteAdmin         bool   `json:"site_admin"`
	} `json:"author"`
	NodeId          string    `json:"node_id"`
	TagName         string    `json:"tag_name"`
	TargetCommitish string    `json:"target_commitish"`
	Name            string    `json:"name"`
	Draft           bool      `json:"draft"`
	Prerelease      bool      `json:"prerelease"`
	CreatedAt       time.Time `json:"created_at"`
	PublishedAt     time.Time `json:"published_at"`
	Assets          []struct {
		Url      string `json:"url"`
		Id       int    `json:"id"`
		NodeId   string `json:"node_id"`
		Name     string `json:"name"`
		Label    string `json:"label"`
		Uploader struct {
			Login             string `json:"login"`
			Id                int    `json:"id"`
			NodeId            string `json:"node_id"`
			AvatarUrl         string `json:"avatar_url"`
			GravatarId        string `json:"gravatar_id"`
			Url               string `json:"url"`
			HtmlUrl           string `json:"html_url"`
			FollowersUrl      string `json:"followers_url"`
			FollowingUrl      string `json:"following_url"`
			GistsUrl          string `json:"gists_url"`
			StarredUrl        string `json:"starred_url"`
			SubscriptionsUrl  string `json:"subscriptions_url"`
			OrganizationsUrl  string `json:"organizations_url"`
			ReposUrl          string `json:"repos_url"`
			EventsUrl         string `json:"events_url"`
			ReceivedEventsUrl string `json:"received_events_url"`
			Type              string `json:"type"`
			UserViewType      string `json:"user_view_type"`
			SiteAdmin         bool   `json:"site_admin"`
		} `json:"uploader"`
		ContentType        string    `json:"content_type"`
		State              string    `json:"state"`
		Size               int       `json:"size"`
		DownloadCount      int       `json:"download_count"`
		CreatedAt          time.Time `json:"created_at"`
		UpdatedAt          time.Time `json:"updated_at"`
		BrowserDownloadUrl string    `json:"browser_download_url"`
	} `json:"assets"`
	TarballUrl string `json:"tarball_url"`
	ZipballUrl string `json:"zipball_url"`
	Body       string `json:"body"`
	Tag        Tag    `json:"tag"`
	Commit     Commit `json:"commit"`
}

// Tag Git 标签
type Tag struct {
	Ref    string `json:"ref"`
	NodeId string `json:"node_id"`
	Url    string `json:"url"`
	Object struct {
		Sha  string `json:"sha"`
		Type string `json:"type"`
	} `json:"object"`
}

// Commit Git 提交
type Commit struct {
	Sha    string `json:"sha"`
	NodeId string `json:"node_id"`
	Url    string `json:"url"`
}

// OldSettings 设置配置（兼容旧版）
type OldSettings struct {
	gorm.Model
	TushareToken           string `json:"tushareToken"`
	LocalPushEnable        bool   `json:"localPushEnable"`
	DingPushEnable         bool   `json:"dingPushEnable"`
	DingRobot              string `json:"dingRobot"`
	UpdateBasicInfoOnStart bool   `json:"updateBasicInfoOnStart"`
	RefreshInterval        int64  `json:"refreshInterval"`

	OpenAiEnable      bool    `json:"openAiEnable"`
	OpenAiBaseUrl     string  `json:"openAiBaseUrl"`
	OpenAiApiKey      string  `json:"openAiApiKey"`
	OpenAiModelName   string  `json:"openAiModelName"`
	OpenAiMaxTokens   int     `json:"openAiMaxTokens"`
	OpenAiTemperature float64 `json:"openAiTemperature"`
	OpenAiApiTimeOut  int     `json:"openAiApiTimeOut"`
	Prompt            string  `json:"prompt"`
	CheckUpdate       bool    `json:"checkUpdate"`
	QuestionTemplate  string  `json:"questionTemplate"`
	CrawlTimeOut      int64   `json:"crawlTimeOut"`
	KDays             int64   `json:"kDays"`
	EnableDanmu       bool    `json:"enableDanmu"`
	BrowserPath       string  `json:"browserPath"`
	EnableNews        bool    `json:"enableNews"`
	DarkTheme         bool    `json:"darkTheme"`
	BrowserPoolSize   int     `json:"browserPoolSize"`
	EnableFund        bool    `json:"enableFund"`
	EnablePushNews    bool    `json:"enablePushNews"`
	SponsorCode       string  `json:"sponsorCode"`
}

func (OldSettings) TableName() string {
	return "settings"
}

// CronTask 定时任务
type CronTask struct {
	ID            uint       `json:"id" gorm:"primarykey"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
	Name          string     `json:"name" gorm:"size:255;not null"`
	CronExpr      string     `json:"cronExpr" gorm:"size:100;not null"`
	TaskType      string     `json:"taskType" gorm:"size:50;not null"` // stock_analysis, fund_analysis, news_fetch, custom
	Target        string     `json:"target" gorm:"size:255"`           // 股票代码或其他目标
	Params        string     `json:"params" gorm:"type:text"`          // JSON 格式的任务参数
	Enable        bool       `json:"enable" gorm:"default:true"`
	LastRunAt     *time.Time `json:"lastRunAt"`
	NextRunAt     *time.Time `json:"nextRunAt"`
	RunCount      int64      `json:"runCount" gorm:"default:0"`
	Status        string     `json:"status" gorm:"size:20;default:active"` // active, paused, error
	Description   string     `json:"description" gorm:"size:500"`
	LastRunResult string     `json:"lastRunResult" gorm:"size:500"`
}

func (CronTask) TableName() string {
	return "cron_tasks"
}

// CronTaskQuery 定时任务查询参数
type CronTaskQuery struct {
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	Name     string `json:"name"`
	TaskType string `json:"taskType"`
	Status   string `json:"status"`
	Enable   *bool  `json:"enable"`
}

// CronTaskPageResp 定时任务分页响应
type CronTaskPageResp struct {
	Total int        `json:"total"`
	Data  []CronTask `json:"data"`
}

// CronTaskPageData 定时任务分页数据
type CronTaskPageData struct {
	List       []CronTask `json:"list"`
	TotalCount int        `json:"totalCount"`
	Page       int        `json:"page"`
	PageSize   int        `json:"pageSize"`
}
