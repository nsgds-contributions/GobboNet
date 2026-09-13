# Verifying a download

The installer is built by
[`.github/workflows/release.yml`](.github/workflows/release.yml) on GitHub's
runners and carries a signed statement saying so. Nothing is built by hand.

`SHA256SUMS` is the exception: it is generated from the installer and is not
itself attested, which is why it is the weaker of the two checks below.

## The one command

```sh
gh attestation verify GobboNetSetup-1.7.3-go-abc1234.exe --repo nsgds-contributions/GobboNet
```

`--repo` rather than `--owner`: the owner form would accept an attestation from
any repository that owner controls.

⚠ **Needs a recent `gh`.** The `attestation` subcommand arrived in 2.49; a distro
package may be older (Ubuntu 24.04 still ships 2.45, where this fails with
`unknown command "attestation"`). Check with `gh --version`. Without upgrading,
the same data is reachable over the API:

```sh
gh api "repos/nsgds-contributions/GobboNet/attestations/sha256:$(sha256sum <file> | cut -d' ' -f1)"
```

That returns the signed bundle but does **not** verify it — reading a signature
is not checking one. Upgrade `gh` for the real check.

**What it proves:** this exact file was produced by that workflow file, from a
named commit of this repository, on a GitHub-hosted runner. The signature is
Sigstore's, tied to the workflow's own identity — there is no key for anyone to
leak or reuse.

**What it does not prove:** that the source is good. Provenance answers "did this
binary come from that source?", not "is that source trustworthy?". Read the code;
that is why it is here.

## The checksums

```sh
sha256sum -c SHA256SUMS
```

Confirms the bytes match what the build published. Weaker than the attestation —
anyone who could replace an asset could replace `SHA256SUMS` too — so it is a
convenience for scripts, not the security boundary. The attestation is.

## Building it yourself

```sh
./build-release.sh                    # binaries for all five targets
installer/build-installer.sh          # the Windows installer, needs makensis
```

The Go **binaries** are reproducible: same toolchain, `-trimpath`,
`CGO_ENABLED=0`, no cgo. CI rebuilds everything on a second clean runner,
extracts both sets and compares every file, failing the release on any
difference — so "you can rebuild this" is checked rather than claimed.

⚠ The `.tar.gz` and `.zip` wrappers CI builds are **not** byte-identical between
runs, and cannot be: both formats record file mtimes. That is why the check
compares what comes out of them. Those archives are not published — the release
carries the installer alone — so this is about the check, not about anything you
can download.

⚠ The **installer `.exe` is not** claimed to be reproducible. NSIS output has not
been shown to be byte-stable here, so it gets provenance and a checksum while its
*payload* stays reproducible. Saying otherwise would be the kind of promise this
file exists to avoid.

## Two more limits worth stating

**Building is automatic; publishing is not yet gated.** Nothing reaches a release
without going through the workflow, but anyone who can push a `v*` tag can start
that workflow. A required-reviewer environment on the publish job would make it a
deliberate click; until that is set, read "nothing is built by hand" as exactly
what it says, and not as "nothing ships without a human looking".

**The installer's builder is not pinned.** `makensis` comes from whatever
`ubuntu-latest` ships at run time. That is consistent with the exe not being
claimed reproducible, but it means the toolchain behind it is not fixed the way
the Go one is.

## What is deliberately missing

**The installer is unsigned.** There is no Authenticode certificate, so Windows
SmartScreen may warn on it, and provenance does not change that — SmartScreen
keys on a certificate and knows nothing about Sigstore. They answer different
questions and neither substitutes for the other.

Note also that signing would not quiet behavioural antivirus: an installer that
unpacks and launches `llama-server.exe` is a shape heuristics dislike whether or
not it is signed. Bundling the engine rather than downloading it at install time
does more for that than a certificate would. See
[`SECURITY.md`](SECURITY.md) for upstream's longer argument.

## The engine

The bundled llama.cpp is pinned by hash in
[`installer-linux/engine.sha256`](installer-linux/engine.sha256) and verified in
CI **before** it is bundled. A re-uploaded upstream asset fails the build rather
than shipping. Bumping the build number means changing that file in the same
commit; that is the point of pinning.
