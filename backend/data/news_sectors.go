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
