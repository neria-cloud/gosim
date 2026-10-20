package gosimtool

import (
	"debug/buildinfo"
	"encoding/json"
	"errors"
	"os"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

// NativeArtifact records the auxiliary host race oracle. It is not a translated
// application artifact and cannot supply compatibility certification evidence.
type NativeArtifact struct {
	Identity buildspec.Identity `json:"identity"`
	SHA256   string             `json:"sha256"`
}

func RecordNativeArtifact(path string, identity buildspec.Identity) error {
	identity.Race = true
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	record := NativeArtifact{Identity: identity, SHA256: buildspec.Hash(data)}
	encoded, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path+".json", encoded, 0o644); err != nil {
		return err
	}
	return VerifyNativeArtifact(path, identity)
}

func VerifyNativeArtifact(path string, identity buildspec.Identity) error {
	data, err := os.ReadFile(path + ".json")
	if err != nil {
		return err
	}
	var record NativeArtifact
	if err := json.Unmarshal(data, &record); err != nil {
		return err
	}
	source, err := buildspec.SourceDigest(identity)
	if err != nil {
		return err
	}
	actual, err := buildspec.SourceDigest(record.Identity)
	if err != nil || actual != source {
		return errors.New("native oracle source identity mismatch")
	}
	data, err = os.ReadFile(path)
	if err != nil {
		return err
	}
	if buildspec.Hash(data) != record.SHA256 {
		return errors.New("native oracle binary content mismatch")
	}
	info, err := buildinfo.ReadFile(path)
	if err != nil {
		return err
	}
	if info.GoVersion != identity.Toolchain {
		return errors.New("native oracle compiler patch mismatch")
	}
	for _, setting := range info.Settings {
		if setting.Key == "-race" && setting.Value == "true" {
			return nil
		}
	}
	return errors.New("native oracle lacks race instrumentation")
}
