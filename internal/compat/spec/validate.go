package spec

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

func validateTupleRecord(t buildspec.Tuple) error {
	if err := buildspec.ValidateVersion(t.Toolchain); err != nil {
		return err
	}
	if err := buildspec.ValidatePlatform(t.HostOS, t.HostArch, t.SimulationOS, t.SimulationArch); err != nil {
		return err
	}
	if _, err := buildspec.Features(t.Features); err != nil {
		return err
	}
	if t.Experiments != "" || (t.Status != "pending" && t.Status != "certified") {
		return fmt.Errorf("invalid tuple experiments/status")
	}
	if t.Status == "pending" {
		if t.ResultDigest != "" || t.SourceDigest != "" {
			return fmt.Errorf("pending tuple must not claim source/result evidence")
		}
	} else if !buildspec.ValidDigest(t.SourceDigest) || !buildspec.ValidDigest(t.ResultDigest) {
		return fmt.Errorf("certified tuple requires source and result digests")
	}
	return nil
}

func tupleKey(t buildspec.Tuple) string {
	t.Features, _ = buildspec.Features(t.Features)
	t.Status, t.SourceDigest, t.ResultDigest = "", "", ""
	b, _ := json.Marshal(t) // record contains only JSON primitive values
	return string(b)
}

func ValidateTuple(build buildspec.Identity, tuples []buildspec.Tuple) error {
	id, err := buildspec.NormalizeIdentity(build)
	if err != nil {
		return err
	}
	var match *buildspec.Tuple
	for _, tuple := range tuples {
		if err := validateTupleRecord(tuple); err != nil {
			return err
		}
		features, _ := buildspec.Features(tuple.Features)
		if tuple.Toolchain == id.Toolchain && tuple.HostOS == id.HostOS && tuple.HostArch == id.HostArch &&
			tuple.SimulationOS == id.SimulationOS && tuple.SimulationArch == id.SimulationArch &&
			tuple.Race == id.Race && tuple.Experiments == id.Experiments && slices.Equal(features, id.Features) {
			if match != nil {
				return fmt.Errorf("ambiguous compatibility tuple")
			}
			copy := tuple
			match = &copy
		}
	}
	if match == nil {
		return fmt.Errorf("no compatibility tuple for %s %s/%s %v race=%t", id.Toolchain, id.HostOS, id.HostArch, id.Features, id.Race)
	}
	if id.Certification == "candidate" {
		if match.Status != "pending" {
			return fmt.Errorf("candidate execution requires a declared pending tuple")
		}
		return nil
	}
	if match.Status != "certified" {
		return fmt.Errorf("compatibility tuple is pending; normal commands require certified evidence")
	}
	source, err := buildspec.SourceDigest(id)
	if err != nil {
		return err
	}
	if match.SourceDigest != source || match.ResultDigest != id.EvidenceDigest {
		return fmt.Errorf("certification evidence does not match executable source/build identity")
	}
	return nil
}
