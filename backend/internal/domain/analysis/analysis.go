package analysis

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/plugin/soft_delete"
)

// AIResponseResult AI分析响应结果
type AIResponseResult struct {
	gorm.Model
	ChatId    string                `json:"chatId"`
	ModelName string                `json:"modelName"`
	StockCode string                `json:"stockCode"`
	StockName string                `json:"stockName"`
	Question  string                `json:"question"`
	Content   string                `json:"content"`
	IsDel     soft_delete.DeletedAt `gorm:"softDelete:flag"`
}

func (AIResponseResult) TableName() string {
	return "ai_response_result"
}

// AIResponseResultQuery 分页查询参数
type AIResponseResultQuery struct {
	Page      int    `form:"page" json:"page"`           // 页码
	PageSize  int    `form:"pageSize" json:"pageSize"`   // 每页大小
	ChatId    string `form:"chatId" json:"chatId"`       // 聊天ID筛选
	ModelName string `form:"modelName" json:"modelName"` // 模型名称筛选
	StockCode string `form:"stockCode" json:"stockCode"` // 股票代码筛选
	StockName string `form:"stockName" json:"stockName"` // 股票名称筛选
	Question  string `form:"question" json:"question"`   // 问题内容模糊搜索
	StartDate string `form:"startDate" json:"startDate"` // 开始日期
	EndDate   string `form:"endDate" json:"endDate"`     // 结束日期
}

// AIResponseResultPageResp 分页查询响应
type AIResponseResultPageResp struct {
	Code    int                      `json:"code"`
	Message string                   `json:"message"`
	Data    AIResponseResultPageData `json:"data"`
}

// AIResponseResultPageData 分页数据
type AIResponseResultPageData struct {
	List       []AIResponseResult `json:"list"`
	Total      int64              `json:"total"`
	Page       int                `json:"page"`
	PageSize   int                `json:"pageSize"`
	TotalPages int                `json:"totalPages"`
}

// PromptTemplate 提示词模板
type PromptTemplate struct {
	ID        int `gorm:"primarykey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	Name      string `json:"name"`
	Content   string `json:"content"`
	Type      string `json:"type"`
	RoleKey   string `json:"roleKey" gorm:"uniqueIndex;size:100"` // 多智能体角色键名，如 multi_fundamental
}

func (PromptTemplate) TableName() string {
	return "prompt_templates"
}

// PromptTemplateQuery 分页查询参数
type PromptTemplateQuery struct {
	Page     int    `form:"page" json:"page"`         // 页码
	PageSize int    `form:"pageSize" json:"pageSize"` // 每页大小
	Name     string `form:"name" json:"name"`         // 模板名称筛选
	Type     string `form:"type" json:"type"`         // 模板类型筛选
	Content  string `form:"content" json:"content"`   // 内容模糊搜索
}

// PromptTemplatePageResp 分页查询响应
type PromptTemplatePageResp struct {
	Code    int                    `json:"code"`
	Message string                 `json:"message"`
	Data    PromptTemplatePageData `json:"data"`
}

// PromptTemplatePageData 分页数据
type PromptTemplatePageData struct {
	List       []PromptTemplate `json:"list"`
	Total      int64            `json:"total"`
	Page       int              `json:"page"`
	PageSize   int              `json:"pageSize"`
	TotalPages int              `json:"totalPages"`
}

// Prompt 提示词
type Prompt struct {
	ID      int    `json:"ID"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Type    string `json:"type"`
}

// InteractiveAnswer 互动问答结果
type InteractiveAnswer struct {
	PageNo      int                        `json:"pageNo"`
	PageSize    int                        `json:"pageSize"`
	TotalRecord int                        `json:"totalRecord"`
	TotalPage   int                        `json:"totalPage"`
	Results     []InteractiveAnswerResults `json:"results"`
	Count       bool                       `json:"count"`
}

// InteractiveAnswerResults 互动问答项
type InteractiveAnswerResults struct {
	EsId             string   `json:"esId" md:"-"`
	IndexId          string   `json:"indexId" md:"-"`
	ContentType      int      `json:"contentType" md:"-"`
	Trade            []string `json:"trade"  md:"行业名称"`
	MainContent      string   `json:"mainContent" md:"投资者提问"`
	StockCode        string   `json:"stockCode" md:"股票代码"`
	Secid            string   `json:"secid" md:"-"`
	CompanyShortName string   `json:"companyShortName" md:"股票名称"`
	CompanyLogo      string   `json:"companyLogo,omitempty" md:"-"`
	BoardType        []string `json:"boardType" md:"-"`
	PubDate          string   `json:"pubDate" md:"发布时间"`
	UpdateDate       string   `json:"updateDate" md:"-"`
	Author           string   `json:"author" md:"-"`
	AuthorName       string   `json:"authorName" md:"-"`
	PubClient        string   `json:"pubClient" md:"-"`
	AttachedId       string   `json:"attachedId" md:"-"`
	AttachedContent  string   `json:"attachedContent" md:"上市公司回复"`
	AttachedAuthor   string   `json:"attachedAuthor" md:"-"`
	AttachedPubDate  string   `json:"attachedPubDate" md:"回复时间"`
	Score            float64  `json:"score" md:"-"`
	TopStatus        int      `json:"topStatus" md:"-"`
	PraiseCount      int      `json:"praiseCount" md:"-"`
	PraiseStatus     bool     `json:"praiseStatus" md:"-"`
	FavoriteStatus   bool     `json:"favoriteStatus" md:"-"`
	AttentionCompany bool     `json:"attentionCompany" md:"-"`
	IsCheck          string   `json:"isCheck" md:"-"`
	QaStatus         int      `json:"qaStatus" md:"-"`
	PackageDate      string   `json:"packageDate" md:"-"`
	RemindStatus     bool     `json:"remindStatus" md:"-"`
	InterviewLive    bool     `json:"interviewLive" md:"-"`
}

// CailianpressWeb 财联社网页资讯
type CailianpressWeb struct {
	Total int `json:"total"`
	List  []struct {
		Title   string `json:"title" md:"资讯标题"`
		Ctime   int    `json:"ctime" md:"资讯时间"`
		Content string `json:"content" md:"资讯内容"`
		Author  string `json:"author" md:"资讯发布者"`
	} `json:"list"`
}

// WordAnalyze 词频分析
type WordAnalyze struct {
	gorm.Model
	DataTime *time.Time `json:"dataTime" gorm:"index;autoCreateTime"`
	WordFreqWithWeight
}

// WordFreqWithWeight 词频统计结果，包含权重信息
type WordFreqWithWeight struct {
	Word      string
	Frequency int
	Weight    float64
	Score     float64
}
