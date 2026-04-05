package tool

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"

	"charm.land/log/v2"

	"github.com/bitravens/paravizor/v1/internal/items"
)

// ParseOutput parses tool stdout according to the output config and returns items.
// Supported formats: line, json, jsonl, csv, regex.
func ParseOutput(reader io.Reader, cfg OutputConfig, produces items.ItemType, source string) ([]items.Item, error) {
	switch cfg.Format {
	case "line":
		return parseLine(reader, cfg, produces, source)
	case "json":
		return parseJSON(reader, cfg, produces, source)
	case "jsonl":
		return parseJSONL(reader, cfg, produces, source)
	case "csv":
		return parseCSV(reader, cfg, produces, source)
	case "regex":
		return parseRegex(reader, cfg, produces, source)
	default:
		return nil, fmt.Errorf("unsupported output format: %s", cfg.Format)
	}
}

func parseLine(reader io.Reader, cfg OutputConfig, produces items.ItemType, source string) ([]items.Item, error) {
	var result []items.Item
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if item := lineToItem(line, produces, source, cfg.Fields); item != nil {
			result = append(result, item)
		}
	}

	return result, scanner.Err()
}

func parseJSONL(reader io.Reader, cfg OutputConfig, produces items.ItemType, source string) ([]items.Item, error) {
	var result []items.Item
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			log.Debug("malformed line", "error", err)
			continue
		}

		if item := mapToItem(obj, produces, source, cfg.Fields); item != nil {
			result = append(result, item)
		}
	}

	return result, scanner.Err()
}

func parseJSON(reader io.Reader, cfg OutputConfig, produces items.ItemType, source string) ([]items.Item, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read json: %w", err)
	}

	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}

	var result []items.Item
	switch v := raw.(type) {
	case []any:
		for _, elem := range v {
			if obj, ok := elem.(map[string]any); ok {
				if item := mapToItem(obj, produces, source, cfg.Fields); item != nil {
					result = append(result, item)
				}
			}
		}
	case map[string]any:
		if item := mapToItem(v, produces, source, cfg.Fields); item != nil {
			result = append(result, item)
		}
	}

	return result, nil
}

func parseCSV(reader io.Reader, cfg OutputConfig, produces items.ItemType, source string) ([]items.Item, error) {
	r := csv.NewReader(reader)
	headers, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read csv headers: %w", err)
	}

	var result []items.Item
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Debug("malformed csv line", "error", err)
			continue
		}

		obj := make(map[string]any, len(headers))
		for i, header := range headers {
			if i < len(record) {
				obj[header] = record[i]
			}
		}

		if item := mapToItem(obj, produces, source, cfg.Fields); item != nil {
			result = append(result, item)
		}
	}

	return result, nil
}

func parseRegex(reader io.Reader, cfg OutputConfig, produces items.ItemType, source string) ([]items.Item, error) {
	re, err := regexp.Compile(cfg.Pattern)
	if err != nil {
		return nil, fmt.Errorf("compile regex pattern: %w", err)
	}

	var result []items.Item
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	names := re.SubexpNames()

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		matches := re.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		obj := make(map[string]any, len(names))
		for i, name := range names {
			if name != "" && i < len(matches) {
				obj[name] = matches[i]
			}
		}

		if item := mapToItem(obj, produces, source, cfg.Fields); item != nil {
			result = append(result, item)
		}
	}

	return result, scanner.Err()
}

// lineToItem converts a plain text line to the appropriate Item type.
// For simple formats where the entire line is the item value.
func lineToItem(line string, produces items.ItemType, source string, _ map[string]string) items.Item {
	switch produces {
	case items.TypeDomain:
		return items.DomainItem{Name: line, SourceName: source}
	case items.TypeURL:
		return items.URLItem{FullURL: line, SourceName: source}
	case items.TypeIP:
		return items.IPItem{Address: line, SourceName: source}
	default:
		return nil
	}
}

// mapToItem converts a parsed key-value object to the appropriate Item type
// using the field mapping from the output config.
// Field mapping values may use a simple JSON-path prefix "$.host" → "host".
func mapToItem(obj map[string]any, produces items.ItemType, source string, fields map[string]string) items.Item {
	get := func(key string) string {
		key = strings.TrimPrefix(key, "$.")
		if v, ok := obj[key]; ok {
			return fmt.Sprintf("%v", v)
		}
		return ""
	}

	switch produces {
	case items.TypeDomain:
		name := ""
		if nameField, ok := fields["name"]; ok {
			name = get(nameField)
		}
		if name == "" {
			name = get("host")
		}
		if name == "" {
			return nil
		}
		return items.DomainItem{Name: name, SourceName: source}

	case items.TypeURL:
		url := ""
		if urlField, ok := fields["full_url"]; ok {
			url = get(urlField)
		}
		if url == "" {
			url = get("url")
		}
		if url == "" {
			return nil
		}
		return items.URLItem{FullURL: url, SourceName: source}

	case items.TypeIP:
		addr := ""
		if addrField, ok := fields["address"]; ok {
			addr = get(addrField)
		}
		if addr == "" {
			addr = get("ip")
		}
		if addr == "" {
			return nil
		}
		return items.IPItem{Address: addr, SourceName: source}

	default:
		return nil
	}
}
