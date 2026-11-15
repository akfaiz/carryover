; NSIS script for the Windows installer. Built by scripts/windows-package.ps1,
; which passes the defines below; don't run makensis on it by hand unless you
; pass them too:
;
;   makensis /DVERSION=0.3.0 /DFILEVERSION=0.3.0.1 /DEXE=path\carryover.exe
;            /DOUTFILE=path\setup.exe carryover.nsi
;
; carryover.ico (next to this file) is the app icon from ui/assets/appicon.png;
; regenerate it when that icon changes.
;
; It installs for the current user only (no admin prompt) into
; %LOCALAPPDATA%\Programs\Carryover. Upgrades install over the same folder, so
; the exe keeps its path and Steam shortcuts made with "Add to Steam" keep
; working. Settings, library and rclone config live in %APPDATA%\Carryover and
; are never touched, also not by the uninstaller.
;
; Silent install: setup.exe /S [/D=C:\path\to\Carryover]  (/D must come last)

Unicode true
ManifestDPIAware true

!ifndef VERSION | FILEVERSION | EXE | OUTFILE
  !error "pass /DVERSION, /DFILEVERSION, /DEXE and /DOUTFILE (see scripts/windows-package.ps1)"
!endif

!define APP "Carryover"
!define APPEXE "carryover.exe"
!define UNINSTKEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP}"
!define SETTINGSKEY "Software\${APP}"

!include "MUI2.nsh"
!include "FileFunc.nsh"

Name "${APP}"
OutFile "${OUTFILE}"
InstallDir "$LOCALAPPDATA\Programs\${APP}"
InstallDirRegKey HKCU "${SETTINGSKEY}" "InstallDir"
RequestExecutionLevel user
SetCompressor /SOLID lzma
BrandingText "${APP} ${VERSION}"

VIProductVersion "${FILEVERSION}"
VIAddVersionKey "ProductName" "${APP}"
VIAddVersionKey "FileDescription" "${APP} Setup"
VIAddVersionKey "FileVersion" "${VERSION}"
VIAddVersionKey "ProductVersion" "${VERSION}"
VIAddVersionKey "LegalCopyright" "Copyright (c) 2026 Ahmad Faiz Kamaludin"

!define MUI_ICON "${__FILEDIR__}\carryover.ico"
!define MUI_UNICON "${__FILEDIR__}\carryover.ico"
!define MUI_ABORTWARNING
!define MUI_COMPONENTSPAGE_NODESC
!define MUI_FINISHPAGE_RUN "$INSTDIR\${APPEXE}"
!define MUI_UNCONFIRMPAGE_TEXT_TOP "Your settings, library and cloud configuration in $APPDATA\${APP} are kept. Delete that folder yourself to remove them."

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_COMPONENTS
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "English"

; The exe can't be replaced while Carryover, a game started through
; "carryover run", or a session watcher is still running from it.
!macro RemoveExe un
  Function ${un}RemoveExe
    retry:
      ClearErrors
      Delete "$INSTDIR\${APPEXE}"
      IfErrors 0 done
      IfFileExists "$INSTDIR\${APPEXE}" 0 done
      MessageBox MB_RETRYCANCEL|MB_ICONEXCLAMATION \
        "${APP} is still running (or a game started from it is). Close it, then click Retry." \
        /SD IDCANCEL IDRETRY retry
      Abort
    done:
  FunctionEnd
!macroend
!insertmacro RemoveExe ""
!insertmacro RemoveExe "un."

Section "${APP}" SecApp
  SectionIn RO
  SetOutPath "$INSTDIR"
  Call RemoveExe
  File "/oname=${APPEXE}" "${EXE}"
  WriteUninstaller "$INSTDIR\uninstall.exe"

  CreateShortcut "$SMPROGRAMS\${APP}.lnk" "$INSTDIR\${APPEXE}"

  WriteRegStr HKCU "${SETTINGSKEY}" "InstallDir" "$INSTDIR"
  WriteRegStr HKCU "${UNINSTKEY}" "DisplayName" "${APP}"
  WriteRegStr HKCU "${UNINSTKEY}" "DisplayVersion" "${VERSION}"
  WriteRegStr HKCU "${UNINSTKEY}" "Publisher" "Ahmad Faiz Kamaludin"
  WriteRegStr HKCU "${UNINSTKEY}" "URLInfoAbout" "https://github.com/akfaiz/carryover"
  WriteRegStr HKCU "${UNINSTKEY}" "DisplayIcon" "$INSTDIR\${APPEXE}"
  WriteRegStr HKCU "${UNINSTKEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "${UNINSTKEY}" "UninstallString" '"$INSTDIR\uninstall.exe"'
  WriteRegStr HKCU "${UNINSTKEY}" "QuietUninstallString" '"$INSTDIR\uninstall.exe" /S'
  WriteRegDWORD HKCU "${UNINSTKEY}" "NoModify" 1
  WriteRegDWORD HKCU "${UNINSTKEY}" "NoRepair" 1
  ${GetSize} "$INSTDIR" "/S=0K" $0 $1 $2
  IntFmt $0 "0x%08X" $0
  WriteRegDWORD HKCU "${UNINSTKEY}" "EstimatedSize" "$0"
SectionEnd

Section "Desktop shortcut" SecDesktop
  CreateShortcut "$DESKTOP\${APP}.lnk" "$INSTDIR\${APPEXE}"
SectionEnd

Function .onInit
  ; An upgrade keeps the desktop shortcut choice made at the first install.
  IfFileExists "$INSTDIR\${APPEXE}" 0 done
  IfFileExists "$DESKTOP\${APP}.lnk" done
  SectionSetFlags ${SecDesktop} 0
  done:
FunctionEnd

Section "Uninstall"
  Call un.RemoveExe
  Delete "$INSTDIR\uninstall.exe"
  RMDir "$INSTDIR" ; only if empty: never delete files we didn't put there

  Delete "$SMPROGRAMS\${APP}.lnk"
  Delete "$DESKTOP\${APP}.lnk"

  DeleteRegKey HKCU "${UNINSTKEY}"
  DeleteRegKey HKCU "${SETTINGSKEY}"
SectionEnd
