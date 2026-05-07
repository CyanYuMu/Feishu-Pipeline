package pipeline

import "testing"

func TestTemplateDefinitionForPredefinedTemplates(t *testing.T) {
	templateIDs := []string{DefaultTemplateID, BugFixTemplateID, RefactorTemplateID}
	for _, templateID := range templateIDs {
		definition := TemplateDefinitionFor(templateID)
		if definition.ID != templateID {
			t.Fatalf("expected template id %s, got %s", templateID, definition.ID)
		}
		if definition.Kind == "" {
			t.Fatalf("expected template %s to declare kind", templateID)
		}
		if len(definition.UseCases) == 0 {
			t.Fatalf("expected template %s to declare use cases", templateID)
		}
		if len(definition.Stages) != len(DefaultStageDefinitions) {
			t.Fatalf("expected template %s to have %d stages, got %d", templateID, len(DefaultStageDefinitions), len(definition.Stages))
		}
	}
}

func TestTemplateDefinitionJSONParses(t *testing.T) {
	definition, err := ParseTemplateDefinition(TemplateDefinitionJSON(BugFixTemplateID))
	if err != nil {
		t.Fatalf("ParseTemplateDefinition returned error: %v", err)
	}
	if definition.ID != BugFixTemplateID {
		t.Fatalf("expected bug fix template id, got %s", definition.ID)
	}
	if len(definition.Stages) == 0 {
		t.Fatal("expected parsed template stages")
	}
}
