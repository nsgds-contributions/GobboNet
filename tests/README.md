# Tests and preview pages

Developer tooling. Nothing here ships, and nothing here is needed to run
GobboNet.

## Frontend suites (`*.mjs`)

Plain Node, no framework and no install step. They read the real files out of
`js/` and evaluate them, so they test what actually ships rather than a copy.

```sh
node tests/test-markdown-render.mjs     # one suite
for f in tests/*.mjs; do node "$f" || echo "FAILED: $f"; done   # all of them
```

`test-image-url-gate.mjs` covers one of the fixes from PR #30 (John McCardle):
it drives the real `safeImageUrl` and asserts nothing it returns can carry a
character that ends an HTML attribute or a CSS `url()`. Its companion,
`test-prompt-safety.py`, went with the batch path. Background is in
[`docs/changelog/CHANGELOG-1.7.3-prompt-and-sanitizer-fixes.md`](../docs/changelog/CHANGELOG-1.7.3-prompt-and-sanitizer-fixes.md).

`test-remote-models.mjs` covers the model dropdown in remote mode — the shape
behind #47, #48 and #27. It drives the real `loadModelsList` /
`onHeaderModelChange` in a fake DOM and includes source guards that no request
path has drifted back to a hardcoded model name. Background is in
[`docs/changelog/CHANGELOG-1.7.3-remote-model-list.md`](../docs/changelog/CHANGELOG-1.7.3-remote-model-list.md).

Two of them cover the same area from opposite ends: `test-cast-identity.mjs`
checks that a past message keeps the character that wrote it, and
`test-cast-mismatch.mjs` checks the notice shown when the *next* reply would
come from someone else.

Each resolves the repo root from its own location, so the working directory does
not matter.

## Removed with the batch path

`test-engine-args.py`, `test-launch-gpu-detect.py`, `test-setup-lan.py` and
`test-prompt-safety.py` all tested `launch.bat`, `fileserver.ps1` or
`identify-model.ps1`, which this fork deleted.

⚠ `test-engine-args.py` was the only thing pinning the llama-server command
line, by requiring every flag to match between the batch path and
`Supervisor.BuildArgs` or be listed as a reasoned difference. Nothing checks
those arguments now. A golden-file test over `BuildArgs`, plus a pass over
`installer/models.ini` asserting every entry yields a usable command, would
restore the cover without needing the batch file back.

## Preview pages (`preview/`)

A suite tells you the renderer produced the right markup. It cannot tell you
whether the result *looks* right. Open these in a browser for that:

- `preview/math-preview.html` — LaTeX and math rendering
- `preview/render-preview.html` — markdown, code blocks, chat bubbles

They load the real `css/` and `js/` from the repo root rather than copies, so
they cannot drift from what ships.

## Go tests

Alongside the code they cover, run the usual way:

```sh
go test ./...
```
