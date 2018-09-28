# jsonpatch

The `jsonpatch` command applies RFC 6902 JSON Patches to JSON or YAML documents.

## Install

Install the latest with `go get -u github.com/nicksnyder/jsonpatch`.

## Example

document1.json

```json
{
  "a": 1
}
```

document2.json

```json
{
  "b": 2
}
```

patch.json

```json
[{ "op": "add", "path": "/c", "value": 3 }]
```

Running jsonpatch

```sh
$ jsonpatch patch.json document1.json document2.json
$ cat document1.json
{
  "a": 1,
  "c": 3
}
$ cat document2.json
{
  "b": 2,
  "c": 3
}
```

## Batch example

document1.json

```json
{
  "a": 1
}
```

document2.json

```json
{
  "b": 2
}
```

batch.json

```json
[
  {
    "glob": "document1*",
    "jsonPatch": [{ "op": "add", "path": "/c", "value": 3 }]
  },
  {
    "glob": "document2*",
    "jsonPatch": [{ "op": "add", "path": "/d", "value": 4 }]
  }
]
```

Running jsonpatch

```sh
$ jsonpatch batch.json
$ cat document1.json
{
  "a": 1,
  "c": 3
}
$ cat document2.json
{
  "b": 2,
  "c": 4
}
```
