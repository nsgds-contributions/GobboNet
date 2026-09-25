# GobboNet — Windows installer on the Go server

A fork of [ElodineOfficial/GobboNet](https://github.com/ElodineOfficial/GobboNet)
with one purpose: a Windows install in which the Go server does the whole job —
first-run setup in the web wizard, llama.cpp and the retrieval model started and
supervised by the server — with no batch launcher in front of it.

**Use upstream unless you specifically want that.** Everything here is either
upstream's work or a change aimed at it. Releases, documentation and support live
there.

## What is different here

| | upstream 1.7.6 | this fork |
|---|---|---|
| Windows first run | `launch.bat`, at a console prompt | the web setup wizard, as on Linux |
| Windows runtime | `launch.bat` hands over to `gobbonet.exe` (or `fileserver.ps1` without one) | `gobbonet.exe` alone |
| Windows installer | `installer-win/`: `launch.bat` and `gobbonet.exe`; llama.cpp fetched on first launch | `installer/`: `gobbonet.exe` with llama.cpp inside |
| Retrieval model | started by `launch.bat` | offered by the wizard, supervised by the server |
| Phone access | `setup-lan.bat`, run by hand as Administrator | one wizard question: sets the listen address and runs `setup-lan.bat` |

`launch.bat`, `fileserver.ps1`, `hw-recommend.ps1`, `identify-model.ps1`,
`installer-win/` and the tests that cover them are still in this tree, unchanged
from upstream, so that merging a new upstream release stays clean. Nothing this
fork builds ships them. `installer/models.ini` is still generated from
`launch.bat` by `installer/gen-catalog.py`.

## Building it

```sh
./build-release.sh                  # the binary, for every platform
installer/build-installer.sh        # the Windows installer; needs makensis, python3, curl and unzip
```

The chat page is compiled into the binary — `build-release.sh` stages it first —
so nothing ships beside it. `installer/build-installer.sh` needs `GOBBONET_EXE`
pointing at a `gobbonet.exe` unzipped from `dist/`. It fetches llama.cpp itself
and refuses the archive unless it matches the hash pinned in
[`engine.sha256`](engine.sha256). Setting `LLAMA_CPP` to an engine you already
have skips that check, and the installed `llama-cpp\ENGINE.txt` says so.

## Everything else

Upstream's own documentation still applies to the parts this fork does not
touch — the chat interface, character cards, retrieval, device sync, encrypted
history and the Linux packages. `docs/GO_SERVER.md` describes the server itself.
