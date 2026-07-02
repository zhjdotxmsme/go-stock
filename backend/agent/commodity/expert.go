package commodity

import "context"

type Expert interface {
	Role() string
	Run(ctx context.Context, cc *CommodityContext) (*ExpertReport, error)
}

var defaultExperts []Expert

func RegisterExpert(e Expert) {
	defaultExperts = append(defaultExperts, e)
}

func GetDefaultExperts() []Expert {
	return defaultExperts
}
