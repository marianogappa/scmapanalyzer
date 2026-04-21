package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/marianogappa/scmapanalyzer/replaymap"
)

type digestEntry struct {
	Key    string
	Result *replaymap.Result
	Dupes  []string
}

// digestFromOutputJSON scans mapOutDir for *.json map analysis files and
// rebuilds digest entries (used so -only-run-map still updates the full digest).
func digestFromOutputJSON(mapOutDir string) ([]digestEntry, error) {
	matches, err := filepath.Glob(filepath.Join(mapOutDir, "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	out := make([]digestEntry, 0, len(matches))
	for _, path := range matches {
		base := filepath.Base(path)
		key := strings.TrimSuffix(base, ".json")
		if key == "" {
			continue
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var res replaymap.Result
		if err := json.Unmarshal(b, &res); err != nil {
			return nil, err
		}
		rp := new(replaymap.Result)
		*rp = res
		dupes := replaymap.DuplicateNamesInResult(rp)
		out = append(out, digestEntry{Key: key, Result: rp, Dupes: dupes})
	}
	return out, nil
}

func writeBasesDigest(path string, entries []digestEntry) error {
	if len(entries) == 0 {
		return nil
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Key < entries[j].Key })

	var b strings.Builder
	b.WriteString("# Replay map base digest\n\n")
	b.WriteString("Per-map base polygon names from replaymap analysis JSON. ")
	b.WriteString("The **Natural expansion** column is the `natural_expansion` field on each start (name of the linked expansion polygon).\n\n")

	anyDup := false
	for _, e := range entries {
		if len(e.Dupes) > 0 {
			anyDup = true
			break
		}
	}
	if anyDup {
		b.WriteString("**Note:** one or more maps still had duplicate base names; see per-map **Duplicate names** lines.\n\n")
	} else {
		b.WriteString("**Duplicate names:** none detected across analyzed maps.\n\n")
	}

	for _, e := range entries {
		writeMapDigestSection(&b, e)
	}

	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func writeMapDigestSection(b *strings.Builder, e digestEntry) {
	r := e.Result
	title := sanitizeMapTitle(r.MapName)
	if title == "" {
		title = e.Key
	}
	b.WriteString("## ")
	b.WriteString(title)
	b.WriteString("\n\n")
	b.WriteString("- **JSON key:** `")
	b.WriteString(e.Key)
	b.WriteString("`\n")
	if len(e.Dupes) == 0 {
		b.WriteString("- **Duplicate base names:** none\n\n")
	} else {
		b.WriteString("- **Duplicate base names:** ")
		for i, d := range e.Dupes {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString("`")
			b.WriteString(d)
			b.WriteString("`")
		}
		b.WriteString("\n\n")
	}

	b.WriteString("### Starts\n\n")
	b.WriteString("| Name | Natural expansion |\n")
	b.WriteString("|------|--------------------|\n")
	for _, s := range r.Starts {
		ne := s.NaturalExpansion
		if ne == "" {
			ne = "—"
		}
		b.WriteString("| ")
		b.WriteString(escapeMDCell(s.Name))
		b.WriteString(" | ")
		b.WriteString(escapeMDCell(ne))
		b.WriteString(" |\n")
	}
	b.WriteString("\n")

	refs := map[string][]string{}
	for _, s := range r.Starts {
		if s.NaturalExpansion == "" {
			continue
		}
		refs[s.NaturalExpansion] = append(refs[s.NaturalExpansion], s.Name)
	}
	for k := range refs {
		sort.Strings(refs[k])
	}

	b.WriteString("### Expansions and naturals\n\n")
	b.WriteString("| Name | Kind | Linked from starts (natural of) |\n")
	b.WriteString("|------|------|-----------------------------------|\n")
	for _, x := range r.Expas {
		link := "—"
		if xs, ok := refs[x.Name]; ok && len(xs) > 0 {
			link = strings.Join(xs, ", ")
		}
		b.WriteString("| ")
		b.WriteString(escapeMDCell(x.Name))
		b.WriteString(" | ")
		b.WriteString(escapeMDCell(x.Kind))
		b.WriteString(" | ")
		b.WriteString(escapeMDCell(link))
		b.WriteString(" |\n")
	}
	b.WriteString("\n")
}

func escapeMDCell(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	return s
}

// sanitizeMapTitle removes StarCraft replay string control codes so markdown
// headings stay readable.
func sanitizeMapTitle(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '\n' || r == '\r' {
			continue
		}
		if unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}
