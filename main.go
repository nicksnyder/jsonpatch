// Command jsonpatch applies RFC 6902 JSON Patches to JSON or YAML documents.
//
// If at least one document is provided, the patch file is parsed as a RFC 6902 JSON Patch.
//
// If no documents are provided, the patch file is parsed as a batch patch file:
//
//	[
//		{
//			"glob": "*.json"
//			"jsonPatch": [
//				{ "op": "add", "path": "/a", "value": 1 }
//			]
//		},
//		{
//			"glob": "*.yaml"
//			"jsonPatch": [
//				{ "op": "test", "path": "/b", "value": 1 },
//				{ "op": "remove", "path": "/b" }
//			]
//		}
//	]
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

var usage = `usage: jsonpatch <patch file> [<documents>]

jsonpatch applies RFC 6902 JSON Patches to JSON or YAML documents.

If at least one document is provided, the patch file is parsed as a RFC 6902 JSON Patch.

If no documents are provided, the patch file is parsed as a batch patch file:

[
	{
		"glob": "*.json" 
		"jsonPatch": [
			{ "op": "add", "path": "/a", "value": 1 }
		]
	},
	{
		"glob": "*.yaml" 
		"jsonPatch": [
			{ "op": "test", "path": "/b", "value": 1 },
			{ "op": "remove", "path": "/b" }
		]
	}
]
`

type patch struct {
	// Glob is glob pattern that determines which files the JSON Patch applies to.
	Glob string `json:"glob"`

	// Patch is a JSON Patch as defined in RFC 6902 from the IETF.
	JSONPatch json.RawMessage `json:"jsonPatch"`
}

func main() {
	os.Exit(testableMain(os.Args[1:], os.Stdout))
}

func testableMain(args []string, stdout io.Writer) int {
	flags := flag.NewFlagSet("jpatch", flag.ContinueOnError)
	flags.SetOutput(stdout)
	flags.Usage = func() {
		fmt.Fprint(stdout, usage)
	}
	outdir := flags.String("outdir", ".", "the directory where patched documents are emitted")
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 2
		}
		return 1
	}

	// Need at least one patch and one document.
	switch flags.NArg() {
	case 0:
		flags.Usage()
		return 2
	case 1:
		patch := flags.Arg(0)
		if err := applyBatchPatch(patch, *outdir); err != nil {
			fmt.Fprintln(stdout, err)
			return 1
		}
	default:
		patch := flags.Arg(0)
		documents := flags.Args()[1:]
		if err := applySinglePatch(patch, documents, *outdir); err != nil {
			fmt.Fprintln(stdout, err)
			return 1
		}
	}
	return 0
}

func applyBatchPatch(patchPath string, outdir string) error {
	patchJSON, err := readJSON(patchPath)
	if err != nil {
		return err
	}

	patches := []patch{}
	d := json.NewDecoder(bytes.NewBuffer(patchJSON))
	d.DisallowUnknownFields()
	if err := d.Decode(&patches); err != nil {
		return errors.Wrapf(err, "invalid batch file %s", patchPath)
	}

	for _, patch := range patches {
		matches, err := filepath.Glob(patch.Glob)
		if err != nil {
			return err
		}
		if err := applyJSONPatch([]byte(patch.JSONPatch), matches, outdir); err != nil {
			return err
		}
	}

	return nil
}

func applySinglePatch(patchPath string, documentPaths []string, outdir string) error {
	patchJSON, err := readJSON(patchPath)
	if err != nil {
		return err
	}

	return applyJSONPatch(patchJSON, documentPaths, outdir)
}

func applyJSONPatch(patchJSON []byte, documentPaths []string, outdir string) error {
	patch, err := jsonpatch.DecodePatch(patchJSON)
	if err != nil {
		return err
	}

	patchedDocs := map[string][]byte{}
	for _, docPath := range documentPaths {
		doc, err := readJSON(docPath)
		if err != nil {
			return err
		}

		patchedDoc, err := patch.ApplyIndent(doc, "  ")
		if err != nil {
			return errors.Wrapf(err, "error applying JSON Patch %s to %s", patchJSON, docPath)
		}

		if yamlExt(docPath) {
			var i interface{}
			if err := json.Unmarshal(patchedDoc, &i); err != nil {
				return err
			}
			y, err := yaml.Marshal(i)
			if err != nil {
				return err
			}
			patchedDoc = y
		}

		patchedDocs[docPath] = patchedDoc
	}

	for path, doc := range patchedDocs {
		outpath := filepath.Join(outdir, path)
		if err := os.MkdirAll(filepath.Dir(outpath), 0777); err != nil {
			return err
		}
		if err := ioutil.WriteFile(outpath, doc, 0644); err != nil {
			return err
		}
	}

	return nil
}

func readJSON(path string) ([]byte, error) {
	buf, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if yamlExt(path) {
		var i interface{}
		err := yaml.Unmarshal(buf, &i)
		if err != nil {
			return nil, err
		}

		i, err = convert(i)
		if err != nil {
			return nil, err
		}

		return json.Marshal(i)
	}

	return buf, nil
}

func yamlExt(path string) bool {
	switch filepath.Ext(path) {
	case ".yaml", ".yml":
		return true
	default:
		return false
	}
}

func convert(i interface{}) (interface{}, error) {
	switch x := i.(type) {
	case map[interface{}]interface{}:
		strmap := map[string]interface{}{}
		for k, v := range x {
			kstr, ok := k.(string)
			if !ok {
				return nil, fmt.Errorf("non-string key %#v with value %#v", k, v)
			}
			c, err := convert(v)
			if err != nil {
				return nil, err
			}
			strmap[kstr] = c
		}
		return strmap, nil
	case []interface{}:
		for i, v := range x {
			c, err := convert(v)
			if err != nil {
				return nil, err
			}
			x[i] = c
		}
	}
	return i, nil
}
