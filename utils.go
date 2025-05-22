package agollolistener

import (
	"os"
	"regexp"
	"strings"
	"unicode"
)

func lowerFirst(s string) string {
	for i, v := range s {
		return string(unicode.ToLower(v)) + s[i+1:]
	}

	return ""
}

func replaceEnvVar(s string) string {
	variables := findVariables(s)
	for _, variable := range variables {
		if len(variable) > 3 {
			name := variable[2 : len(variable)-1]
			value := os.Getenv(name)
			s = strings.Replace(s, variable, value, -1)
		}
	}
	return s
}

func findVariables(input string) []string {
	re := regexp.MustCompile(`\$\{[\w\-]+}`)
	matches := re.FindAllStringSubmatch(input, -1)

	contains := make(map[string]bool)
	var variables []string
	for _, match := range matches {
		if !contains[match[0]] {
			contains[match[0]] = true
			variables = append(variables, match[0])
		}
	}
	return variables
}
