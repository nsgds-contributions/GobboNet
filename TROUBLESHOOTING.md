# Troubleshooting GobboNet

Most problems land in one of four buckets. Work down in order — the first
one is far more common than people expect.

---

## A custom port did not take, or "I rebooted and it started working"

Both have the same short list of causes.

**Something was already holding the port.** Almost always a GobboNet file
server left over from a previous run — closing the launcher window does not
stop it. A reboot clears it, which is why rebooting appears to fix a setting
that was never wrong. From 1.5.9 the launcher checks first and names the
holder:

```
[*] Port 9066 is already in use by PID 9312 (gobbonet.exe).
```

`gobbonet doctor` reports the same thing under **WEB PORT**, including whether
the holder is another GobboNet — it recognises its own login page.

End that PID in Task Manager and start again — no reboot needed.

**The port file could not be read.** `.gobbonet-port` must contain a single
number. Earlier builds fell back to 9066 silently if it contained anything
else, so a port chosen during setup could vanish with no explanation. The
launcher now strips stray characters and, if nothing usable is left, prints
what the file actually contained.

**The chat works here but not on your phone.** The firewall rule and the
Windows URL reservation are both per-port, and neither carries over when the
port changes — so upgrading from a build that used 8080, or picking a custom
port, left them pointing at the wrong number.

Before 1.7.3 this was easy to hit and hard to see, because `setup-lan.bat`
resolved the port from `.gobbonet-port`, which is only written *after* the
server binds. Run from the installer's finish page it read a file that did not
exist yet and fell back to 9066 — correct on a fresh install by luck, wrong for
anyone upgrading from 8080. From 1.7.3 it asks the binary instead
(`gobbonet config get listen_port`), which is right before the server has ever
run.

**LAN access is two things, and one without the other looks like a fault.** The
server has to be listening on the network (`listen_host = 0.0.0.0`) *and* the
firewall has to allow it. Choosing phone access in the setup wizard does both;
declining its Administrator prompt switches both back off, deliberately, so you
are left on this machine only rather than half-configured.

Fix: right-click **setup-lan.bat** → Run as administrator, then run
`gobbonet config set listen_host 0.0.0.0` in an ordinary window — not the
elevated one, which may write a different user's settings. `gobbonet doctor`
reads the **LAN ACCESS** section for you and names whichever half is missing.

⚠ **Windows may also have written its own rule.** The first time GobboNet binds
a network port, Windows asks "Allow access?", and answering yes creates inbound
rules named after the executable. Those are what make a phone work on a machine
where `setup-lan.bat` was never run — and `teardown-lan.bat` removes them along
with its own. To see them:

```
netsh advfirewall firewall show rule name=all dir=in | findstr /i gobbonet
```

To set a port for one run only:

```
gobbonet config set listen_port 9067
gobbonet
```

`GEMMA_LISTEN_PORT` is the old spelling. It still works and warns, but prefer
the current one — from 1.7.3 `setup-lan.bat` reads both, and before that it
read only the old one, so following the deprecation moved the server without
moving the firewall rule.

That beats `.gobbonet-port`, which beats the 9066 default.

---

## "My character's picture disappeared after updating"

It shows a letter now instead of the avatar. Nothing was deleted.

Some cards store the picture **inside** the card; others just hold a web
address pointing at one. From 1.5.9, GobboNet no longer loads the second
kind by default — fetching a picture from a website tells that website your
IP address, and it happens the moment a message renders, without you
clicking anything. In an app whose whole point is that nothing leaves your
machine, that was the one thing that did.

Pictures pasted or uploaded directly into a card are unaffected and always
work.

**To load them again:** Settings → **Remote Images In Cards** → tick *Allow
remote images*. Avatars and backgrounds repaint immediately; no reload.

The address is still on the card either way, so you can flip this back and
forth freely. If you would rather make the picture permanent, download it
and re-upload it into the card — then it lives in the card and the setting
stops mattering.

You should have seen a one-time notice about this on first launch. It only
appears if you actually have an affected card, and only once.

---

## "My chats vanished after updating"

They did not. 1.5.5 moved the default port from **8080 to 9066**, and a
browser keys its storage to the exact address you opened — so
`localhost:9066` is a different origin from `localhost:8080` and starts
empty.

Your conversations are safe in `state.json` — in the **data** folder, not the
install folder, so wiping or reinstalling GobboNet does not touch them.
`gobbonet doctor` prints the path under **CONFIG** (`%USERPROFILE%\.local\share\gobbonet`
by default). GobboNet restores them automatically the first time you open the
new address. If the sidebar is empty after a moment, force it from the
Data panel with **Restore from server**.

The old `:8080` origin still holds a copy too. Clearing it is optional;
see [`docs/PURGE.md`](docs/PURGE.md) if you want it gone.

### Why the port moved

8080 is the most contended port on a developer machine — Tomcat, Jenkins,
and most tutorial dev servers all reach for it, and Hyper-V, WSL2 and
Docker reserve blocks that swallow it. Squatting on it made GobboNet the
thing you had to close to get your own work started. 9066 ("gobb" on a
keypad) sits clear of all of that.

To pick a different port, set one during install, or edit
`.gobbonet-port` in the install folder — one line, just the number. For a
single run, `set GEMMA_LISTEN_PORT=8420` before launching wins over both.

---

## The chat page will not load (nothing on :9066)

**Read the log first.** `gobbonet doctor` reports the paths, the port, the
engine and whether anything is already bound. A start that fails before it can
say so writes `startup-error.log` into the config folder, and the engine's own
output goes to `llama-server.log` in the data folder; `doctor` prints both
paths.

```
gobbonet.exe doctor
type "%USERPROFILE%\.config\gobbonet\startup-error.log"
```

Those separate the failures that look identical from outside:

| What you see | What it means |
|---|---|
| `[FATAL] No access secret provided` | Not a port problem at all — see *Password* below |
| doctor: `in use: YES` and an owner that is not GobboNet | Something else holds the port — checklist below |
| doctor: `in use: YES … 401 from our own auth` | GobboNet is already running; you have two copies |
| doctor: `listen_host: 127.0.0.1 — this machine only` | Working, but this PC only — see *phone* above |
| `could not bind … either` | Port genuinely unavailable — checklist below |
| *(the window closes with nothing)* | Read `startup-error.log`; if that is absent too, antivirus quarantine |

### "netstat says nothing is using the port"

That can be true and the port still unbindable. **netstat cannot see
Windows port reservations.** Hyper-V, WSL2, Docker Desktop and the Windows
NAT service reserve large dynamic TCP blocks, and a web port can land inside one
often enough to be a leading suspect. Check with:

```
netsh interface ipv4 show excludedportrange protocol=tcp
```

If a range covers your port, either use a different port:

```
gobbonet config set listen_port 8420
gobbonet
```

…or reserve the port back and **reboot**:

```
netsh int ipv4 add excludedportrange protocol=tcp startport=9066 numberofports=1
```

`setup-lan.bat` now checks this for you and says so.

### Other bind causes

```
netsh http show urlacl url=http://+:9066/     :: missing reservation? run setup-lan.bat as admin
netsh http show servicestate                  :: another service (IIS, VMware, Citrix) owning it?
```

Note that a missing URL ACL is no longer fatal — the server falls back to
`127.0.0.1` only. The chat works on this PC; phones will not reach it until
you run `setup-lan.bat` as Administrator.

---

## Password problems

The password is `access_secret` in `config.toml`, not a separate file. A
missing or unparseable one stops the server before it tries to listen, which
looks exactly like a port failure and sends people hunting the wrong thing —
`[FATAL] No access secret provided` is the line that says so.

To set a new one without touching anything else:

```
gobbonet set-password
```

To start the whole first-run flow again, including the password:

```
gobbonet setup --force
```

⚠ `.gobbonet-secret` in the install folder was the batch path's, and nothing
here reads it. If you have one, it is left over from an older GobboNet; the
uninstaller removes it now, and deleting it by hand is safe and changes
nothing.

---

## The model will not come back after a swap

`/health` answers `ok` only when llama-server is idle and loaded. While it is
loading, or busy with the reply you are reading, it answers something else, and
treating that as death restarts a perfectly good server forever. The supervisor
separates three states — healthy, running but not ready, and not running — and
only the last justifies a restart.

Watch it rather than guess: the console prints `[swap] launching:` with the full
command line, then `[swap] active model is now …`. `gobbonet doctor` reports
whether the engine is found and what it will offload to.

⚠ **`stop-gobbonet.bat` is a blunt instrument.** It kills every
`llama-server.exe` by image name, and on this build the retrieval model is a
second `llama-server.exe` — so it stops embeddings too, silently disabling
retrieval until GobboNet is restarted. Prefer Ctrl+C in the GobboNet window,
which stops both cleanly.

---

## "llama-server already running" but nothing works

If you have **Ollama** installed, older versions of GobboNet mistook it for
llama.cpp. Both used port 11434, and the launcher accepted any HTTP answer
as proof its own server was up -- including Ollama's 404. It then skipped
starting llama.cpp, found nothing healthy, and restarted into a port Ollama
already owned.

Fixed in 1.5.8 two ways: llama.cpp now defaults to **11437**, and the
launcher requires a 200 with the expected body before believing a service
is its own.

If you still see a collision, set the ports explicitly before launching:

```
gobbonet config set llm_url http://127.0.0.1:11437
gobbonet config set listen_port 9066
gobbonet
```

---

## The model will not load

GobboNet reports the failure through `/swap-status`, and `doctor` names `llama-server.log`. Two common causes:

- **Not enough VRAM.** Pick a smaller model or a heavier quantisation.
- **Stale server.** Closing the window without stopping the servers can
  leave `llama-server.exe` holding the port. Check with
  `netstat -ano | findstr "11437 11436 9066"` and end those PIDs.

If a model downloaded but never loads, check for a leftover `.part` file in
`models\` — that is an aborted download and is safe to delete.

---

## Windows Defender, and how to stop it interfering

GobboNet has a shape Defender does not like: it downloads a multi-gigabyte
file, opens several local listening ports, and runs PowerShell out of a
folder in your user profile. Every one of those is normal here and every one
of them is also what a lot of malware does. Defender judges the shape, not
the intent, so it sometimes acts.

The most disruptive version of this is not a warning at all — it is a
scheduled scan quarantining a file overnight while you are away from the PC.
You come back to a model that will not load, or a server that exits before it
can log anything, with nothing on screen explaining why.

**Excluding the folder prevents that:**

```
Windows Security  >  Virus & threat protection  >  Manage settings  >
Exclusions  >  Add an exclusion  >  Folder  >  pick your GobboNet folder
```

The default folder is:

```
%LOCALAPPDATA%\GobboNet
```

The installer shows this on its own screen before first launch, so if you
skipped past it, this is that.

### Other symptoms worth knowing

- **SmartScreen: "Windows protected your PC"** on the installer. It is
  unsigned — **More info → Run anyway**. If a `.sha256` was published beside the
  download, check the file against it; this fork does not publish releases, so
  there may not be one.
- **`gobbonet.exe` does nothing at all when run.** Antivirus quarantine.
  Check your antivirus protection history first — a quarantined file is listed
  there with a timestamp. (The old AppLocker/WDAC script-policy cause went with
  the PowerShell path; a Go binary is not a script.)
- **A model that loaded yesterday will not load today.** Check the models
  folder still contains the `.gguf`. A quarantined file disappears silently.

None of this requires turning Defender off, and you should not. A folder
exclusion is narrower and reversible.

---

## Linux / Wine

Run it natively. The server is Go and cross-compiles for Linux, and there are
Debian and Fedora packages — Wine was only ever a workaround for the PowerShell
server, which this fork no longer has. The one Windows-only piece left is the
hardware probe the installer runs.

---

## Still stuck

Include these in a bug report and it can usually be answered in one reply:

1. `gobbonet doctor` (whole output)
2. `llama-server.log`, and `startup-error.log` if the server never came up
3. Whether the chat page itself loaded — that proves the listener bound, which
   rules out a whole class of theories
4. Output of `netsh interface ipv4 show excludedportrange protocol=tcp`
