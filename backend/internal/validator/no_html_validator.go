package validator

import (
	"strings"

	"github.com/go-playground/validator/v10"
)

func NoHTMLTags(
	fl validator.FieldLevel,
) bool {
	value := fl.Field().String()
	return !strings.ContainsAny(value, "<>")
}
