# jsonpatch

The `jsonpatch` command applies RFC 6902 JSON Patches to JSON or YAML documents.

## Example

document.json

```json
{
  "a": 1
}
```

patch.json

```json
[{ "op": "add", "path": "/b", "value": 2 }]
```

Running jsonpatch

```sh
$ jsonpatch patch.json document.json
$ cat document.json
{
  "a": 1,
  "b": 2
}
```

## Install

Install the latest with `go get -u github.com/nicksnyder/jsonpatch`.
