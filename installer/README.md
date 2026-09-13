# GobboNet Windows installer

Turns "downloaded the setup exe" into "chatting" without asking a question at a
prompt. ⚠ Not without a console *window*: `gobbonet.exe` is built for the
console subsystem, so starting it opens one that prints the banner and stays for
the life of the server, and closing it stops the server. That is upstream's
shape for the Go server on every platform. What the retired `launch.bat` asked at a `C:\>` prompt is split in two:
the machine-specific part — hardware probe, file layout, shortcuts — is a wizard
page here, and everything about the user's own choices (password, backend,
models) belongs to the web setup wizard the finish page launches.

## Wizard flow

| # | Page | Notes |
|---|------|-------|
| 1 | Welcome | Elodine's 1.3 artwork and copy, reworded for the bundled engine |
| 2 | Directory | Per-user, `$LOCALAPPDATA\GobboNet`, no elevation |
| 3 | Hardware | Runs `hardware-probe.ps1` out of `$PLUGINSDIR`, shows GPU/VRAM/RAM/disk |
| 4 | Install | Copies files, writes `model_dir` on a first install only |
| 5 | Finish | **Start GobboNet** |

⛔ **LAN access is not here either, and must not come back.** This page used to
carry a "set up phone access" checkbox that ran `setup-lan.bat`. That opens the
firewall and nothing else; the web wizard asks the same question a minute later
and is what writes `listen_host`, defaulting to loopback. Ticking one and taking
the other's default left an open firewall in front of a socket bound to
127.0.0.1. Measured symptom: the phone cannot connect, reports a timeout, and
nothing on the machine says why.

The wizard owns both halves now and runs `setup-lan.bat` itself, elevated, after
its own reply is on the wire. **Declining that prompt writes `listen_host` back
to 127.0.0.1** rather than leaving the socket bound wide with no rule. Observed
before that change: declining, then being shown Windows' own "Allow access?"
dialog at first listen — a second prompt for the choice just refused, whose
answer writes rules named after the executable that `teardown-lan.bat` did not
then remove. Removing the LAN rules at uninstall reverts the bind for the same
reason, unelevated from NSIS so it writes the config of the user being
uninstalled. `gobbonet doctor` cross-checks the two, and the startup banner
re-checks the firewall on every run.

**Backend, model and password are not here.** `internal/setup` already asks all
three, downloads with whatever verification the entry allows — see *Download
integrity* below — and writes the completion marker, and it is what the Linux
packages use. Two wizards disagreeing about the same three
answers is worse than one, so this installer keeps only what an installer can
uniquely do and hands the rest over.

⛔ `server_exe` is the wizard's alone, in both directions, and this installer no
longer touches it. Writing it here let a stale value override a remote choice the
wizard had just made; clearing it here broke every upgrade of a working local
install. `config.Mode()` reads any non-empty value as local mode, and nothing in
the wizard can undo either.

The uninstaller asks about settings, models and the LAN rules as checkboxes on
one page rather than three sequential dialogs, so the explanations stay on
screen next to the choice they qualify.

⚠ The LAN box is offered whenever `teardown-lan.bat` is present, not only when
`setup-lan.bat` has run. The old gate gave someone who had merely answered
Windows' own "Allow access?" prompt no way to remove the rules that created —
it tested whether *our* script ran as a proxy for whether rules exist, and on
the common path that proxy is false. `teardown-lan.bat` deletes by **program
path** as well as by name for the same reason: Windows names its rules after the
executable, so a name list never matched them, and they survive an uninstall to
apply again on reinstall to the same folder.

Page 4 runs **before** the install section, so `$INSTDIR` is still empty when
the probe fires. `.onInit` extracts `hardware-probe.ps1` into `$PLUGINSDIR`
and the page runs that copy, writing `hardware.json` and `hardware.ini` there
too; the install section copies `hardware.json` into `$INSTDIR` afterwards.
⚠ Nothing reads that file — `launch.bat` did, and it is gone; teaching the web
wizard to preselect a model from it is the open follow-up. Reading the probe from
`$INSTDIR` instead — as an earlier revision did — invokes a path that does not
exist yet, and every install silently takes the "could not read this machine's
hardware" branch.

## What is bundled vs downloaded

**Bundled:** `gobbonet.exe`, web assets, llama.cpp, `hardware-probe.ps1` and the
LAN/stop scripts. `setup-lan.bat` is still installed and still on the Start menu:
the wizard runs once, and it is the path for changing your mind later. Run by
hand it does only the firewall half, so its closing text now says which command
does the other.
**Downloaded:** nothing. The installer fetches no file at all now — models are the
web wizard's job, over the server's download path, with whatever verification the
catalogue entry allows (see *Download integrity*).

That split is not arbitrary. The retired `launch.bat` documented at length that
`cmd → temp .ps1 with Bypass → downloads an executable archive` is the shape
behavioral AV reads as malware staging, and that it kills the process tree
with no error text. An unsigned installer fetching a zip of `.exe` files is
the same shape with a worse parent process, so llama.cpp ships inside the
installer. A `.gguf` is inert data and carries no such signature — and it is
also the only file too large to bundle.

The PowerShell the installer *does* run only reads WMI, the registry and
`nvidia-smi`. It downloads nothing, so it is not the pattern that caused the
trouble.

## The catalogue is hand-maintained

`models.ini` was generated from `launch.bat` by `gen-catalog.py`, which parsed
the menu lines, the VRAM ladder and the download blocks so the two could not
drift. `launch.bat` is gone from this fork, so its generator went with it and
the file is now the source of truth in its own right.

Edit it directly. `build-installer.sh` requires it and fails loudly if it is
missing, but nothing regenerates or validates it any more — a wrong `ctx` or
`min_vram` reaches users unchallenged.

⚠ Its rungs are known to be optimistic: the 3B entry pairs `ctx=32768` with
`min_vram=6`, and that will not load on a 6 GB card.

## Download integrity

Mirrors the policy the batch path used, because HuggingFace serves an LFS
*pointer* — a few hundred bytes of text — instead of the model when things
go wrong, and it arrives as a clean HTTP 200:

- hash mismatch → fatal, file deleted
- pointer unreadable or unparseable → warn and continue
- file under 1 GB → fatal, **but only when there was no hash to check**

That last condition matters: the floor was written for multi-GB chat models and
ran even after a checksum verified, so it deleted a correct 146 MB retrieval
model. A verified hash already proves the bytes.

This lives in `internal/modelfetch` now rather than in the installer, since the
wizard does the downloading.

## Building

```sh
sudo apt-get install nsis          # 3.09 preferred; see below
../build-release.sh                # produces dist/<version>/…windows-amd64.zip
unzip -d /tmp/gn dist/*/gobbonet-*-windows-amd64.zip

# llama.cpp is bundled, so it has to be on disk first
mkdir -p vendor/llama-cpp
#   https://github.com/ggml-org/llama.cpp/releases  →  -bin-win-vulkan-x64.zip
#   extract it into installer/vendor/llama-cpp/
#
# It must be the VULKAN asset. The CPU-only zip has the same filenames minus
# ggml-vulkan.dll, and an installer built from it runs everything on the
# processor. gpu_layers defaults to auto so llama.cpp fits the offload itself,
# which means the difference is silent: no error, just a slow machine.
# build-installer.sh refuses that unless you pass LLAMA_BACKEND=cpu.

GOBBONET_EXE=/tmp/gn/*/gobbonet.exe ./build-installer.sh
```

Elodine built 1.3 with **NSIS 3.09**. Debian bookworm ships 3.08. The script
warns on a mismatch rather than failing — it builds fine either way, but a
version match keeps the installer diffable against one Elodine builds, which
matters for review.

## Layout

```
models.ini           the model catalogue; hand-maintained, edit directly
gobbonet.nsi         the wizard
build-installer.sh   stages payload/, runs makensis
art/                 modern-header.bmp, modern-wizard.bmp, gobbonet.ico
                     (extracted from GobboNetSetup-1.3.exe — Elodine's work)
payload/             GENERATED staging folder. Not committed.
vendor/llama-cpp/    the bundled engine. Not committed (fetched by hand).
```

`vendor/` lives here rather than at the repo root on purpose: a directory
named `vendor` at a Go module root is reserved by the toolchain, and putting
non-Go files there breaks `go build`.

## Model swaps stall on Windows

⚠ `terminateGroup(pgid, false)` runs `taskkill /PID <pid> /T` without `/F`. Observed on
real hardware: it is refused on every swap, and the old code then waited the full 5 s grace
period for a stop nothing had accepted -- twice, once in `stop()` and once in `reapGroup()`
-- so each model change cost about eleven seconds of doing nothing before the forced kill
that actually worked. A refusal is now escalated immediately; a *delivered* stop still gets
its grace period.

That only removes the waiting; **why** the stop is refused is still unknown, and the fix
does not depend on knowing. A real graceful shutdown would mean
`CREATE_NEW_PROCESS_GROUP` plus `GenerateConsoleCtrlEvent`, which llama.cpp's own console
handler would answer -- untested, and it risks signalling this process, so it wants its own
hardware check.

Upstream's code, unchanged here apart from the wait.

## Not done yet

- ~~Run it on Windows~~ — **done.** Install, the wizard end to end, both model
  downloads, chat, `doctor`, uninstall with every combination of the checkboxes,
  reinstall over each, a reboot, LAN access from a phone, and switching models
  in the header dropdown have all been exercised on real hardware. Six defects
  came out of it that no fixture testing here would have found; each is in the
  commit message with its reproduction.

- ⚠ **`hardware.json` still has no reader.** The probe tells the user what their
  machine has and feeds no decision, which is how a 6 GB card was offered a model
  with a context it cannot hold. Teaching the wizard to preselect from it is the
  main open piece.
- **Unsigned.** See the signing discussion — a cert changes the SmartScreen
  story but not the behavioral-AV story, which is why the bundle/download
  split above still stands regardless.
- The `launch.exe` / `launchLAN.exe` C shims from 1.3 are deliberately
  dropped. They existed to locate the install folder and `ShellExecute` a
  `.bat`; `gobbonet.exe` is a real executable, so the shortcuts point
  straight at it.
