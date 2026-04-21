package skills

import (
	"strings"
	"testing"
)

// LOCAL FIX START: escaped and malformed frontmatter regression coverage
func TestNormalizeSkillContent_UnescapesFrontmatterNewlines(t *testing.T) {
	raw := `---
\nname: Todoist Weekly Overview RU\ndescription: Weekly tasks\n---\n# Todoist Weekly Overview RU\n`

	got := NormalizeSkillContent(raw)

	if got == raw {
		t.Fatal("expected escaped newlines to be normalized")
	}
	if got[:4] != "---\n" {
		t.Fatalf("expected real newline after frontmatter opener, got %q", got[:8])
	}
}

func TestParseSkillFrontmatter_WithEscapedNewlines(t *testing.T) {
	raw := `---\nname: Todoist Weekly Overview RU\ndescription: Weekly tasks\nslug: todoist-weekly-overview-ru\n---\n# Todoist Weekly Overview RU`

	name, desc, slug, fields := ParseSkillFrontmatter(raw)

	if name != "Todoist Weekly Overview RU" {
		t.Fatalf("name = %q", name)
	}
	if desc != "Weekly tasks" {
		t.Fatalf("description = %q", desc)
	}
	if slug != "todoist-weekly-overview-ru" {
		t.Fatalf("slug = %q", slug)
	}
	if fields["name"] != name {
		t.Fatalf("frontmatter name = %q", fields["name"])
	}
}

func TestNormalizeSkillContent_RepairsMalformedFrontmatterOpener(t *testing.T) {
	raw := "---\\name: Todoist Weekly Overview RU\ndescription: Weekly tasks\n---\n# Todoist Weekly Overview RU\n"

	got := NormalizeSkillContent(raw)

	if got[:4] != "---\n" {
		t.Fatalf("expected real newline after malformed opener, got %q", got[:8])
	}
	if fields := strings.SplitN(got, "\n", 3); len(fields) < 2 || fields[1] != "name: Todoist Weekly Overview RU" {
		t.Fatalf("unexpected normalized opener: %q", got)
	}
}

func TestParseSkillFrontmatter_WithMalformedFrontmatterOpener(t *testing.T) {
	raw := "---\\name: Todoist Weekly Overview RU\ndescription: Weekly tasks\nslug: todoist-weekly-overview-ru\n---\n# Todoist Weekly Overview RU"

	name, desc, slug, fields := ParseSkillFrontmatter(raw)

	if name != "Todoist Weekly Overview RU" {
		t.Fatalf("name = %q", name)
	}
	if desc != "Weekly tasks" {
		t.Fatalf("description = %q", desc)
	}
	if slug != "todoist-weekly-overview-ru" {
		t.Fatalf("slug = %q", slug)
	}
	if fields["name"] != name {
		t.Fatalf("frontmatter name = %q", fields["name"])
	}
}

// LOCAL FIX END: escaped and malformed frontmatter regression coverage
