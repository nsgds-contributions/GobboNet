# GobboNet — Windows installer on the Go server

A fork of [ElodineOfficial/GobboNet](https://github.com/ElodineOfficial/GobboNet)
with one purpose: give Windows the Go server that the Debian and Fedora packages
already ship, and retire the batch and PowerShell path underneath it.

**Use upstream unless you specifically want that.** Everything here is either
upstream's work or a change aimed at it. Releases, documentation and support live
there.

## What is different here

| | upstream `main` | this fork |
|---|---|---|
| Windows runtime | `fileserver.ps1` + `launch.bat` | the Go server, as on Linux |
| Windows installer | `installer-win/` (batch payload) | `installer/` (Go payload) |
| First run | console prompts | the existing web setup wizard |
| Retrieval model | fetched by `launch.bat` | offered by the wizard, supervised by the server |
| GPU offload | `gpu_layers` forced to 99 | `auto`, so llama.cpp fits it to your card |

The batch and PowerShell path is **deleted** here, along with the tests and the
installer that served it. `installer/models.ini` was generated from `launch.bat`
and is hand-maintained now.

## Building it

```sh
./build-release.sh                  # the binary, for every platform
installer/build-installer.sh        # the Windows installer, needs makensis
```

`installer/build-installer.sh` expects `GOBBONET_EXE` and `LLAMA_CPP` when they
are not already in `dist/` and `installer/vendor/`. The bundled engine is pinned
in `installer-linux/engine.sha256`; verify the archive against it before
bundling, because nothing in the build does that yet.

## Downloads

Each release carries a ready-to-run **`GobboNetSetup-*.exe`** — the Windows
installer, with `gobbonet.exe`, the web assets and llama.cpp inside it. Download
it, run it, and the setup wizard opens in your browser. Nothing else is needed.

That is the only download. The server is built for Linux, macOS and Windows too
— and CI proves those builds are reproducible — but they are not published here,
because this fork exists for the Windows installer. Build them with
`./build-release.sh`; upstream is the place to get them ready-made.

⚠ The installer is **unsigned**, so Windows SmartScreen may warn on it:
**More info → Run anyway**. [`VERIFY.md`](VERIFY.md) explains what you can check
instead, and why a certificate would not settle the question a checksum does.

## Verifying a download

Release assets are built only by GitHub Actions and carry Sigstore provenance:

```sh
gh attestation verify <file> --repo nsgds-contributions/GobboNet
```

[`VERIFY.md`](VERIFY.md) covers what that proves, what it does not, and why the
installer is still unsigned.

## Everything else

Upstream's own documentation still applies to the parts this fork does not
touch — the chat interface, character cards, retrieval, and the Linux packages.
`docs/GO_SERVER.md` describes the server itself.
