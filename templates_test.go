package templatesjs_test

import (
	"bytes"
	"encoding/json"
	"html/template"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/BenLubar/templatesjs"
)

func init() {
	if _, err := os.Stat("testdata/templates.js/.git"); os.IsNotExist(err) {
		cmd := exec.Command("git", "clone", "--depth=1", "https://github.com/psychobunny/templates.js.git", "testdata/templates.js")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			panic(err)
		}
	} else if err == nil {
		cmd := exec.Command("git", "pull", "--depth=1")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Dir = "testdata/templates.js"
		if err := cmd.Run(); err != nil {
			panic(err)
		}
	} else {
		panic(err)
	}
}

func readTestData() (data interface{}, err error) {
	f, err := os.Open("testdata/templates.js/tests/data.json")
	if err != nil {
		return
	}
	defer f.Close()

	err = json.NewDecoder(f).Decode(&data)
	if err != nil {
		return
	}

	err = f.Close()
	return
}

func readTestTemplates(t testing.TB) (raw, expected map[string]string, err error) {
	const templatesDirectory = "testdata/templates.js/tests/templates"

	rawFiles, err := filepath.Glob(filepath.Join(templatesDirectory, "*.tpl"))
	if err != nil {
		return
	}

	expectedFiles, err := filepath.Glob(filepath.Join(templatesDirectory, "*.html"))
	if err != nil {
		return
	}

	var b []byte

	raw = make(map[string]string)
	for _, name := range rawFiles {
		b, err = ioutil.ReadFile(name)
		if err != nil {
			return
		}

		raw[strings.TrimSuffix(filepath.Base(name), ".tpl")] = string(b)
	}

	expected = make(map[string]string)
	for _, name := range expectedFiles {
		b, err = ioutil.ReadFile(name)
		if err != nil {
			return
		}

		expected[strings.TrimSuffix(filepath.Base(name), ".html")] = string(b)
	}

	for key := range raw {
		if _, ok := expected[key]; !ok {
			t.Errorf("Missing expected file: %s.html", key)
			delete(raw, key)
		}
	}

	return
}

func TestSuite(t *testing.T) {
	data, err := readTestData()
	if err != nil {
		t.Fatal(err)
	}

	raw, expected, err := readTestTemplates(t)
	if err != nil {
		t.Fatal(err)
	}

	tmplTmpl := template.New("").Funcs(template.FuncMap{
		"canspeak": func(data, iterator, numblocks interface{}) string {
			if data.(map[string]interface{})["isHuman"].(bool) && data.(map[string]interface{})["name"].(string) == "Human" {
				return "Can speak"
			}
			return "Cannot speak"
		},
		"test": func(data interface{}) bool {
			return data.(map[string]interface{})["forum"].(string) != "" && !data.(map[string]interface{})["double"].(bool)
		},
		"isHuman": func(data, iterator interface{}) bool {
			return data.(map[string]interface{})["animals"].([]interface{})[iterator.(int)].(map[string]interface{})["isHuman"].(bool)
		},
	})

	keys := make([]string, 0, len(raw))
	for key := range raw {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var buf bytes.Buffer

	for _, key := range keys {
		tmpl, err := tmplTmpl.Clone()
		if err != nil {
			t.Errorf("%q: %v", key, err)
			continue
		}

		src, err := templatesjs.Convert(raw[key])
		if err != nil {
			t.Errorf("%q: %v", key, err)
			continue
		}

		tmpl, err = tmpl.New(key).Parse(src)
		if err != nil {
			t.Errorf("%q: %v", key, err)
			continue
		}

		buf.Reset()
		err = tmpl.Execute(&buf, data)
		if err != nil {
			t.Errorf("%q: %v", key, err)
			continue
		}

		parsed := strings.Replace(buf.String(), "\r\n", "\n", -1)
		expect := strings.Replace(expected[key], "\r\n", "\n", -1)

		if parsed != expect {
			t.Errorf("%q: result did not match:\nexpected: %q\nactual:   %q", key, expect, parsed)
		}
	}
}
