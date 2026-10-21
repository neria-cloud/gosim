package audit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func RenderReport(report ComparisonReport) ([]byte, error) {
	if _, err := json.Marshal(report); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	fmt.Fprintf(&out, "# Go compatibility comparison\n\nCandidate: %s; parent: %s\n\nComplete: %t; API diff: %s; findings: %d\n\n", report.Candidate.Version, report.ParentFamily, report.Complete, report.APIDiff, len(report.Findings))
	out.WriteString("Source inspection does not establish semantic compatibility or approve support. Race contexts describe source selection only.\n\n")
	for _, c := range report.Contexts {
		fmt.Fprintf(&out, "- Context %s/%s, native=%t, race-source=%t: complete=%t\n", c.Context.OS, c.Context.Arch, c.Context.Native, c.Context.Race, c.Complete)
		for _, e := range c.Errors {
			fmt.Fprintf(&out, "  - %s\n", e)
		}
	}
	groups := groupFindings(report.Findings)
	out.WriteString("\n| Severity | Distinct findings |\n| --- | ---: |\n")
	for _, severity := range []string{"incomplete", "incompatible", "review", "information"} {
		count := 0
		for _, group := range groups {
			if group.Finding.Severity == severity {
				count++
			}
		}
		fmt.Fprintf(&out, "| %s | %d |\n", severity, count)
	}
	for _, group := range groups {
		f := group.Finding
		fmt.Fprintf(&out, "\n## %s: %s %s\n\n%s\n\nContexts: `%s`\n", f.Severity, f.Kind, recordKey(f.Key), f.Message, auditJSON(group.Contexts))
		if len(f.Before) > 0 {
			fmt.Fprintf(&out, "\nBefore: `%s`\n", f.Before)
		}
		if len(f.After) > 0 {
			fmt.Fprintf(&out, "\nAfter: `%s`\n", f.After)
		}
		for _, l := range f.Locations {
			fmt.Fprintf(&out, "\nSource: %s:%d\n", l.Path, l.Line)
		}
	}
	return out.Bytes(), nil
}

// Each output path is owned only while it still refers to the file we created.
// Cleanup must preserve files subsequently placed there by somebody else.
func removeOwned(path string, info os.FileInfo) error {
	current, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !os.SameFile(current, info) {
		return nil
	}
	return os.Remove(path)
}

func writeExclusive(path string, data []byte) (info os.FileInfo, resultErr error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, err
	}
	info, err = f.Stat()
	if err == nil {
		_, err = f.Write(data)
	}
	return info, errors.Join(err, f.Close())
}

func artifactJSON(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	return append(data, '\n'), err
}

// Link publishes a closed temporary file atomically without replacing a racing
// destination. A plain rename could overwrite a path this invocation never owned.
func publishExclusive(path string, data []byte) (resultErr error) {
	f, err := os.CreateTemp(filepath.Dir(path), ".gosim-audit-*")
	if err != nil {
		return err
	}
	temp := f.Name()
	info, statErr := f.Stat()
	published := false
	defer func() {
		resultErr = errors.Join(resultErr, os.Remove(temp))
		if resultErr != nil && published {
			resultErr = errors.Join(resultErr, removeOwned(path, info))
		}
	}()
	if statErr != nil {
		return errors.Join(statErr, f.Close())
	}
	_, err = f.Write(data)
	err = errors.Join(err, f.Close())
	if err != nil {
		return err
	}
	err = os.Link(temp, path)
	published = err == nil
	return err
}

func WriteComparison(directory string, report ComparisonReport, draft DraftDelta) (resultErr error) {
	if report.Schema != 1 || draft.Schema != 1 || draft.Kind != "gosim-profile-draft" || report.Complete != draft.Complete {
		return fmt.Errorf("inconsistent comparison artifacts")
	}
	markdown, err := RenderReport(report)
	if err != nil {
		return err
	}
	draftJSON, err := artifactJSON(draft)
	if err != nil {
		return err
	}
	reportJSON, err := artifactJSON(report)
	if err != nil {
		return err
	}
	if err := os.Mkdir(directory, 0o700); err != nil {
		return err
	}
	dirInfo, err := os.Lstat(directory)
	if err != nil {
		return err
	}
	owned := map[string]os.FileInfo{}
	defer func() {
		if resultErr == nil {
			return
		}
		for path, info := range owned {
			if info != nil {
				resultErr = errors.Join(resultErr, removeOwned(path, info))
			}
		}
		resultErr = errors.Join(resultErr, removeOwned(directory, dirInfo))
	}()
	for _, file := range []struct {
		name string
		data []byte
	}{{"report.md", markdown}, {"delta.draft.json", draftJSON}} {
		path := filepath.Join(directory, file.name)
		info, err := writeExclusive(path, file.data)
		if info != nil {
			owned[path] = info
		}
		if err != nil {
			return err
		}
	}
	// The manifest is last; a missing manifest means publication was interrupted.
	return publishExclusive(filepath.Join(directory, "report.json"), reportJSON)
}

func WriteObservation(path string, snapshot ParentObservation) error {
	if snapshot.Schema != 1 || !snapshot.Complete || len(snapshot.Observations.Contexts) == 0 {
		return fmt.Errorf("refusing incomplete parent observation")
	}
	for _, c := range snapshot.Observations.Contexts {
		if !c.Complete || len(c.Errors) > 0 {
			return fmt.Errorf("refusing incomplete parent context")
		}
	}
	data, err := artifactJSON(snapshot)
	if err != nil {
		return err
	}
	return publishExclusive(path, data)
}

type findingGroup struct {
	Finding  Finding
	Contexts []InspectedContext
}

func groupFindings(findings []Finding) []findingGroup {
	groups := map[string]findingGroup{}
	for _, f := range findings {
		key := string(auditJSON([]any{f.Kind, f.Key, f.Before, f.After, f.Severity, f.Message}))
		group, ok := groups[key]
		if !ok {
			group.Finding = f
			group.Finding.Locations = slices.Clone(f.Locations)
		}
		if !slices.Contains(group.Contexts, f.Context) {
			group.Contexts = append(group.Contexts, f.Context)
		}
		for _, location := range f.Locations {
			if !slices.Contains(group.Finding.Locations, location) {
				group.Finding.Locations = append(group.Finding.Locations, location)
			}
		}
		groups[key] = group
	}
	out := make([]findingGroup, 0, len(groups))
	for _, group := range groups {
		slices.SortFunc(group.Contexts, func(a, b InspectedContext) int { return strings.Compare(contextKey(a), contextKey(b)) })
		out = append(out, group)
	}
	priority := map[string]int{"incomplete": 0, "incompatible": 1, "review": 2, "information": 3}
	slices.SortFunc(out, func(a, b findingGroup) int {
		if difference := priority[a.Finding.Severity] - priority[b.Finding.Severity]; difference != 0 {
			return difference
		}
		return strings.Compare(string(auditJSON(a.Finding)), string(auditJSON(b.Finding)))
	})
	return out
}

// PrepareOutput checks destinations before expensive compiler discovery. Only
// parent directories are created; final publication still claims paths exclusively.
func PrepareOutput(path string) (resultErr error) {
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		if err != nil {
			return err
		}
		return fmt.Errorf("output already exists: %s", path)
	}
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(parent, ".gosim-audit-probe-*")
	if err != nil {
		return err
	}
	name := f.Name()
	link := name + ".link"
	defer func() { resultErr = errors.Join(resultErr, os.Remove(name)) }()
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Link(name, link); err != nil {
		return fmt.Errorf("audit destination requires exclusive hard-link publication: %w", err)
	}
	return os.Remove(link)
}
