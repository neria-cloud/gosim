package audit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func RenderReport(report ComparisonReport) ([]byte, error) {
	var out bytes.Buffer
	fmt.Fprintf(&out, "# Go compatibility comparison\n\nCandidate: %s; parent: %s\n\nComplete: %t; API diff: %s; findings: %d\n\n", report.Candidate.Version, report.ParentFamily, report.Complete, report.APIDiff, len(report.Findings))
	out.WriteString("Source inspection does not establish semantic compatibility or approve support. Race contexts describe source selection only.\n\n")
	for _, c := range report.Contexts {
		fmt.Fprintf(&out, "- Context %s/%s, native=%t, race-source=%t: complete=%t\n", c.Context.OS, c.Context.Arch, c.Context.Native, c.Context.Race, c.Complete)
		for _, e := range c.Errors {
			fmt.Fprintf(&out, "  - %s\n", e)
		}
	}
	for _, f := range report.Findings {
		fmt.Fprintf(&out, "\n## %s: %s %s\n\n%s\n\nContext: %s\n", f.Severity, f.Kind, recordKey(f.Key), f.Message, contextKey(f.Context))
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
