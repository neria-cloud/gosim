package buildcfg

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

func certificationFixture(t *testing.T) (buildspec.Identity, buildspec.CertificationReport) {
	t.Helper()
	id := fixtureIdentity()
	source, err := buildspec.SourceDigest(id)
	if err != nil {
		t.Fatal(err)
	}
	report := buildspec.CertificationReport{Schema: 2, Identity: id, FrameworkDigest: source, Tuple: buildspec.TupleFor(id), ExecutionMode: "native", WorkerArch: id.HostArch}
	for _, name := range []string{"build", "execute", "native", "large"} {
		report.Suites = append(report.Suites, buildspec.SuiteResult{Name: name, Passed: true, LogDigest: buildspec.Hash([]byte(name))})
	}
	report.ResultDigest, err = buildspec.ReportDigest(report)
	if err != nil {
		t.Fatal(err)
	}
	return id, report
}

// These are in-memory report validation checks, not certification executions.
func TestManualReportRejectsFalseStaleAndIncompleteEvidence(t *testing.T) {
	t.Parallel()
	_, report := certificationFixture(t)
	if err := buildspec.ValidateReport(report); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"source", "failed", "incomplete", "tampered", "worker", "image", "tuple", "large-skipped", "identity", "old-schema"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, r := certificationFixture(t)
			switch name {
			case "source":
				r.FrameworkDigest = strings.Repeat("b", 64)
			case "failed":
				r.Suites[0].Passed = false
			case "incomplete":
				r.Suites = nil
			case "tampered":
				r.Suites[0].LogDigest = strings.Repeat("b", 64)
			case "worker":
				r.WorkerArch = "other"
			case "image":
				r.ContainerImageDigest = "sha256:" + strings.Repeat("a", 64)
			case "tuple":
				r.Tuple.Race = !r.Tuple.Race
			case "large-skipped":
				r.Suites = r.Suites[:3]
				r.ResultDigest, _ = buildspec.ReportDigest(r)
			case "identity":
				r.Identity.TranslatorDigest = strings.Repeat("b", 64)
			case "old-schema":
				r.Schema = 1
			}
			if err := buildspec.ValidateReport(r); err == nil {
				t.Fatal("accepted invalid report")
			}
		})
	}
}

func TestEvidenceIsOutsideExecutableIdentity(t *testing.T) {
	t.Parallel()
	id, r := certificationFixture(t)
	before, _ := buildspec.Digest(id)
	// A report can be missing, stale or invalid without changing the build value.
	r.FrameworkDigest = strings.Repeat("c", 64)
	r.ResultDigest = ""
	if err := buildspec.ValidateReport(r); err == nil {
		t.Fatal("accepted stale report")
	}
	after, _ := buildspec.Digest(id)
	if before != after {
		t.Fatal("external evidence changed artifact identity")
	}
	data, err := json.Marshal(id)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "certification") || strings.Contains(string(data), "evidence_digest") {
		t.Fatal("evidence remains in executable identity", string(data))
	}
	source, _ := buildspec.SourceDigest(id)
	id.ModuleGraphDigest = strings.Repeat("b", 64)
	changed, _ := buildspec.Digest(id)
	sameSource, _ := buildspec.SourceDigest(id)
	if changed == before || source != sameSource {
		t.Fatal("application and framework identity not separated")
	}
}

func TestReducedReportRequiresApplicableNativeAndBoundarySuites(t *testing.T) {
	t.Parallel()
	_, r := certificationFixture(t)
	r.Identity.Features = []string{"rt"}
	r.Identity.ProviderRevisions = nil
	r.Tuple = buildspec.TupleFor(r.Identity)
	r.FrameworkDigest, _ = buildspec.SourceDigest(r.Identity)
	r.ResultDigest, _ = buildspec.ReportDigest(r)
	if err := buildspec.ValidateReport(r); err == nil {
		t.Fatal("reduced report omitted native units")
	}
	r.Suites = append(r.Suites, buildspec.SuiteResult{Name: "native-units", Passed: true, LogDigest: buildspec.Hash([]byte("units"))})
	r.ResultDigest, _ = buildspec.ReportDigest(r)
	if err := buildspec.ValidateReport(r); err != nil {
		t.Fatal(err)
	}
}
