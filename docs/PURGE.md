# Removing your GobboNet data

Conversations live in more than one place, and only some of them can be
reached by the app. This is the complete list, what clears each, and what
nothing can clear.

---

## Where the data actually is

| Location | What is there | Cleared by |
|---|---|---|
| Browser storage on this PC | every thread, character, persona, macro, plus cached embeddings and retrieval telemetry | **Data → PURGE ALL** in the app |
| `state.json` in the **data** folder | a mirror of the above, so a reload never loses a thread | `gobbonet uninstall` |
| `state-<name>.json` in the **data** folder | a separate device-sync profile's mirror (DATA → DEVICE SYNC → Separate profile) | the delete button in DEVICE SYNC, or `gobbonet uninstall` |
| `state.keyring` in the **data** folder | the key that decrypts an encrypted history (`gobbonet keyring init`) | `gobbonet decrypt`, or `gobbonet uninstall` |
| `.state.lock` in the **data** folder | an empty lock file the running server holds; no data | `gobbonet uninstall` |
| `.jobs\` in the **data** folder | generation spool; the Go server keeps jobs in memory and only sweeps this | `gobbonet uninstall` |
| `.gobbonet-state.json`, `.gobbonet-secret`, `.jobs\` in the **install** folder | an older GobboNet's transcripts, password hash and spool — the batch path kept them here | the uninstaller, from this version on |
| Browser storage on a **phone or tablet** that connected | a full copy, held by that device | only that device — or a purge or delete on another synced device |

The important one: **browser storage is keyed to the exact address you
opened**, so `127.0.0.1:9066`, `localhost:9066`, `pcname.local:9066` and a
LAN IP are four separate buckets. Clearing one does not touch the others.

---

## Clearing this PC

### 1. In the app — the fast path

**Data → PURGE ALL.** This deletes threads, characters, personas,
schedules, folders, macros, extensions, cached embeddings and retrieval
telemetry from this browser, then resets to a clean install.

It reports anything it could not clear. If you see a list, close any other
GobboNet tabs and run it again — a second tab holding the database open
blocks deletion.

> Before 1.5.9 this button did **not** actually delete stored
> conversations. It reset what was on screen and left every record in
> place, so history could reappear on reload. If you purged on an earlier
> version, purge again on 1.5.9 or clear site data as below.

PURGE ALL also deletes your conversations from the server-side mirror, one at a
time: every chat this browser has synced is deleted on the server if the server
still holds the version this browser last saw. A chat another device has added
to since is kept, and comes back here on the next sync. The characters, personas
and settings on the server are reset to the defaults this browser now holds.

⚠ **It reaches your other devices too.** Once a chat is gone from the server,
every other device that synced it and has not changed it since removes it as
well, the next time it checks in (opening a chat, or coming back to the tab).
Export first if another device should keep a copy. A device with sync switched
off (DATA → DEVICE SYNC → Off) keeps its own copy until you clear it there.

⚠ **Wait for the sync indicator to confirm, then reload, and only then trust
it.** If the deletes do not land — the server was down, you closed the tab too
quickly, or this browser had never synced with the server — the next reload sees
empty local storage next to a server that still has data, and **restores it
silently, with no prompt**. If the indicator never confirms, do step 2 by hand.

### 2. The server-side mirror

⚠ **These do not live in the install folder**, so deleting that folder, or
running the Windows uninstaller and leaving the settings box unticked, leaves
them behind. They are in the data folder, which is separate on purpose: a
reinstall is meant to keep your conversations.

`gobbonet uninstall` is the command that clears them, and the Windows
uninstaller's **settings** checkbox is what calls it. To do it by hand instead,
stop GobboNet and delete:

```powershell
$d = "$env:USERPROFILE\.local\share\gobbonet"
Remove-Item "$d\state.json", "$d\state.json.bak", "$d\state.keyring", "$d\.state.lock" -Force -ErrorAction SilentlyContinue
Remove-Item "$d\state-*.json*" -Force -ErrorAction SilentlyContinue
Remove-Item "$d\.jobs" -Recurse -Force -ErrorAction SilentlyContinue
```

Profiles (`state-*.json`) and, on an encrypted install, `state.keyring` sit
beside `state.json`; delete them the same way, with GobboNet stopped. Without
`state.keyring` an encrypted history can never be read again.

`gobbonet doctor` prints the real data folder under **CONFIG** if you moved it.
To find every copy on the machine, including old test installs:

```powershell
Get-ChildItem $env:USERPROFILE -Recurse -Force -ErrorAction SilentlyContinue `
  -Include "state.json", "state-*.json", "state.keyring", ".gobbonet-state.json", ".gobbonet-secret", ".jobs" |
  Where-Object FullName -like "*obbo*" |
  Select-Object FullName, Length, LastWriteTime
```

⚠ **If you upgraded from a version that used `launch.bat`,** look in the install
folder too — the default below, or wherever you unpacked `launch.bat` if you
used a zip rather than the installer. That path kept the state mirror, the spool and the password hash
there, nothing since has read them, and until this version no uninstaller
removed them — so they can outlive an uninstall:

```powershell
Remove-Item "$env:LOCALAPPDATA\GobboNet\.gobbonet-state.json" -Force -ErrorAction SilentlyContinue
Remove-Item "$env:LOCALAPPDATA\GobboNet\.gobbonet-secret"     -Force -ErrorAction SilentlyContinue
Remove-Item "$env:LOCALAPPDATA\GobboNet\.jobs" -Recurse -Force -ErrorAction SilentlyContinue
```

### 3. Browser storage, the thorough way

An uninstaller is a native program; the data sits inside a browser profile
and no installer can reach in. Clear it yourself:

- **Chrome / Edge** — open the chat, press F12 → Application → Storage →
  **Clear site data**. Or Settings → Privacy → third-party cookies → See all
  site data → search the port number → delete.
- **Firefox** — Settings → Privacy → Cookies and Site Data → Manage Data →
  search the address → Remove.

**Do this for every address you used.** Each is a separate bucket:
`127.0.0.1:9066`, `localhost:9066`, your PC's `.local` name, and any LAN IP.
If you changed the port at some point, the old port is a separate bucket
again.

---

## Clearing a phone or tablet

Whatever a device downloaded, it kept. Nothing on the PC can push to it.
A device with sync on does remove a chat deleted elsewhere, but only when it
next checks in, and only a chat it has not changed since; one with sync off,
or one that never reconnects, keeps everything — and at uninstall time the
server is being torn down anyway.

On the device itself, open the same address and clear site data through the
mobile browser's settings. On iOS Safari: Settings → Safari → Advanced →
Website Data → find the address → swipe to delete.

If you cannot get the device back, that copy stays where it is. That is a
real limit, not an oversight — see below.

---

## What cannot be cleared

**A device that connected once and never came back.** A phone that used the
chat over your LAN holds a full copy in its own browser storage. There is no
mechanism on the PC side that reaches it. Uninstalling GobboNet does not
change that, and no future version can, because the only channel that ever
existed was the phone asking the server for something.

If you are handling material where that matters, the practical answer is to
clear each device while you still have it, before uninstalling.

---

## Verifying

After purging and clearing site data, reload the chat. An empty sidebar and
a default character mean this browser is clean. To be sure nothing is left
on disk:

```powershell
Get-ChildItem "$env:LOCALAPPDATA\GobboNet" -Force |
  Where-Object { $_.Name -like ".gobbonet*" -or $_.Name -like "*.log" -or $_.Name -eq ".jobs" }
Get-ChildItem "$env:USERPROFILE\.local\share\gobbonet" -Force -ErrorAction SilentlyContinue
```

Two folders, because conversations live in the second one. The first is the
install folder and only holds anything if you upgraded from a `launch.bat`
version — the `.jobs` term is there because the old filter matched only files
and would have shown that directory's name but never flagged it.

An empty result from both means no conversation data is left on disk.
