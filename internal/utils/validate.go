package utils

import (
	"regexp"
	"strconv"

	"github.com/go-playground/validator/v10"
)

var hexColorRegex = regexp.MustCompile(`^#([a-fA-F0-9]{6}|[a-fA-F0-9]{3})$`)

func ValidateColor(fl validator.FieldLevel) bool {
	s := fl.Field().String()
	if hexColorRegex.MatchString(s) {
		return true
	}
	n, err := strconv.Atoi(s)
	return err == nil && n >= 0 && n <= 255
}

func ValidRegex(fl validator.FieldLevel) bool {
	pattern := fl.Field().String()
	if pattern == "" {
		return true
	}
	_, err := regexp.Compile(pattern)
	return err == nil
}
