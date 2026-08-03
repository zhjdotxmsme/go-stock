package market

import (
	"time"

	"gorm.io/gorm"
)

// Telegraph 电报消息
type Telegraph struct {
	gorm.Model
	Time            string          `json:"time"`
	DataTime        *time.Time      `json:"dataTime" gorm:"index"`
	Title           string          `json:"title" gorm:"index"`
	Content         string          `json:"content" gorm:"index"`
	SubjectTags     []string        `json:"subjects" gorm:"-:all"`
	StocksTags      []string        `json:"stocks" gorm:"-:all"`
	IsRed           bool            `json:"isRed" gorm:"index"`
	Url             string          `json:"url"`
	Source          string          `json:"source" gorm:"index"`
	TelegraphTags   []TelegraphTags `json:"tags" gorm:"-:migration;foreignKey:TelegraphId"`
	SentimentResult string          `json:"sentimentResult" gorm:"index"`
}

func (Telegraph) TableName() string {
	return "telegraph_list"
}

// TelegraphTags 电报标签关联
type TelegraphTags struct {
	gorm.Model
	TagId       uint `json:"tagId"`
	TelegraphId uint `json:"telegraphId"`
}

func (TelegraphTags) TableName() string {
	return "telegraph_tags"
}

// Tags 标签
type Tags struct {
	gorm.Model
	Name string `json:"name"`
	Type string `json:"type"`
}

func (Tags) TableName() string {
	return "tags"
}

// TVNews 电视新闻
type TVNews struct {
	Id         string `json:"id"`
	Title      string `json:"title"`
	Published  int    `json:"published"`
	Urgency    int    `json:"urgency"`
	Permission string `json:"permission"`
	StoryPath  string `json:"storyPath"`
	Provider   struct {
		Id     string `json:"id"`
		Name   string `json:"name"`
		LogoId string `json:"logo_id"`
	} `json:"provider"`
}

// TVNewsDetail 电视新闻详情
type TVNewsDetail struct {
	ShortDescription string `json:"shortDescription"`
	Tags             []struct {
		Title string `json:"title"`
		Args  []struct {
			Id    string `json:"id"`
			Value string `json:"value"`
		} `json:"args"`
	} `json:"tags"`
	Copyright string `json:"copyright"`
	Id        string `json:"id"`
	Title     string `json:"title"`
	Published int    `json:"published"`
	Urgency   int    `json:"urgency"`
	StoryPath string `json:"storyPath"`
}

// ReutersNews 路透新闻
type ReutersNews struct {
	Body        string `json:"body"`
	Headline    string `json:"headline"`
	Link        string `json:"link"`
	PictureLink string `json:"picture_link"`
	PubDate     string `json:"pub_date"`
}
