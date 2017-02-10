package template

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/go-multierror"
)

var templateFormatRegex = regexp.MustCompile(`\{\{([-\w\p{L}]+)\}\}`)

func Evaluate(content []byte, variables Variables) ([]byte, error) {
	var variableErrors error

	used := make(map[string]bool, len(variables))
	for _, key := range variables {
		used[key] = false
	}

	filled := templateFormatRegex.ReplaceAllFunc(content, func(match []byte) []byte {
		key := string(templateFormatRegex.FindSubmatch(match)[1])

		value, found := variables[key]
		if !found {
			variableErrors = multierror.Append(variableErrors, fmt.Errorf("unbound variable in template: '%s'", key))
			return match
		}

		saveValue, _ := json.Marshal(value)
		used[key] = true

		return []byte(saveValue)
	})

	var unused []string
	for key, wasUsed := range used {
		if !wasUsed {
			unused = append(unused, key)
		}
	}

	if len(unused) > 0 {
		variableErrors = multierror.Append(variableErrors, fmt.Errorf("Unused variables: %s", strings.Join(unused, ",")))
	}

	return filled, variableErrors
}

func EvaluateEmpty(content []byte) []byte {
	return templateFormatRegex.ReplaceAll(content, []byte(`""`))
}
