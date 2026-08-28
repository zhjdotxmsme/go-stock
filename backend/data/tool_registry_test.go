package data

import (
	"strings"
	"testing"
)

// TestLegacyRegistryInternalParity is the A1 step 2 gate for the legacy
// OpenAI-style chain: after Tools() assembly, every schema must have a
// registered handler and every handler a schema. Drift previously failed
// only at LLM-call time ("unknown tool"); Tools() now logs it at assembly,
// and this test fails the build on it.
func TestLegacyRegistryInternalParity(t *testing.T) {
	tools := Tools(nil)
	if len(tools) == 0 {
		t.Fatal("legacy registry assembled empty")
	}

	// 双向一致性以 Tools() 内部（API-Key 过滤前）的门禁结果为准：
	// 过滤后视图里 Key 未配置的工具只剩 handler 没有 schema，属预期。
	if len(lastSchemaHandlerDrift) > 0 {
		t.Errorf("schemas without handlers (%d): %s", len(lastSchemaHandlerDrift), strings.Join(lastSchemaHandlerDrift, ", "))
	}
	if len(lastHandlerSchemaDrift) > 0 {
		t.Errorf("handlers without schemas (%d): %s", len(lastHandlerSchemaDrift), strings.Join(lastHandlerSchemaDrift, ", "))
	}

	schemaNames := map[string]bool{}
	for _, tl := range tools {
		name := tl.Function.Name
		if strings.TrimSpace(name) == "" {
			t.Error("assembled schema with empty function name")
			continue
		}
		schemaNames[name] = true
	}

	// Definition-centric registrations must surface through Tools() too.
	if _, ok := schemaNames["GetCurrentTime"]; !ok {
		t.Error("definition-registered tool GetCurrentTime missing from assembled list")
	}

	t.Logf("legacy registry: %d schemas, %d handlers, parity ok", len(schemaNames), len(toolHandlers))
}

// TestToolDefinitionRegistration pins the definition-centric registration
// mechanics: order preserved, handler wired, empty-name rejected.
func TestToolDefinitionRegistration(t *testing.T) {
	if len(toolDefinitionOrder) == 0 {
		t.Fatal("no definitions registered (GetCurrentTime init expected)")
	}
	first := toolDefinitionOrder[0]
	def, ok := toolDefinitions[first]
	if !ok {
		t.Fatalf("order entry %q missing from definition map", first)
	}
	if def.Handler == nil {
		t.Errorf("definition %q has nil handler", first)
	}
	if def.Schema.Function.Name != first {
		t.Errorf("definition order %q mismatches schema name %q", first, def.Schema.Function.Name)
	}
	if !toolDefinitionsBound {
		t.Error("Tools() was not called before this test; bind gate did not run")
	}
}

// TestBindDefinitionsToSchemas_DriftDetection pins the gate's drift reporting.
func TestBindDefinitionsToSchemas_DriftDetection(t *testing.T) {
	fake := []Tool{{Type: "function", Function: ToolFunction{Name: "DefinitelyNotARealTool"}}}
	drift := bindDefinitionsToSchemas(fake)
	found := false
	for _, name := range drift {
		if name == "DefinitelyNotARealTool" {
			found = true
		}
	}
	if !found {
		t.Fatal("gate failed to report a schema without handler")
	}
}
