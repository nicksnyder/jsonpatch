package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	jsonpatch "github.com/evanphx/json-patch"
	yaml "gopkg.in/yaml.v2"
)

var jsonDocument = []byte(`{
	"a": [
		{
			"b": 1,
			"c": 2
		},
		{
			"b": 3,
			"c": 4
		}
	]
}`)

var patchedJSONDocument = []byte(`{
	"a": [
		{
			"b": 11,
			"c": 2
		},
		{
			"b": 3
		},
		{
			"b": 5,
			"c": 6
		}
	]
}`)

var jsonPatch = []byte(`[
	{ "op": "test", "path": "/a/0/b", "value": 1 },
	{ "op": "replace", "path": "/a/0/b", "value": 11 },

	{ "op": "add", "path": "/a/-", "value": {"b": 5, "c": 6} },

	{ "op": "test", "path": "/a/1/c", "value": 4 },
	{ "op": "remove", "path": "/a/1/c" }
]`)

var yamlDocument = mustMarshal(jsonToYAML(jsonDocument))
var patchedYAMLDocument = mustMarshal(jsonToYAML(patchedJSONDocument))
var yamlPatch = mustMarshal(jsonToYAML(jsonPatch))

func TestMain(t *testing.T) {
	testCases := []struct {
		args     []string
		infiles  map[string][]byte
		outfiles map[string][]byte
		stdout   string
		exitCode int
	}{
		{
			args:     []string{"-help"},
			exitCode: 2,
			stdout:   usage,
		},

		{
			args: []string{"patch.json", "one.json"},
			infiles: map[string][]byte{
				"patch.json": jsonPatch,
				"one.json":   jsonDocument,
			},
			outfiles: map[string][]byte{
				"one.json": patchedJSONDocument,
			},
			exitCode: 0,
		},
		{
			args: []string{"patch.yaml", "one.json"},
			infiles: map[string][]byte{
				"patch.yaml": yamlPatch,
				"one.json":   jsonDocument,
			},
			outfiles: map[string][]byte{
				"one.json": patchedJSONDocument,
			},
			exitCode: 0,
		},
		{
			args: []string{"patch.yml", "one.json"},
			infiles: map[string][]byte{
				"patch.yml": yamlPatch,
				"one.json":  jsonDocument,
			},
			outfiles: map[string][]byte{
				"one.json": patchedJSONDocument,
			},
			exitCode: 0,
		},

		{
			args: []string{"patch.json", "one.json", "two.json"},
			infiles: map[string][]byte{
				"patch.json": jsonPatch,
				"one.json":   jsonDocument,
				"two.json":   jsonDocument,
			},
			outfiles: map[string][]byte{
				"one.json": patchedJSONDocument,
				"two.json": patchedJSONDocument,
			},
			exitCode: 0,
		},
		{
			args: []string{"patch.json", "one.yaml", "two.yml"},
			infiles: map[string][]byte{
				"patch.json": jsonPatch,
				"one.yaml":   yamlDocument,
				"two.yml":    yamlDocument,
			},
			outfiles: map[string][]byte{
				"one.yaml": patchedYAMLDocument,
				"two.yml":  patchedYAMLDocument,
			},
			exitCode: 0,
		},

		// Test JSON Patch that fails test.
		{
			args: []string{"patch.json", "one.json"},
			infiles: map[string][]byte{
				"patch.json": []byte(`[{ "op": "test", "path": "/a/0/b", "value": 2 }]`),
				"one.json":   jsonDocument,
			},
			outfiles: map[string][]byte{},
			exitCode: 1,
			stdout:   `error applying JSON Patch [{ "op": "test", "path": "/a/0/b", "value": 2 }] to one.json: Testing value /a/0/b failed` + "\n",
		},

		// Test batch.
		{
			args: []string{"batch.json"},
			infiles: map[string][]byte{
				"batch.json": []byte(fmt.Sprintf("[{ \"glob\": \"one.json\", \"jsonPatch\": %s }, { \"glob\": \"t*.json\", \"jsonPatch\": %s }]", jsonPatch, jsonPatch)),
				"one.json":   jsonDocument,
				"two.json":   jsonDocument,
			},
			outfiles: map[string][]byte{
				"one.json": patchedJSONDocument,
				"two.json": patchedJSONDocument,
			},
			exitCode: 0,
		},

		// Test passing JSON Patch file when batch patch file is expected.
		{
			args: []string{"patch.json"},
			infiles: map[string][]byte{
				"patch.json": []byte(`[{ "op": "test", "path": "/a/0/b", "value": 2 }]`),
			},
			outfiles: map[string][]byte{},
			exitCode: 1,
			stdout:   `invalid batch file patch.json: json: unknown field "op"` + "\n",
		},

		// Test passing batch patch file when JSON patch file is expected.
		{
			args: []string{"batch.json", "one.json"},
			infiles: map[string][]byte{
				"batch.json": []byte(`[{ "glob": "one.json", "jsonPatch": [] }]`),
				"one.json":   jsonDocument,
			},
			outfiles: map[string][]byte{},
			exitCode: 1,
			stdout:   `error applying JSON Patch [{ "glob": "one.json", "jsonPatch": [] }] to one.json: Unexpected kind: unknown` + "\n",
		},
	}

	for _, testCase := range testCases {
		t.Run(strings.Join(testCase.args, " "), func(t *testing.T) {
			indir := mustTempDir("in")
			defer os.RemoveAll(indir)
			expecteddir := mustTempDir("expected")
			defer os.RemoveAll(expecteddir)
			actualdir := mustTempDir("actual")
			defer os.RemoveAll(actualdir)

			// Setup directory of expected files.
			if err := os.Chdir(expecteddir); err != nil {
				t.Fatal(err)
			}
			for name, content := range testCase.outfiles {
				if err := ioutil.WriteFile(name, content, 0666); err != nil {
					t.Fatal(err)
				}
			}

			// Setup directory of input files.
			if err := os.Chdir(indir); err != nil {
				t.Fatal(err)
			}
			for name, content := range testCase.infiles {
				t.Logf("infile %s\n%s\n", name, content)
				if err := ioutil.WriteFile(name, content, 0666); err != nil {
					t.Fatal(err)
				}
			}

			// Run jsonpatch.
			args := append([]string{"-outdir", actualdir}, testCase.args...)
			var stdout bytes.Buffer
			code := testableMain(args, &stdout)

			// Check stdout.
			if actual := stdout.String(); actual != testCase.stdout {
				t.Fatalf("\nexpected stdout:\n%s\ngot stdout:\n%s", testCase.stdout, actual)
			}

			// Check error code.
			if code != testCase.exitCode {
				t.Fatalf("expected exit code %d; got %d", testCase.exitCode, code)
			}

			// Check expected files.
			if err := check(expecteddir, actualdir); err != nil {
				t.Fatal(err)
			}

			// Check for unexpected files.
			if err := check(actualdir, expecteddir); err != nil {
				t.Fatal(err)
			}
		})
	}

}

func check(needle, haystack string) error {
	return filepath.Walk(needle, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		expected, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(needle, path)
		if err != nil {
			return err
		}

		actual, err := ioutil.ReadFile(filepath.Join(haystack, rel))
		if err != nil {
			return err
		}

		if !jsonpatch.Equal(expected, actual) {
			return fmt.Errorf("unexpected contents %s\n%s\n%s\n%s\n%s", rel, filepath.Base(needle), expected, filepath.Base(haystack), actual)
		}
		return nil
	})

}

func mustTempDir(prefix string) string {
	outdir, err := ioutil.TempDir("", prefix)
	if err != nil {
		panic(err)
	}
	return outdir
}

func mustMarshal(buf []byte, err error) []byte {
	if err != nil {
		panic(err)
	}
	return buf
}

func jsonToYAML(j []byte) ([]byte, error) {
	var i interface{}
	if err := json.Unmarshal(j, &i); err != nil {
		return nil, err
	}
	return yaml.Marshal(i)
}

func yamlToJSON(y []byte) ([]byte, error) {
	var i interface{}
	if err := yaml.Unmarshal(y, &i); err != nil {
		return nil, err
	}
	return json.Marshal(i)
}
