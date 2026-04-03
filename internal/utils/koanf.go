package utils

import (
	"fmt"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

var koanfConf = koanf.Conf{
	Delim:       ".",
	StrictMerge: true,
}

// NewKoanf returns a fresh koanf instance using the standard delimiter config.
func NewKoanf() *koanf.Koanf {
	return koanf.NewWithConf(koanfConf)
}

// LoadYAMLFile loads a single YAML file into a fresh koanf instance.
func LoadYAMLFile(path string) (*koanf.Koanf, error) {
	k := NewKoanf()
	if err := k.Load(file.Provider(path), yaml.Parser()); err != nil {
		return nil, fmt.Errorf("load %s: %w", path, err)
	}
	return k, nil
}

// MergeYAMLFiles loads base first, then merges each override file on top in order.
// Keys present in override files win over base. Missing override files are silently skipped
// when skipMissing is true.
func MergeYAMLFiles(skipMissing bool, paths ...string) (*koanf.Koanf, error) {
	k := NewKoanf()
	for _, path := range paths {
		err := k.Load(
			file.Provider(path),
			yaml.Parser(),
			koanf.WithMergeFunc(shallowMerge),
		)
		if err != nil {
			if skipMissing && isNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("load %s: %w", path, err)
		}
	}
	return k, nil
}

// UnmarshalKoanf unmarshals k into a value of type T, starting from defaults.
// Struct tags used for mapping: "yaml".
func UnmarshalKoanf[T any](k *koanf.Koanf, defaults T) (T, error) {
	v := defaults
	if err := k.UnmarshalWithConf("", &v, koanf.UnmarshalConf{Tag: "yaml"}); err != nil {
		return v, fmt.Errorf("unmarshal config: %w", err)
	}
	if err := Validator.Struct(v); err != nil {
		return v, fmt.Errorf("invalid config: %w", err)
	}
	return v, nil
}

// shallowMerge copies all keys from src into dest, with src winning on conflicts.
// This is intentionally shallow at the top level — koanf handles nested keys
// via its own dotted-key flattening before this function is called.
func shallowMerge(src, dest map[string]any) error {
	for k, v := range src {
		dest[k] = v
	}
	return nil
}

// isNotExist reports whether err indicates a file-not-found condition.
func isNotExist(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "no such file") ||
		strings.Contains(s, "cannot find") ||
		strings.Contains(s, "does not exist")
}
