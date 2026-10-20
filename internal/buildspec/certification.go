package buildspec

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

const minimumFullFeatureCount = 3

type MatrixOptions struct {
	OutputDir            string
	ExecutionMode        string
	ContainerImageDigest string
}

type SuiteResult struct {
	Name      string `json:"name"`
	Passed    bool   `json:"passed"`
	LogDigest string `json:"log_digest"`
}

type CertificationReport struct {
	FrameworkDigest      string             `json:"framework_digest"`
	Schema               int                `json:"schema"`
	Tuple                CertificationTuple `json:"tuple"`
	Identity             Identity           `json:"identity"`
	ExecutionMode        string             `json:"execution_mode"`
	WorkerArch           string             `json:"worker_arch"`
	ContainerImageDigest string             `json:"container_image_digest"`
	Suites               []SuiteResult      `json:"suites"`
	ResultDigest         string             `json:"result_digest"`
}

func TupleFor(id Identity) CertificationTuple {
	return CertificationTuple{Toolchain: id.Toolchain, HostOS: id.HostOS, HostArch: id.HostArch, SimulationOS: id.SimulationOS, SimulationArch: id.SimulationArch, Features: slices.Clone(id.Features), Race: id.Race, Experiments: id.Experiments}
}

func SameTuple(a, b CertificationTuple) bool {
	a.Features, _ = Features(a.Features)
	b.Features, _ = Features(b.Features)
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

func ReportDigest(report CertificationReport) (string, error) {
	report.ResultDigest = ""
	report.Suites = slices.Clone(report.Suites)
	slices.SortFunc(report.Suites, func(a, b SuiteResult) int { return strings.Compare(a.Name, b.Name) })
	data, err := json.Marshal(report)
	if err != nil {
		return "", err
	}
	return Hash(data), nil
}

func ValidateReport(report CertificationReport) error {
	if report.Schema != 2 {
		return fmt.Errorf("unsupported certification report schema")
	}
	id, err := NormalizeIdentity(report.Identity)
	if err != nil {
		return err
	}
	if !SameTuple(report.Tuple, TupleFor(id)) {
		return fmt.Errorf("report tuple does not match executed identity")
	}
	source, err := SourceDigest(id)
	if err != nil || source != report.FrameworkDigest {
		return fmt.Errorf("report framework identity mismatch")
	}

	if err := ValidateExecution(report.Identity, report.WorkerArch, report.ExecutionMode, report.ContainerImageDigest); err != nil {
		return err
	}
	if err := validateReportSuites(report.Suites, len(id.Features) < minimumFullFeatureCount); err != nil {
		return err
	}
	digest, err := ReportDigest(report)
	if err != nil || digest != report.ResultDigest {
		return fmt.Errorf("certification result digest mismatch")
	}
	return nil
}

func validateReportSuites(suites []SuiteResult, reducedFeatures bool) error {
	seen := map[string]bool{}
	for _, suite := range suites {
		if suite.Name == "" || seen[suite.Name] || !suite.Passed || !ValidDigest(suite.LogDigest) {
			return fmt.Errorf("failed, duplicate or incomplete suite %q", suite.Name)
		}
		seen[suite.Name] = true
	}
	for _, name := range []string{"build", "execute", "native"} {
		if !seen[name] {
			return fmt.Errorf("missing required suite %s", name)
		}
	}
	if !seen["large"] {
		return fmt.Errorf("missing required large suite; ordinary validation cannot certify a tuple")
	}
	if reducedFeatures {
		for _, name := range []string{"native-units"} {
			if !seen[name] {
				return fmt.Errorf("missing reduced-feature suite %s", name)
			}
		}
	}
	return nil
}

func ValidateExecution(id Identity, workerArch, mode, image string) error {
	if workerArch != id.HostArch {
		return fmt.Errorf("report worker architecture mismatch")
	}
	switch mode {
	case "native":
		if image != "" {
			return fmt.Errorf("native report claims a container image")
		}
	case "container-native", "container-emulated":
		if id.HostOS != "linux" || !strings.HasPrefix(image, "sha256:") || !ValidDigest(strings.TrimPrefix(image, "sha256:")) {
			return fmt.Errorf("container report requires Linux and an image digest")
		}
	default:
		return fmt.Errorf("unknown execution mode %q", mode)
	}

	return nil
}
