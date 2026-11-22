package analysis

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"go-stock/backend/data/khunter"
	"go-stock/backend/db"
	"go-stock/backend/models"
)

func setupAnalysisTestDB(t *testing.T) {
	t.Helper()
	old := db.Dao
	d, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	db.Dao = d
	t.Cleanup(func() {
		if sqlDB, err := d.DB(); err == nil {
			_ = sqlDB.Close()
		}
		db.Dao = old
	})
	_ = db.Dao.AutoMigrate(&models.KhunterObservation{}, &models.KhunterKronosPrediction{}, &models.KhunterRunSummary{})
}

func TestGetRecommendationsBlendsFactor(t *testing.T) {
	setupAnalysisTestDB(t)
	_ = db.Dao.Create(&models.KhunterObservation{
		Code: "sh600519", Name: "茅台", SignalDate: "2026-11-20", Status: "观察中",
		Score: 80, DataStatus: "valid",
	}).Error
	_ = db.Dao.Create(&models.KhunterKronosPrediction{
		Code: "sh600519", SignalDate: "2026-11-20", FactorScore: 60, Weight: 0.05,
		ForecastJSON: `{"summary":{"direction":"up"}}`,
	}).Error
	_ = db.Dao.Create(&models.KhunterObservation{
		Code: "sz000001", Name: "平安", SignalDate: "2026-11-20", Status: "观察中",
		Score: 70, DataStatus: "valid",
	}).Error

	svc := NewAnalysisCenterService()
	rows, err := svc.GetRecommendations("", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("应返回 2 行: %+v", rows)
	}
	// 80*(1-0.05)+60*0.05 = 79
	if rows[0].Code != "sh600519" || rows[0].BlendedScore != 79 || !rows[0].HasFactor {
		t.Fatalf("因子混入错误: %+v", rows[0])
	}
	if rows[1].HasFactor || rows[1].BlendedScore != 70 {
		t.Fatalf("无因子应保持原分: %+v", rows[1])
	}
}

func TestGetRecommendationsExcludesDegradedFactor(t *testing.T) {
	setupAnalysisTestDB(t)
	_ = db.Dao.Create(&models.KhunterObservation{
		Code: "sh600519", Name: "茅台", SignalDate: "2026-11-20", Status: "观察中",
		Score: 80, DataStatus: "valid",
	}).Error
	_ = db.Dao.Create(&models.KhunterKronosPrediction{
		Code: "sh600519", SignalDate: "2026-11-20", FactorScore: 50, Weight: 0.05,
		DegradedReason: "KRONOS_OFFLINE",
	}).Error

	rows, err := NewAnalysisCenterService().GetRecommendations("", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].HasFactor || rows[0].BlendedScore != 80 {
		t.Fatalf("降级因子不得混入排序分: %+v", rows)
	}
	if !rows[0].Degraded || rows[0].DegradedReason == "" {
		t.Fatalf("降级原因必须透出给前端: %+v", rows[0])
	}
}

func TestGetForecastReturnsPersistedJSON(t *testing.T) {
	setupAnalysisTestDB(t)
	_ = db.Dao.Create(&models.KhunterKronosPrediction{
		Code: "sh600519", SignalDate: "2026-11-20", FactorScore: 60, Weight: 0.05,
		ForecastJSON: `{"summary":{"direction":"up","changePct":4}}`,
	}).Error
	got, err := NewAnalysisCenterService().GetForecast("sh600519", "2026-11-20")
	if err != nil {
		t.Fatal(err)
	}
	if got == "" {
		t.Fatal("应返回落库的预测明细")
	}
}

func TestGetPredictJobUnknownIDReturnsError(t *testing.T) {
	svc := NewAnalysisCenterService()
	if _, err := svc.GetPredictJob("pj_nope"); !errors.Is(err, khunter.ErrPredictJobNotFound) {
		t.Fatalf("未知任务必须返回 ErrPredictJobNotFound: %v", err)
	}
}

func TestGetRecommendationsReturnsErrorWhenDaoNil(t *testing.T) {
	saved := db.Dao
	db.Dao = nil
	defer func() { db.Dao = saved }()
	if _, err := NewAnalysisCenterService().GetRecommendations("", 10); err == nil {
		t.Fatal("db.Dao 为 nil 时必须返回错误，不得返回空列表")
	}
}
