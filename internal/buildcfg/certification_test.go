package buildcfg

import (
	"strings"
	"testing"

	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/compat/spec"
)

func certificationFixture(t *testing.T) (buildspec.Identity, spec.Profile, buildspec.CertificationReport) {
	t.Helper()
	id := fixtureIdentity()
	report := buildspec.CertificationReport{Schema: 1, Identity: id, Tuple: buildspec.TupleFor(id), ExecutionMode: "native", WorkerArch: id.HostArch}
	for _, name := range []string{"build", "execute", "native", "large"} {
		report.Suites = append(report.Suites, buildspec.SuiteResult{Name: name, Passed: true, LogDigest: buildspec.Hash([]byte(name))})
	}
	report.ResultDigest, _ = buildspec.ReportDigest(report)
	tuple := report.Tuple
	tuple.Status = "certified"
	tuple.ResultDigest = report.ResultDigest
	tuple.SourceDigest, _ = buildspec.SourceDigest(id)
	return id, spec.Profile{Tuples: []buildspec.Tuple{tuple}}, report
}

func TestCertificationRejectsFalseStaleAndIncompleteEvidence(t *testing.T) {
	t.Parallel()
	id, profile, report := certificationFixture(t)
	accepted, err := CertifiedIdentity(id, profile, []buildspec.CertificationReport{report})
	if err != nil || accepted.Certification != "certified" || accepted.EvidenceDigest != report.ResultDigest {
		t.Fatal(accepted, err)
	}
	for _, name := range []string{"pending", "missing", "source", "failed", "incomplete", "tampered", "worker", "image", "tuple", "identity", "validation-only", "large-skipped"} {
		t.Run(name, func(t *testing.T) {
			id, p, r := certificationFixture(t)
			reports := []buildspec.CertificationReport{r}
			switch name {
			case "pending":
				p.Tuples[0].Status = "pending"
				p.Tuples[0].SourceDigest = ""
				p.Tuples[0].ResultDigest = ""
			case "missing":
				reports = nil
			case "source":
				id.RuntimeRevision = strings.Repeat("b", 64)
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
			case "validation-only":
				r.ValidationOnly = true
				r.ResultDigest, _ = buildspec.ReportDigest(r)
				p.Tuples[0].ResultDigest = r.ResultDigest
			case "large-skipped":
				r.Suites = r.Suites[:3]
				r.ResultDigest, _ = buildspec.ReportDigest(r)
				p.Tuples[0].ResultDigest = r.ResultDigest
			case "identity":
				r.Identity.TranslatorDigest = strings.Repeat("b", 64)
			}
			if reports != nil {
				reports[0] = r
			}
			if _, err := CertifiedIdentity(id, p, reports); err == nil {
				t.Fatal("accepted invalid certification")
			}
		})
	}
}

func TestApplicationGraphSeparatesArtifactsNotFrameworkCertification(t *testing.T) {
	t.Parallel()
	id, profile, report := certificationFixture(t)
	before, _ := buildspec.Digest(id)
	id.ModuleGraphDigest = strings.Repeat("b", 64)
	after, _ := buildspec.Digest(id)
	if before == after {
		t.Fatal("application graph did not invalidate artifact")
	}
	if _, err := CertifiedIdentity(id, profile, []buildspec.CertificationReport{report}); err != nil {
		t.Fatal("unchanged framework required application certification", err)
	}
}

func TestReducedReportRequiresApplicableNativeAndBoundarySuites(t *testing.T) {
	t.Parallel()
	_, _, report := certificationFixture(t)
	report.Identity.Features = []string{"rt"}
	report.Identity.ProviderRevisions = nil
	report.Tuple = buildspec.TupleFor(report.Identity)
	report.ResultDigest, _ = buildspec.ReportDigest(report)
	if err := buildspec.ValidateReport(report); err == nil {
		t.Fatal("reduced tuple accepted without boundary checks")
	}
	report.ValidationOnly = true
	report.ResultDigest, _ = buildspec.ReportDigest(report)
	if err := buildspec.ValidateReport(report); err == nil {
		t.Fatal("reduced tuple omitted runtime/model unit suites")
	}
	report.Suites = append(report.Suites, buildspec.SuiteResult{Name: "native-units", Passed: true, LogDigest: buildspec.Hash([]byte("native-units"))})
	report.ResultDigest, _ = buildspec.ReportDigest(report)
	if err := buildspec.ValidateReport(report); err != nil {
		t.Fatal(err)
	}
	report.Suites = report.Suites[:2]
	report.ResultDigest, _ = buildspec.ReportDigest(report)
	if err := buildspec.ValidateReport(report); err == nil {
		t.Fatal("reduced tuple omitted native metatests")
	}
}
