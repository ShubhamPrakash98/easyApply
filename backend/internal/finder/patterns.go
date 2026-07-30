package finder

import "strings"

// DefaultPatternGenerator produces candidate emails from a full name + domain.
// Order matters: the verifier stops at the first deliverable, so we put the
// most common patterns first.
type DefaultPatternGenerator struct{}

func NewDefaultPatternGenerator() *DefaultPatternGenerator {
	return &DefaultPatternGenerator{}
}

func (DefaultPatternGenerator) Generate(fullName, domain string) []string {
	first, last := splitFirstLast(fullName)
	if first == "" || domain == "" {
		return nil
	}

	// If we only have a first name, generate a small set.
	if last == "" {
		return dedupe([]string{
			first + "@" + domain,
		})
	}

	fi := first[:1]
	li := last[:1]

	return dedupe([]string{
		first + "." + last + "@" + domain, // jane.smith
		first + last + "@" + domain,       // janesmith
		fi + last + "@" + domain,          // jsmith
		first + "@" + domain,              // jane
		first + li + "@" + domain,         // janes
		last + "." + first + "@" + domain, // smith.jane
		first + "_" + last + "@" + domain, // jane_smith
		last + first + "@" + domain,       // smithjane
	})
}

// splitFirstLast extracts the first and last token of a name. Middle tokens
// are ignored (so "Jane Q. Smith" → "jane", "smith").
func splitFirstLast(full string) (string, string) {
	full = strings.TrimSpace(full)
	if full == "" {
		return "", ""
	}
	// Strip punctuation like "Jane Q." → "Jane Q"
	tokens := strings.FieldsFunc(full, func(r rune) bool {
		switch r {
		case ' ', '\t', '\n', ',', '.':
			return true
		}
		return false
	})
	if len(tokens) == 0 {
		return "", ""
	}
	first := strings.ToLower(tokens[0])
	if len(tokens) == 1 {
		return first, ""
	}
	last := strings.ToLower(tokens[len(tokens)-1])
	// Guard: if last name is a single character (initial), fall back to
	// treating it as no last name.
	if len(last) < 2 {
		return first, ""
	}
	return first, last
}

func dedupe(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
