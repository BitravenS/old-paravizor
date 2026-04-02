package config

import (
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"charm.land/log/v2"
	"github.com/go-playground/validator/v10"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

var validate *validator.Validate
var hexColorRegex = regexp.MustCompile(`^#([a-fA-F0-9]{6}|[a-fA-F0-9]{3})$`)

type parsingError struct {
	path string
	err  error
}

func (e parsingError) Error() string {
	return fmt.Sprintf("failed parsing config at path %s with error %v", e.path, e.err)
}
func validateColor(fl validator.FieldLevel) bool {
	s := fl.Field().String()
	if hexColorRegex.MatchString(s) {
		return true
	}
	n, err := strconv.Atoi(s)
	return err == nil && n >= 0 && n <= 255
}

func initParser() ConfigParser {
	validate = validator.New()

	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.Split(fld.Tag.Get("yaml"), ",")[0]
		if name == "-" {
			return ""
		}
		return name
	})

	validate.RegisterValidation("color", validateColor)

	return ConfigParser{
		k: koanf.NewWithConf(conf),
	}
}

func (parser ConfigParser) loadGlobalConfig(globalCfgPath string) error {
	return parser.k.Load(file.Provider(globalCfgPath), yaml.Parser())
}

func (parser ConfigParser) mergeConfigs(globalCfgPath, userProvidedCfgPath string) (Config, error) {
	if err := parser.loadGlobalConfig(globalCfgPath); err != nil {
		return Config{}, parsingError{err: err, path: globalCfgPath}
	}
	log.Info("Loaded global config", "path", globalCfgPath)
	if err := parser.k.Load(
		file.Provider(userProvidedCfgPath),
		yaml.Parser(),
		koanf.WithMergeFunc(func(
			overrides, dest map[string]any,
		) error {

			return nil
		}),
	); err != nil {
		return Config{}, parsingError{err: err, path: userProvidedCfgPath}
	}
	log.Info("Loaded user provided config", "path", userProvidedCfgPath)

	return parser.unmarshalConfigWithDefaults()
}

// TODO: Make it load config from project directory
func (parser ConfigParser) getProvidedConfigPath(location string) string {
	var userProvidedCfgPath string
	if location != "" {
		userProvidedCfgPath = location
	} else if cfg := os.Getenv("PRVZR_CONFIG"); cfg != "" {
		userProvidedCfgPath = cfg
	}

	return userProvidedCfgPath
}

func ParseConfig(location string) (Config, error) {
	parser := initParser()

	var config Config
	var err error

	userProvidedCfgPath := parser.getProvidedConfigPath(location)

	if userProvidedCfgPath != "" {
		if err := parser.k.Load(file.Provider(userProvidedCfgPath), yaml.Parser()); err != nil {
			return Config{}, parsingError{path: userProvidedCfgPath, err: err}
		}
		log.Info("Loaded user provided config (skipping global)", "path", userProvidedCfgPath)
		return parser.unmarshalConfigWithDefaults()
	}

	globalCfgPath, err := parser.getGlobalConfigPathOrCreateIfMissing()
	if err != nil {
		return config, parsingError{path: globalCfgPath, err: err}
	}

	if userProvidedCfgPath != "" {
		mergedCfg, err := parser.mergeConfigs(globalCfgPath, userProvidedCfgPath)
		if err != nil {
			return Config{}, err
		}
		return mergedCfg, nil
	}

	if err = parser.loadGlobalConfig(globalCfgPath); err != nil {
		log.Error("failed loading global config", "err", err)
		return Config{}, parsingError{path: globalCfgPath, err: err}
	}

	return parser.unmarshalConfigWithDefaults()
}

func (parser ConfigParser) unmarshalConfigWithDefaults() (Config, error) {
	cfg := parser.getDefaultConfig()
	err := parser.k.UnmarshalWithConf("", &cfg, koanf.UnmarshalConf{Tag: "yaml"})
	if err != nil {
		return Config{}, err
	}

	err = validate.Struct(cfg)
	return cfg, err
}
