; ================================================================
; GobboNet installer -- Go server edition
;
; Goal: the user goes from "downloaded the setup exe" to "chatting"
; without answering a question at a prompt. Everything the old
; launch.bat asked at a C:\> prompt is now a wizard page, and the
; finish page's "Start GobboNet" checkbox lands on a working chat.
;
; It does NOT mean no console window. gobbonet.exe is built for the
; console subsystem -- no -H windowsgui -- so starting it opens a
; window that prints the banner and stays for the life of the server,
; and closing that window stops it (see killjob_windows.go). That is
; upstream's shape for the Go server on every platform; changing it
; needs a log file for the banner first, or the startup warnings go
; nowhere.
;
; WHAT IS BUNDLED vs DOWNLOADED
;   bundled:     gobbonet.exe, web assets, llama.cpp, the .ps1 helpers
;   downloaded:  the GGUF model, and only the GGUF model
;
; That split is deliberate. launch.bat documents at length that
; "cmd -> temp .ps1 with Bypass -> downloads an executable archive"
; is the shape behavioral AV reads as malware staging, and that it
; kills the process tree with no error text. An unsigned installer
; fetching a zip full of .exe files is the same shape with a worse
; parent process, so llama.cpp ships inside the installer instead.
; A .gguf is inert data and carries no such signature, so that one
; download stays online -- it is also the only file too large to
; bundle.
;
; The PowerShell we do run (hardware-probe.ps1) only reads WMI,
; the registry and nvidia-smi. It downloads nothing, so it is not
; the pattern launch.bat removed -- and launch.bat already invokes
; it exactly this way.
;
; BUILD: ../installer/build-installer.sh  (do not call makensis directly;
;        the payload staging and version stamp happen there)
; ================================================================

Unicode true

; Must come before anything that emits data (ReserveFile, File, plugins).
; NSIS refuses to change compressor once the header has been touched.
SetCompressor /SOLID lzma

!include "MUI2.nsh"
!include "LogicLib.nsh"
!include "nsDialogs.nsh"
!include "FileFunc.nsh"
!include "WinMessages.nsh"

; INetC ships in-tree so the build does not depend on what happens to be
; installed in the system NSIS plugin folder. x86-unicode is the correct
; variant: this is a Unicode installer, and the 1.3 build was too.

; FileFunc's ${GetSize} has to be instantiated before use.
!insertmacro GetSize

; models.ini is read in .onInit, before any section runs. With SetCompressor
; /SOLID the whole archive would otherwise have to be decompressed to reach
; it; ReserveFile puts it first in the data block instead.

;-------------------------------------------------------------------
; Build-time inputs. build-installer.sh passes these with -D.
;-------------------------------------------------------------------
!ifndef VERSION
  !error "VERSION not defined -- build via build-installer.sh"
!endif
!ifndef PAYLOAD
  !error "PAYLOAD not defined -- build via build-installer.sh"
!endif

Name "GobboNet"
OutFile "GobboNetSetup-${VERSION}.exe"
InstallDir "$LOCALAPPDATA\GobboNet"
InstallDirRegKey HKCU "Software\GobboNet" "InstallDir"

; Per-user install. The 1.3 installer made the same call and its welcome
; page advertises it: "Installs to your user folder. No administrator
; rights required." Asking for elevation here would be a regression, and
; a per-user install is also what keeps the LAN step opt-in.
RequestExecutionLevel user


VIProductVersion "${VERSION_QUAD}"
VIAddVersionKey "ProductName"     "GobboNet"
VIAddVersionKey "FileDescription" "GobboNet Installer"
VIAddVersionKey "FileVersion"     "${VERSION}"
VIAddVersionKey "ProductVersion"  "${VERSION}"
VIAddVersionKey "CompanyName"     "Elodine"
VIAddVersionKey "LegalCopyright"  "Elodine / GoblinCorps -- free to use, copy and modify"

;-------------------------------------------------------------------
; State
;-------------------------------------------------------------------
Var HwIni            ; path to the probe's flat INI
Var HwVram
Var HwRam
Var HwDiskFree
Var HwTier
Var HwGpuName
Var HwProbed        ; "1" once the probe has run

; page control handles
Var Dlg
Var Lbl
Var ChkStart
Var SetupDone        ; "1" when `setup --status` says a previous setup survived

; Uninstaller options page: the controls, then the answers read off them.
Var UnDataChk
Var UnModelsChk
Var UnLanChk
Var UnRemoveData
Var UnRemoveModels
Var UnRemoveLan

;-------------------------------------------------------------------
; MUI look. Reuses the 1.3 artwork so the wizard still reads as
; Elodine's, not as a generic NSIS default.
;-------------------------------------------------------------------
!define MUI_ABORTWARNING
; Compiled into the exe's resources, so it comes from art/ at build time.
; The payload also carries a copy, but that one is for the shortcuts to
; point at after install.
!define MUI_ICON   "art\gobbonet.ico"
!define MUI_UNICON "art\gobbonet.ico"
!define MUI_HEADERIMAGE
!define MUI_HEADERIMAGE_BITMAP "art\modern-header.bmp"
!define MUI_WELCOMEFINISHPAGE_BITMAP "art\modern-wizard.bmp"

!define MUI_WELCOMEPAGE_TITLE "GobboNet ${VERSION}"
!define MUI_WELCOMEPAGE_TEXT \
"Local chat for local models. No account, no API key, no telemetry, no corpo middleman. What you type stays on the machine you type it on.$\r$\n$\r$\n\
This installer carries llama.cpp with it and downloads nothing. Setup finishes in your browser, where you choose a password, where the AI runs, and which model to fetch.$\r$\n$\r$\n\
Installs to your user folder. No administrator rights required."

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY

Page custom ProbePageCreate   ProbePageLeave

!insertmacro MUI_PAGE_INSTFILES
Page custom FinishPageCreate  FinishPageLeave

UninstPage custom un.OptionsPageCreate un.OptionsPageLeave
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "English"

;===================================================================
; Helpers
;===================================================================

; Run a command, streaming its output into the details pane, and abort
; the install if it exits non-zero. Every external call in this script
; goes through here so that a failure stops the install instead of
; leaving a half-configured folder that looks installed.
!macro RunChecked cmd what
  DetailPrint "${what}"
  nsExec::ExecToLog '${cmd}'
  Pop $0
  ${If} $0 != 0
    DetailPrint "  [ERROR] ${what} failed (exit $0)"
    Abort "${what} failed with exit code $0. See the details pane above."
  ${EndIf}
!macroend

;-------------------------------------------------------------------
; Read a hex SHA-256 out of certutil's output file.
; certutil prints:
;   line 1: "SHA256 hash of file X:"
;   line 2: the hash (older builds space-separate the bytes)
;   line 3: "CertUtil: -hashfile command completed successfully."
; Stack: (out) hash-or-empty
;-------------------------------------------------------------------

;===================================================================
; PAGE 1 -- backend: this machine, or a server elsewhere
;===================================================================

;===================================================================
; PAGE 2 -- hardware probe (local only)
;
; The probe is run from a timer rather than inline so the page paints
; its "checking..." text first. dxdiag's fallback path can take ~10s
; and a frozen blank wizard reads as a crash.
;===================================================================
Function ProbePageCreate
  !insertmacro MUI_HEADER_TEXT "Checking your hardware" \
    "So you know what this machine can run before the wizard offers you a model."

  nsDialogs::Create 1018
  Pop $Dlg
  ${If} $Dlg == error
    Abort
  ${EndIf}

  ${NSD_CreateLabel} 0 0u 100% 40u "Detecting GPU, memory and free disk space..."
  Pop $Lbl

  ${If} $HwProbed != "1"
    GetDlgItem $0 $HWNDPARENT 1
    EnableWindow $0 0                       ; disable Next during the probe
    ${NSD_CreateTimer} RunProbe 200
  ${EndIf}

  nsDialogs::Show
FunctionEnd

Function RunProbe
  ${NSD_KillTimer} RunProbe

  StrCpy $HwIni "$PLUGINSDIR\hardware.ini"

  ; -Quiet keeps the console chatter out; we read the INI for the numbers.
  ; Not fatal on failure: a probe that cannot see the GPU should leave the
  ; user with an unfiltered catalogue, not a dead installer.
  ;
  ; Everything here lives in $PLUGINSDIR, not $INSTDIR: this page runs before
  ; the install section, so $INSTDIR holds nothing yet and may not even exist.
  ; SecMain copies hardware.json across afterwards so launch.bat inherits the
  ; probe rather than re-running it.
  nsExec::ExecToLog '"$SYSDIR\WindowsPowerShell\v1.0\powershell.exe" -NoProfile \
    -ExecutionPolicy Bypass -File "$PLUGINSDIR\hardware-probe.ps1" \
    -OutputPath "$PLUGINSDIR\hardware.json" -IniPath "$HwIni" \
    -ModelsDir "$INSTDIR\models" -Quiet'
  Pop $0

  StrCpy $HwProbed "1"
  StrCpy $HwVram 0
  StrCpy $HwRam 0
  StrCpy $HwDiskFree 0
  StrCpy $HwTier "unknown"
  StrCpy $HwGpuName "unknown"

  ${If} $0 == 0
    ReadINIStr $HwGpuName  "$HwIni" "hardware" "gpu_name"
    ReadINIStr $HwVram     "$HwIni" "hardware" "vram_gb"
    ReadINIStr $HwRam      "$HwIni" "hardware" "ram_gb"
    ReadINIStr $HwDiskFree "$HwIni" "hardware" "disk_free_gb"
    ReadINIStr $HwTier     "$HwIni" "hardware" "recommended_tier"
    ${NSD_SetText} $Lbl "GPU:  $HwGpuName$\r$\nVRAM: $HwVram GB$\r$\n\
RAM:  $HwRam GB$\r$\nFree disk: $HwDiskFree GB$\r$\n$\r$\nSuggested tier: $HwTier"
  ${Else}
    ${NSD_SetText} $Lbl "Could not read this machine's hardware.$\r$\n$\r$\n\
Setup will still offer every model. Pick one that fits your GPU."
  ${EndIf}

  GetDlgItem $0 $HWNDPARENT 1
  EnableWindow $0 1
FunctionEnd

; No skip check here: when the create function Aborts, the page is never
; shown and NSIS never calls its leave function.
Function ProbePageLeave
FunctionEnd

;===================================================================
; PAGE 3 -- model catalogue
;
; The list, the sizes and the recommendation all come from models.ini,
; hand-maintained since launch.bat was retired. Nothing about the
; catalogue is written twice.
;===================================================================

;===================================================================
; INSTALL
;===================================================================
Section "GobboNet" SecMain
  SetOutPath "$INSTDIR"
  SetOverwrite on

  ; An upgrade over a running install cannot overwrite gobbonet.exe while it
  ; is running -- NSIS reports a write failure on a file the user can plainly
  ; see, which reads as a corrupt download. Extracted to $PLUGINSDIR in
  ; .onInit because on a FIRST install $INSTDIR has no copy yet.
  DetailPrint "Stopping any running GobboNet..."
  stop_retry:
  nsExec::ExecToLog '"$SYSDIR\cmd.exe" /c ""$PLUGINSDIR\stop-gobbonet.bat" /quiet"'
  Pop $0
  ${If} $0 != 0
    ; Say what is wrong while it can still be fixed. Letting the File command
    ; below hit a locked gobbonet.exe instead produces "could not write to
    ; file", which names the symptom and not the cause.
    MessageBox MB_RETRYCANCEL|MB_ICONEXCLAMATION \
      "GobboNet is still running and could not be stopped automatically.$\r$\n$\r$\n\
Close the GobboNet window (and any llama-server window), then choose Retry.$\r$\n$\r$\n\
If it will not close, a reboot always clears it." \
      IDRETRY stop_retry
    Abort "Setup stopped: GobboNet is still running."
  ${EndIf}

  DetailPrint "Installing GobboNet ${VERSION}..."
  File "${PAYLOAD}\gobbonet.exe"
  File "${PAYLOAD}\gobbonet.ico"
  File /r "${PAYLOAD}\web"

  ; The PowerShell helpers stay: launch.bat still uses them for adding
  ; further models. The probe page ran its own copy out of $PLUGINSDIR
  ; (see .onInit) because this section had not executed yet.
  File "${PAYLOAD}\setup-lan.bat"
  ; teardown-lan.bat is the counterpart to setup-lan.bat, and it has to be
  ; installed even for users who never run the LAN setup: the uninstaller
  ; calls it, and it is the only thing that clears a URL reservation.
  File "${PAYLOAD}\teardown-lan.bat"
  File "${PAYLOAD}\stop-gobbonet.bat"
  File "${PAYLOAD}\hardware-probe.ps1"

  ; The running server needs this too, not just the installer.
  ;
  ; .onInit extracts a copy into $PLUGINSDIR for the model page, and NSIS
  ; deletes $PLUGINSDIR when the installer exits -- so for three releases
  ; nothing named models.ini survived the install. catalog.Discover() looks
  ; beside the exe, found nothing, and the settings panel's Add a Model modal
  ; had no fallback list when the remote catalogue was unreachable. On Windows
  ; that meant a 503 and an empty modal.
  ;
  ; Compile-time source, the same one ReserveFile names, rather than routing
  ; through $PAYLOAD: no reason for a second staging hop to be able to go stale.
  File "models.ini"

  ; Carry the probe result forward. $PLUGINSDIR is deleted when the
  ; installer exits, and launch.bat reads hardware.json from its own
  ; folder; without this the first launch re-probes for no reason. Silent
  ; and unconditional: a remote-backend install skipped the probe page
  ; entirely, and a missing source here is simply nothing to copy.
  CopyFiles /SILENT "$PLUGINSDIR\hardware.json" "$INSTDIR\hardware.json"

  DetailPrint "Installing bundled llama.cpp..."
  SetOutPath "$INSTDIR\llama-cpp"
  File /r "${PAYLOAD}\llama-cpp\*.*"

  CreateDirectory "$INSTDIR\models"
  SetOutPath "$INSTDIR"


  ;--------------------------------------------------------------
  ; Configuration
  ;
  ; Written through the CLI rather than by templating a .toml here,
  ; so the installer cannot drift from the server's own idea of what
  ; a valid config is.
  ;--------------------------------------------------------------
  DetailPrint "Writing configuration..."
  ; First install only. `setup --status` exits 0 once setup has completed, and on
  ; an upgrade this write would reset a model_dir the user moved in the settings
  ; panel.
  nsExec::Exec '"$INSTDIR\gobbonet.exe" setup --status'
  Pop $0
  ${If} $0 == 0
    StrCpy $SetupDone "1"
  ${Else}
    StrCpy $SetupDone "0"
  ${EndIf}
  ${If} $0 != 0
    !insertmacro RunChecked '"$INSTDIR\gobbonet.exe" config set model_dir "$INSTDIR\models"' \
                            "Setting model_dir"
  ${Else}
    DetailPrint "  Existing setup found; leaving model_dir alone."
  ${EndIf}

  ; server_exe is the wizard's alone, in both directions: it writes the bundled
  ; engine's path for local and clears it for remote. Touching it here broke
  ; upgrades -- clearing it over a completed local install left Mode() remote,
  ; serving against a default llm_url with nothing listening and no wizard,
  ; because the first-run guard sees an access_secret and stands down.

  ;--------------------------------------------------------------
  ; Shortcuts and uninstall metadata
  ;
  ; The 1.3 payload shipped launch.exe / launchLAN.exe: small C shims
  ; whose only job was to locate the folder and ShellExecute a .bat,
  ; complete with a hardcoded "usually an antivirus block" error path.
  ; gobbonet.exe is a real executable, so the shortcut points at it and
  ; both shims are gone.
  ;--------------------------------------------------------------
  CreateDirectory "$SMPROGRAMS\GobboNet"
  CreateShortcut "$SMPROGRAMS\GobboNet\GobboNet.lnk" \
                 "$INSTDIR\gobbonet.exe" "" "$INSTDIR\gobbonet.ico"
  CreateShortcut "$SMPROGRAMS\GobboNet\GobboNet LAN Setup.lnk" \
                 "$INSTDIR\setup-lan.bat" "" "$INSTDIR\gobbonet.ico"
  CreateShortcut "$SMPROGRAMS\GobboNet\Uninstall GobboNet.lnk" "$INSTDIR\uninstall.exe"
  CreateShortcut "$DESKTOP\GobboNet.lnk" \
                 "$INSTDIR\gobbonet.exe" "" "$INSTDIR\gobbonet.ico"

  WriteUninstaller "$INSTDIR\uninstall.exe"
  WriteRegStr HKCU "Software\GobboNet" "InstallDir" "$INSTDIR"

  !define UNINST_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\GobboNet"
  WriteRegStr HKCU "${UNINST_KEY}" "DisplayName"     "GobboNet"
  WriteRegStr HKCU "${UNINST_KEY}" "DisplayVersion"  "${VERSION}"
  WriteRegStr HKCU "${UNINST_KEY}" "DisplayIcon"     "$INSTDIR\gobbonet.ico"
  WriteRegStr HKCU "${UNINST_KEY}" "Publisher"       "Elodine"
  WriteRegStr HKCU "${UNINST_KEY}" "URLInfoAbout"    "https://github.com/ElodineOfficial"
  WriteRegStr HKCU "${UNINST_KEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "${UNINST_KEY}" "UninstallString" '"$INSTDIR\uninstall.exe"'
  WriteRegDWORD HKCU "${UNINST_KEY}" "NoModify" 1
  WriteRegDWORD HKCU "${UNINST_KEY}" "NoRepair" 1

  ${GetSize} "$INSTDIR" "/S=0K" $0 $1 $2
  IntFmt $0 "0x%08X" $0
  WriteRegDWORD HKCU "${UNINST_KEY}" "EstimatedSize" "$0"
SectionEnd

;===================================================================
; FINISH -- the checkbox this whole exercise is for
;===================================================================
Function FinishPageCreate
  !insertmacro MUI_HEADER_TEXT "Done" "GobboNet is installed and configured."

  nsDialogs::Create 1018
  Pop $Dlg
  ${If} $Dlg == error
    Abort
  ${EndIf}

  ${If} $SetupDone == "1"
    StrCpy $1 "Installed. Your existing settings were kept, so there is nothing to set \
up -- starting GobboNet goes straight to the chat."
  ${Else}
    StrCpy $1 "Installed. The first start finishes setup in your browser -- a password, \
where the AI runs, and a model to download. After that it just serves the chat."
  ${EndIf}

  ${NSD_CreateLabel} 0 0u 100% 40u "$1$\r$\n$\r$\n\
Everything in the install folder is plain text you can read, break and rebuild. \
That is the point."
  Pop $Lbl

  ${NSD_CreateCheckbox} 0 48u 100% 12u "Start GobboNet"
  Pop $ChkStart
  ${NSD_Check} $ChkStart

  ; No LAN checkbox here. It only opened the firewall, while the web wizard
  ; asks the same question and writes listen_host -- so ticking one and taking
  ; the other's default left an open firewall in front of a loopback socket.
  ; Measured: the phone cannot connect and nothing explains it. The wizard owns
  ; both halves now, including switching them back off together.

  ; The Next button is the last one on this page.
  GetDlgItem $0 $HWNDPARENT 1
  SendMessage $0 ${WM_SETTEXT} 0 "STR:&Finish"

  nsDialogs::Show
FunctionEnd

Function FinishPageLeave
  ${NSD_GetState} $ChkStart $0
  ${If} $0 == ${BST_CHECKED}
    ; gobbonet.exe starts llama-server itself when server_exe is set,
    ; so this single call is the whole "installed -> running" step.
    Exec '"$INSTDIR\gobbonet.exe"'
  ${EndIf}
FunctionEnd

;===================================================================
Function .onInit
  InitPluginsDir
  ; The probe page runs BEFORE MUI_PAGE_INSTFILES, so nothing has been
  ; written to $INSTDIR yet when it fires. Extract the probe here, into
  ; the temp plugins dir, or RunProbe invokes a path that does not exist
  ; and every install silently takes the "could not read this machine's
  ; hardware" branch. Section SecMain still installs its own copy into
  ; $INSTDIR -- launch.bat calls it later, long after $PLUGINSDIR is gone.
  File /oname=$PLUGINSDIR\hardware-probe.ps1 "${PAYLOAD}\hardware-probe.ps1"
  ; Same reasoning: SecMain runs it before it has installed anything, so it
  ; cannot come from $INSTDIR on a first install.
  File /oname=$PLUGINSDIR\stop-gobbonet.bat "${PAYLOAD}\stop-gobbonet.bat"

  StrCpy $HwProbed "0"
  StrCpy $HwVram 0
  StrCpy $HwDiskFree 0
  StrCpy $HwTier "unknown"
FunctionEnd

;===================================================================
; Everything the uninstaller might remove, on one page.
;
; It was three sequential yes/no dialogs, which is the worst way to ask: no way
; to see the choices together, no way to revise one, and the explanation for
; each vanishes the moment it is answered. The browser caveat in particular was
; a dialog nobody could re-read.
Function un.OptionsPageCreate
  !insertmacro MUI_HEADER_TEXT "Uninstall GobboNet" \
    "The program is removed either way. Everything below is KEPT unless you tick it."

  nsDialogs::Create 1018
  Pop $Dlg
  ${If} $Dlg == error
    Abort
  ${EndIf}

  ${NSD_CreateCheckbox} 0 2u 100% 10u "Settings and this machine's copy of your conversations"
  Pop $UnDataChk
  ${NSD_CreateLabel} 12u 13u 96% 26u \
    "Your chats and characters are held by the BROWSER you opened the chat in, not \
on disk, and no uninstaller can reach those. To clear them too, delete site data \
for http://127.0.0.1:9066 in that browser. Until you do, a reinstall shows them all again."
  Pop $Lbl

  ${NSD_CreateCheckbox} 0 44u 100% 10u "Downloaded models, including the 146 MB retrieval model"
  Pop $UnModelsChk
  ${NSD_CreateLabel} 12u 55u 96% 18u \
    "Large files that would have to be downloaded again. Kept by default."
  Pop $Lbl

  ; Offered whenever the script is present, not only when setup-lan.bat ran.
  ; .gobbonet-lan was the old gate, and it tested the wrong thing: Windows writes
  ; its own rules for gobbonet.exe when the "Allow access?" prompt is answered at
  ; first launch, so a machine can be reachable with that marker absent. Left
  ; unchecked, so nobody is asked to elevate who does not tick it.
  ${If} ${FileExists} "$INSTDIR\teardown-lan.bat"
    ${NSD_CreateCheckbox} 0 78u 100% 10u "LAN access rules (needs Administrator approval)"
    Pop $UnLanChk
    ${NSD_CreateLabel} 12u 89u 96% 34u \
      "Firewall rules that let other devices reach GobboNet -- both the ones \
setup-lan.bat added and the ones Windows wrote itself when you answered its \
$\"Allow access?$\" prompt. Windows stores these, so they outlive an uninstall and \
apply again if you reinstall to the same folder."
    Pop $Lbl
  ${Else}
    StrCpy $UnLanChk ""
  ${EndIf}

  nsDialogs::Show
FunctionEnd

Function un.OptionsPageLeave
  ${NSD_GetState} $UnDataChk $UnRemoveData
  ${NSD_GetState} $UnModelsChk $UnRemoveModels
  ${If} $UnLanChk == ""
    StrCpy $UnRemoveLan 0
  ${Else}
    ${NSD_GetState} $UnLanChk $UnRemoveLan
  ${EndIf}
FunctionEnd

Section "Uninstall"
  ;--------------------------------------------------------------
  ; STOP EVERYTHING FIRST.
  ;
  ; This section used to go straight to Delete "$INSTDIR\gobbonet.exe" with
  ; nothing stopped. A running server holds its own binary and the install
  ; folder open, so the delete fails and Windows says "the folder is still
  ; open in another program" -- which reads as a stuck uninstall and leaves a
  ; server still holding the web port after the user believes it is gone.
  ;--------------------------------------------------------------
  DetailPrint "Stopping GobboNet..."
  ${If} ${FileExists} "$INSTDIR\stop-gobbonet.bat"
    un_stop_retry:
    nsExec::ExecToLog '"$SYSDIR\cmd.exe" /c ""$INSTDIR\stop-gobbonet.bat" /quiet"'
    Pop $0
    ${If} $0 != 0
      MessageBox MB_RETRYCANCEL|MB_ICONEXCLAMATION \
        "GobboNet is still running and could not be stopped automatically.$\r$\n$\r$\n\
Close it, then choose Retry. Continuing now would leave files behind and \
report that the folder is still open in another program." \
        IDRETRY un_stop_retry
      Abort "Uninstall stopped: GobboNet is still running."
    ${EndIf}
  ${Else}
    ; Upgrading from a build that predates the helper: do the two that matter
    ; inline. taskkill needs no elevation for this user's own processes.
    nsExec::ExecToLog '"$SYSDIR\taskkill.exe" /F /IM gobbonet.exe'
    Pop $0
    nsExec::ExecToLog '"$SYSDIR\taskkill.exe" /F /IM llama-server.exe'
    Pop $0
  ${EndIf}

  ;--------------------------------------------------------------
  ; LAN TEARDOWN -- the reason a broken port survives a reinstall.
  ;
  ; setup-lan.bat registers a machine-wide URL reservation with HTTP.SYS.
  ; That lives in the kernel, not in $INSTDIR, so deleting this folder does
  ; not touch it -- and a reservation with no listener behind it makes
  ; Windows answer the port with 503 on its own. That is why uninstalling,
  ; wiping the folder and reinstalling can all fail to fix it.
  ;
  ; It cannot be removed from here directly: this is a per-user install
  ; (RequestExecutionLevel user) and netsh needs Administrator. So the work
  ; is in teardown-lan.bat, and elevation is asked for only when the box is
  ; ticked -- it is unchecked by default, so a user with nothing to remove is
  ; never prompted.
  ;--------------------------------------------------------------
  ${If} ${FileExists} "$INSTDIR\teardown-lan.bat"
    ${If} $UnRemoveLan == 1
      ; Wait: the script lives in $INSTDIR and this section is about to delete it.
      ExecShellWait "runas" "$INSTDIR\teardown-lan.bat" "/quiet"
      ; The bind is the other half of the same decision. Removing the rules and
      ; leaving listen_host at 0.0.0.0 means the next install serves on the LAN
      ; with nothing allowing it, and Windows raises its own "Allow access?"
      ; dialog at first listen -- which is how untracked rules get written in
      ; the first place. Unelevated on purpose: this writes the config of the
      ; user being uninstalled, and the elevated script above may not be them.
      ; Harmless when settings are being removed too; that just deletes it.
      ${If} ${FileExists} "$INSTDIR\gobbonet.exe"
        nsExec::ExecToLog '"$INSTDIR\gobbonet.exe" config set listen_host 127.0.0.1'
        Pop $0
        DetailPrint "LAN access switched back off (listen_host 127.0.0.1)"
      ${EndIf}
    ${EndIf}
  ${EndIf}

  ;--------------------------------------------------------------
  ; THE USER'S OWN DATA.
  ;
  ; config.toml and state do NOT live under $INSTDIR. ConfigDir() and
  ; DataDir() are XDG paths on every platform, Windows included, so the real
  ; locations are %USERPROFILE%\.config\gobbonet and
  ; %USERPROFILE%\.local\share\gobbonet. Deleting the install folder has
  ; never touched either, which is why a reinstall kept finding the same
  ; broken settings -- including a server_exe pointing at a folder that no
  ; longer existed.
  ;
  ; gobbonet.exe already knows how to clear them properly (and how to ask
  ; about models), so this offers to run it rather than reimplementing the
  ; policy here -- and it has to run BEFORE the binary is deleted.
  ;--------------------------------------------------------------
  ; Models FIRST, and not gated on $INSTDIR\models.
  ;
  ; Both orderings were wrong. Removing settings first deletes the config that
  ; resolves model_dir, so the models call then fell back to the default data
  ; directory and cleared the retrieval model while leaving the chat models in
  ; $INSTDIR untouched. And gating on "$INSTDIR\models\*.gguf" second-guessed a
  ; path only the binary knows: with model_dir pointing anywhere else, ticking
  ; the box removed nothing at all.
  ${If} $UnRemoveModels == 1
    ${If} ${FileExists} "$INSTDIR\gobbonet.exe"
      nsExec::ExecToLog '"$INSTDIR\gobbonet.exe" uninstall --yes --models-only'
      Pop $0
      ${If} $0 != 0
        DetailPrint "  [WARN] model cleanup exited $0; removing $INSTDIR\models only."
        RMDir /r "$INSTDIR\models"
      ${EndIf}
    ${Else}
      RMDir /r "$INSTDIR\models"
    ${EndIf}
  ${EndIf}

  ${If} ${FileExists} "$INSTDIR\gobbonet.exe"
    ${If} $UnRemoveData == 1
      nsExec::ExecToLog '"$INSTDIR\gobbonet.exe" uninstall --yes --keep-models'
      Pop $0
      ${If} $0 != 0
        DetailPrint "  [WARN] could not clear user settings (exit $0)."
        DetailPrint "         Run: gobbonet uninstall"
      ${EndIf}
    ${EndIf}
  ${EndIf}


  Delete "$INSTDIR\gobbonet.exe"
  Delete "$INSTDIR\gobbonet.ico"
  Delete "$INSTDIR\*.bat"
  Delete "$INSTDIR\*.ps1"
  Delete "$INSTDIR\hardware.json"
  Delete "$INSTDIR\models.ini"
  Delete "$INSTDIR\uninstall.exe"
  ; Dotfiles: *.bat above does not match these, and a leftover .gobbonet-port
  ; would outlive the install and feed a stale port to the next setup-lan.bat.
  Delete "$INSTDIR\.gobbonet-port"
  Delete "$INSTDIR\.gobbonet-lan"

  ; Batch-era leftovers. fileserver.ps1 kept the state mirror, the job spool and
  ; the password hash in the INSTALL folder; the Go server uses the data folder
  ; and never reads any of them. Nothing removed them, and RMDir below is not
  ; recursive, so upgrading from a batch install and then uninstalling left
  ; conversations and a password hash sitting in $INSTDIR. This folder is the
  ; installer's to clear.
  Delete "$INSTDIR\.gobbonet-state.json"
  Delete "$INSTDIR\.gobbonet-secret"
  RMDir /r "$INSTDIR\.jobs"
  RMDir /r "$INSTDIR\web"
  RMDir /r "$INSTDIR\llama-cpp"
  RMDir "$INSTDIR"

  Delete "$SMPROGRAMS\GobboNet\*.lnk"
  RMDir  "$SMPROGRAMS\GobboNet"
  Delete "$DESKTOP\GobboNet.lnk"

  DeleteRegKey HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\GobboNet"
  DeleteRegKey HKCU "Software\GobboNet"
SectionEnd
