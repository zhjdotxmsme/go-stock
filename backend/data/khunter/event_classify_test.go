package khunter

import "testing"

func TestClassifyAnnouncement(t *testing.T) {
	cases := []struct {
		title    string
		wantType string
		wantNil  bool
	}{
		{"2026年半年度业绩预告：净利润同比增长80%", "业绩预增", false},
		{"2026年一季度业绩预告：净利润同比增长20%", "业绩略增", false},
		{"关于控股股东增持公司股份的公告", "大股东增持", false},
		{"关于部分董事增持计划的公告", "股东增持", false},
		{"关于回购公司股份方案的公告", "股票回购", false},
		{"2026年年度业绩预告：预计首亏", "业绩暴雷", false},
		{"2026年业绩预告：净利润预减60%", "业绩预减", false},
		{"关于持股5%以上股东减持计划的公告", "大股东减持", false},
		{"关于股东减持股份结果公告", "股东减持", false},
		{"股票交易异常波动公告", "异常波动", false},
		{"关于召开股东大会的通知", "", true},
	}
	for _, c := range cases {
		ev := ClassifyAnnouncement(c.title, "2026-09-01", "600519")
		if c.wantNil {
			if ev != nil {
				t.Fatalf("%q 应不匹配, got %+v", c.title, ev)
			}
			continue
		}
		if ev == nil || ev.EventType != c.wantType {
			t.Fatalf("%q → expect %s, got %+v", c.title, c.wantType, ev)
		}
		if ev.ExpireDate <= ev.EventDate {
			t.Fatalf("有效期应晚于事件日: %+v", ev)
		}
	}
}
