package main

import (
	"time"
)

// @Author spark
// @Date 2025/6/8 20:45
// @Desc
//--------------------------------------------------------------------------------

var ShanghaiTimezone = time.FixedZone("CST", 8*60*60)

func GetShanghaiTime() time.Time {
	return time.Now().In(ShanghaiTimezone)
}

func FormatShanghaiTime(t time.Time) string {
	if ShanghaiTimezone == nil {
		return t.Format("2006-01-02 15:04:05")
	}
	return t.In(ShanghaiTimezone).Format("2006-01-02 15:04:05")
}
