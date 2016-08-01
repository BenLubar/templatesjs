package templatesjs // import "github.com/BenLubar/templatesjs"

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

func Convert(src string) (string, error) {
	return parse(src)
}

const identifierRegex = `[_\pL\pNl][_\pL\pNl\d]*`
const arrayRegex = `\[[0-9]+\]`
const namespaceRegex = `\.(?:` + identifierRegex + `|` + arrayRegex + `)`
const specialRegex = `@(?:index|key|value|first)`
const keyRegex = `(?:` + specialRegex + `|` + identifierRegex + `(?:` + namespaceRegex + `)*)`

var rawKeyRegex = regexp.MustCompile(`\{\{(\.\.\/)?(` + keyRegex + `)\}\}`)
var escapedKeyRegex = regexp.MustCompile(`\{(\.\.\/)?(` + keyRegex + `)\}`)

var functionRegex = regexp.MustCompile(`\{function\.(` + identifierRegex + `)((?:[ ]*,[ ]*` + keyRegex + `)*)\}`)
var functionsArgs = regexp.MustCompile(`[ ]*,[ ]*`)

var ifFunctionRegex = regexp.MustCompile(`<!-- IF (!?)function\.(` + identifierRegex + `)((?:[ ]*,[ ]*` + keyRegex + `)*) -->[\r\n]*`)
var ifRegex = regexp.MustCompile(`<!-- IF (!?)(\.\.\/)?(` + keyRegex + `) -->[\r\n]*`)
var elseRegex = regexp.MustCompile(`<!-- ELSE -->[\r\n]*`)
var endifRegex = regexp.MustCompile(`<!-- ENDIF (!?)(\.\.\/)?(` + keyRegex + `) -->`)

var rawSpecialRegex = regexp.MustCompile(`(\A|\})([^\{]*?)(` + specialRegex + `)`)
var loopFindRegex = regexp.MustCompile(`<!-- BEGIN (` + keyRegex + `) -->`)

func parse(template string) (string, error) {
	template, err := convertVariables(template)
	if err != nil {
		return "", err
	}

	template, err = convertConditions(template)
	if err != nil {
		return "", err
	}

	template, err = convertLoops(template, "$", "$")
	if err != nil {
		return "", err
	}

	return template, nil
}

func convertVariables(template string) (string, error) {
	for _, problem := range rawKeyRegex.FindAllString(template, 1) {
		return "", errors.Errorf("templatesjs: double braced variables are unsupported (near %q)", problem)
	}

	for _, fn := range functionRegex.FindAllStringSubmatch(template, -1) {
		search := fn[0]
		method := fn[1]
		args := functionsArgs.Split(fn[2], -1)

		replace := "{{" + convertMethod(method, args) + "}}"

		template = strings.Replace(template, search, replace, -1)
	}

	for _, key := range escapedKeyRegex.FindAllStringSubmatch(template, -1) {
		search := key[0]
		replace := "{{" + convertName(key[2]) + "}}"

		template = strings.Replace(template, search, replace, -1)
	}

	return template, nil
}

func convertConditions(template string) (string, error) {
	for _, cond := range ifFunctionRegex.FindAllStringSubmatch(template, -1) {
		search := cond[0]
		not := ""
		if cond[1] != "" {
			not = "not "
		}
		method := cond[2]
		args := functionsArgs.Split(cond[3], -1)

		replace := "{{if " + not + " (" + convertMethod(method, args) + ")}}"

		template = strings.Replace(template, search, replace, -1)
	}

	for _, cond := range ifRegex.FindAllStringSubmatch(template, -1) {
		search := cond[0]
		not := ""
		if cond[1] != "" {
			not = "not "
		}
		replace := "{{if " + not + convertName(cond[3]) + "}}"

		template = strings.Replace(template, search, replace, -1)
	}

	template = elseRegex.ReplaceAllLiteralString(template, "{{- else}}")
	template = endifRegex.ReplaceAllLiteralString(template, "{{- end}}")

	return template, nil
}

func convertLoops(template, prefix, iterator string) (string, error) {
	for i := 0; ; i++ {
		loop := loopFindRegex.FindStringSubmatch(template)
		if loop == nil {
			return template, nil
		}

		loopSearch := regexp.QuoteMeta(loop[1])
		loopRegex, err := regexp.Compile(`<!-- BEGIN (` + loopSearch + `) -->[\r\n]*([\s\S]*?)<!-- END ` + loopSearch + ` -->`)
		if err != nil {
			return "", err
		}

		loop = loopRegex.FindStringSubmatch(template)
		search := loop[0]
		key := strings.Replace(convertName(loop[1]), "$.", prefix+".", -1)

		newIterator := iterator + "i" + strconv.Itoa(i)
		newKey := "(index " + key + " " + newIterator + ")"

		body := strings.Replace(loop[2], key+".", newKey+".", -1)
		body, err = convertLoops(body, newKey, newIterator)
		if err != nil {
			return "", err
		}

		for {
			next := rawSpecialRegex.ReplaceAllString(body, "$1$2{{$3}}")
			if next == body {
				break
			}
			body = next
		}

		body = strings.Replace(body, "@index", newIterator, -1)
		body = strings.Replace(body, "@key", newIterator, -1)
		body = strings.Replace(body, "@value", newIterator+"_v", -1)
		body = strings.Replace(body, "@first", "(not "+newIterator+")", -1)

		replace := "{{range " + newIterator + ", " + newIterator + "_v := " + key + "}}" + body + "{{- end}}"

		template = strings.Replace(template, search, replace, -1)
	}
}

func convertName(name string) string {
	if strings.HasPrefix(name, "@") {
		return name
	}

	converted, remaining := "$", name

	for {
		if remaining == "" {
			return converted
		}

		if remaining[0] == '[' {
			remaining = remaining[1:]
			i := strings.IndexByte(remaining, ']')
			index := remaining[:i]
			remaining = remaining[i+1:]
			converted = "(index " + converted + " " + index + ")"
			continue
		}

		if remaining[0] == '.' {
			remaining = remaining[1:]
		}

		var name string
		if i := strings.IndexAny(remaining, ".["); i == -1 {
			name, remaining = remaining, ""
		} else {
			name, remaining = remaining[:i], remaining[i:]
		}

		switch name {
		case "length":
			converted = "(len " + converted + ")"
		default:
			converted = converted + "." + name
		}
	}
}

func convertMethod(name string, args []string) string {
	var parameters []string
	if len(args) > 1 {
		args = args[1:]
		parameters = make([]string, len(args))
		for i, arg := range args {
			parameters[i] = convertName(arg)
		}
	}

	return name + " $ " + strings.Join(parameters, " ")
}
