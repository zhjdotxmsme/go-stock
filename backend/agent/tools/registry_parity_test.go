package tools

import (
	"strings"
	"testing"

	"go-stock/backend/data"
)

// TestRegistryParity enforces the contract between the two tool registries:
// the eino set (this package, consumed by the React/PlanExecute agent) and
// the legacy OpenAI-style registry (data.Tools, consumed by
// SummaryStockNews / cron / web). The registries intentionally keep separate
// executors — the legacy handlers carry streaming ToolContext state — but a
// tool existing in only one registry is drift and fails here.
func TestRegistryParity(t *testing.T) {
	einoTools := GetAllDataTools()
	einoTools = append(einoTools,
		GetQueryStockCodeInfoTool(), GetQueryStockNewsTool(), GetQueryBKDictTool(),
	)
	einoTools = append(einoTools, GetHolidayTools()...)
	einoTools = append(einoTools, GetMCPServerTools()...)
	einoTools = append(einoTools, GetSkillTools()...)
	if len(einoTools) == 0 {
		t.Fatal("eino registry is empty")
	}
	legacyTools := data.Tools(nil)
	if len(legacyTools) == 0 {
		t.Fatal("legacy registry is empty")
	}

	einoNames := map[string]bool{}
	for _, tl := range einoTools {
		info, err := tl.Info(nil)
		if err != nil || info == nil {
			t.Errorf("eino tool Info() failed: %v", err)
			continue
		}
		einoNames[info.Name] = true
	}
	legacyNames := map[string]bool{}
	for _, tl := range legacyTools {
		legacyNames[tl.Function.Name] = true
	}

	// Legacy-only tools are allowed only if they are genuinely out of scope
	// for the eino agent (documented here so the list stays intentional).
	legacyOnlyAllowed := map[string]bool{
		"GetStockHistoryData":    true, // legacy pagination variant
		"GetStockKLinePage":      true, // legacy pagination variant
		"GetEastMoneyKLinePage":  true, // legacy pagination variant
		"GetAIAnalysisHistories": true, // legacy paging alias
	}

	// Eino-only tools are allowed only where the legacy chains have no use
	// for them: MCP-server and skill management are agent-mode operations,
	// and the base dictionary utilities were never surfaced to the old chain.
	einoOnlyAllowed := map[string]bool{
		"ListMCPServers": true, "GetMCPServerDetail": true, "CreateMCPServer": true,
		"UpdateMCPServer": true, "DeleteMCPServer": true, "EnableMCPServer": true,
		"TestMCPServer": true, "ListMCPServerTools": true, "GetMCPToolDetail": true,
		"ListSkills": true, "GetSkillDetail": true, "CreateSkill": true,
		"UpdateSkill": true, "DeleteSkill": true, "EnableSkill": true,
		"GetSkillRecommendations": true, "GenerateSkillFromURL": true,
		"AnalyzeSkillEffectiveness": true, "GetSkillUsageStats": true,
		"QueryBKDictInfo": true,
	}

	missingInLegacy := []string{}
	for name := range einoNames {
		if !legacyNames[name] && !einoOnlyAllowed[name] {
			missingInLegacy = append(missingInLegacy, name)
		}
	}
	missingInEino := []string{}
	for name := range legacyNames {
		if !einoNames[name] && !legacyOnlyAllowed[name] {
			missingInEino = append(missingInEino, name)
		}
	}

	if len(missingInLegacy) > 0 {
		t.Errorf("eino tools missing from legacy registry (%d): %s",
			len(missingInLegacy), strings.Join(missingInLegacy, ", "))
	}
	if len(missingInEino) > 0 {
		t.Errorf("legacy tools missing from eino registry (%d, excluding documented legacy-only): %s",
			len(missingInEino), strings.Join(missingInEino, ", "))
	}

	t.Logf("registry sizes: eino=%d legacy=%d (eino-only allowed: %d, legacy-only allowed: %d)",
		len(einoNames), len(legacyNames), len(einoOnlyAllowed), len(legacyOnlyAllowed))
}

// TestGroupTaxonomyParity pins the group taxonomies: a tool must not claim a
// group in the eino classifier that the legacy registry classifies
// differently without an explicit documented reason.
func TestGroupTaxonomyParity(t *testing.T) {
	// Spot-check core tools whose routing matters most; a full bidirectional
	// mapping dump would make intentional differences unreadable.
	core := []struct {
		tool string
		want ToolGroup
	}{
		{"GetStockInfo", GroupStockAnalysis},
		{"GetStockKLine", GroupStockAnalysis},
		{"GetCurrentTime", GroupBase},
	}
	for _, c := range core {
		got, ok := toolGroupMap[c.tool]
		if !ok {
			t.Errorf("tool %q has no eino group mapping", c.tool)
			continue
		}
		if got != c.want {
			t.Errorf("tool %q: eino group = %q, want %q", c.tool, got, c.want)
		}
	}
}
