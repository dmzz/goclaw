package skills

import (
	"path/filepath"
	"regexp"
	"strings"
)

// SlugRegexp validates skill slugs: lowercase alphanumeric with hyphens, no leading/trailing hyphen.
var SlugRegexp = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

// LOCAL FIX START: repair malformed frontmatter opener ---\name:
// escapedFrontmatterOpenerRegexp matches malformed frontmatter openers like
// ---\name: ... where the model inserted a stray backslash instead of a newline.
// Match only the known frontmatter keys so valid escaped newlines like ---\nname:
// still flow through the generic unescape path below.
var escapedFrontmatterOpenerRegexp = regexp.MustCompile(`(?m)^---\\(name|description|slug)(\s*:)`)

// LOCAL FIX END: repair malformed frontmatter opener ---\name:

// ParseSkillFrontmatter extracts name, description, and slug from SKILL.md YAML frontmatter.
// Also returns the full parsed frontmatter as a map for DB storage.
func ParseSkillFrontmatter(content string) (name, description, slug string, allFields map[string]string) {
	allFields = make(map[string]string)
	// LOCAL FIX START: normalize incoming SKILL.md content before parsing
	normalized := NormalizeSkillContent(content)
	// LOCAL FIX END: normalize incoming SKILL.md content before parsing
	// LOCAL FIX START: parse frontmatter from normalized content
	if !strings.HasPrefix(strings.TrimSpace(normalized), "---") {
		return "", "", "", allFields
	}
	fm := extractFrontmatter(normalized)
	if fm == "" {
		return "", "", "", allFields
	}
	// LOCAL FIX END: parse frontmatter from normalized content
	allFields = parseSimpleYAML(fm)
	name = allFields["name"]
	description = allFields["description"]
	slug = allFields["slug"]
	return
}

// LOCAL FIX START: normalize escaped or malformed SKILL.md frontmatter
// NormalizeSkillContent canonicalizes SKILL.md content before validation/persistence.
// It repairs common model/tool escaping mistakes where frontmatter is sent with
// escaped newlines or a malformed opener such as ---\name: ...
func NormalizeSkillContent(content string) string {
	content = normalizeLineEndings(content)
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "---") && extractFrontmatter(content) == "" {
		// First repair the malformed opener captured in traces: ---\name: ...
		content = escapedFrontmatterOpenerRegexp.ReplaceAllString(content, "---\n$1$2")
		// First repair the common escaped-newline case: ---\nname: ...
		if strings.Contains(content, `\n`) || strings.Contains(content, `\r\n`) || strings.Contains(content, `\t`) {
			content = strings.ReplaceAll(content, `\r\n`, "\n")
			content = strings.ReplaceAll(content, `\n`, "\n")
			content = strings.ReplaceAll(content, `\t`, "\t")
		}
	}
	return normalizeLineEndings(content)
}

// LOCAL FIX END: normalize escaped or malformed SKILL.md frontmatter

// Slugify converts a skill name into a valid slug (lowercase, alphanumeric + hyphens).
func Slugify(name string) string {
	s := strings.ToLower(name)
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, s)
	// Collapse multiple dashes
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")
	if s == "" {
		s = "skill"
	}
	return s
}

// IsSystemArtifact returns true for OS-generated junk that should be skipped
// during file extraction and listing (e.g. __MACOSX, .DS_Store, Thumbs.db).
func IsSystemArtifact(name string) bool {
	base := filepath.Base(name)
	// macOS resource fork / metadata folders and files
	if base == "__MACOSX" || strings.HasPrefix(base, "._") {
		return true
	}
	// Check if any path component is __MACOSX
	for _, part := range strings.Split(filepath.ToSlash(name), "/") {
		if part == "__MACOSX" {
			return true
		}
	}
	// Common OS junk files
	switch base {
	case ".DS_Store", "Thumbs.db", "desktop.ini", ".Spotlight-V100", ".Trashes", ".fseventsd":
		return true
	}
	return false
}
