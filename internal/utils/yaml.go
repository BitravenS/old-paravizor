package utils

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
	yamler "gopkg.in/yaml.v3"
)

var Validator *validator.Validate

func init() {
	Validator = validator.New()

	// Use yaml field names in validation error messages instead of Go struct field names.
	Validator.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.Split(fld.Tag.Get("yaml"), ",")[0]
		if name == "-" {
			return ""
		}
		return name
	})

	Validator.RegisterValidation("color", ValidateColor)
	Validator.RegisterValidation("regex", ValidRegex)
}

func ParseYAML[T any](path string) (T, error) {
	var zero T
	data, err := os.ReadFile(path)
	if err != nil {
		return zero, fmt.Errorf("read %s: %w", path, err)
	}
	return parseYAMLBytes[T](path, data)
}

func ParseYAMLBytes[T any](source string, data []byte) (T, error) {
	return parseYAMLBytes[T](source, data)
}

func parseYAMLBytes[T any](source string, data []byte) (T, error) {
	var v T
	if err := yamler.Unmarshal(data, &v); err != nil {
		return v, fmt.Errorf("parse %s: %w", source, err)
	}
	if err := Validator.Struct(v); err != nil {
		return v, fmt.Errorf("invalid yaml at %s: %w", source, err)
	}
	return v, nil
}

// ParseYAMLBytesMultiDoc decodes a multi-document YAML byte slice into a slice of T.
// Documents that fail to decode are skipped. Validation and empty-doc filtering
// are the caller's responsibility (check zero-value fields like Name == "").
func ParseYAMLBytesMultiDoc[T any](data []byte) ([]T, error) {
	decoder := yamler.NewDecoder(bytes.NewReader(data))
	var results []T
	for {
		var v T
		if err := decoder.Decode(&v); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("decode multi-doc yaml: %w", err)
		}
		results = append(results, v)
	}
	return results, nil
}

func WriteYAML[T any](path string, v T) error {
	if err := Validator.Struct(v); err != nil {
		return fmt.Errorf("invalid yaml: %w", err)
	}
	data, err := yamler.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal yaml: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("commit %s: %w", path, err)
	}
	return nil
}
