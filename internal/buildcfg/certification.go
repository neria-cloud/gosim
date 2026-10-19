package buildcfg

import (
	"fmt"

	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/compat/spec"
)

// CertifiedIdentity accepts only reviewed tuple metadata and a complete report
// matching this executable framework revision. It never updates metadata.
func CertifiedIdentity(id buildspec.Identity, profile spec.Profile, reports []buildspec.CertificationReport) (buildspec.Identity, error) {
	source, err := buildspec.SourceDigest(id)
	if err != nil {
		return id, err
	}
	for _, tuple := range profile.Tuples {
		if !buildspec.SameTuple(tuple, buildspec.TupleFor(id)) {
			continue
		}
		if tuple.Status != "certified" {
			return id, fmt.Errorf("compatibility tuple is pending; normal commands require certified evidence")
		}
		if tuple.SourceDigest != source {
			return id, fmt.Errorf("certification source fingerprint is stale")
		}
		for _, report := range reports {
			if report.ResultDigest != tuple.ResultDigest {
				continue
			}
			if err := buildspec.ValidateReport(report); err != nil {
				return id, err
			}
			if report.ValidationOnly {
				return id, fmt.Errorf("ordinary validation report lacks certification coverage")
			}
			actual, err := buildspec.SourceDigest(report.Identity)
			if err != nil || actual != source || !buildspec.SameTuple(tuple, report.Tuple) {
				return id, fmt.Errorf("certification evidence belongs to different sources/tuple")
			}
			id.Certification, id.EvidenceDigest = "certified", tuple.ResultDigest
			if err := spec.ValidateTuple(id, profile.Tuples); err != nil {
				return id, err
			}
			return id, nil
		}
		return id, fmt.Errorf("certified tuple has no verified report")
	}
	return id, fmt.Errorf("no declared compatibility tuple")
}
