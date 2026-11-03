package multi

import "testing"

func TestReportOrHold(t *testing.T) {
	kept := &FinalReport{OverallRating: "buy", Conclusion: "ok"}
	if got := reportOrHold(kept); got != kept {
		t.Fatal("已有报告不应被替换")
	}
	got := reportOrHold(nil)
	if got == nil || got.OverallRating != "hold" || got.Conclusion == "" {
		t.Fatalf("空报告应降级为 hold，实际 %#v", got)
	}
}
