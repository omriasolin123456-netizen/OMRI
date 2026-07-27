#NoEnv
#SingleInstance Force
; =============================================================================
; Microinvest Склад Pro — помощник заполнения карточек автозапчастей
; AutoHotkey UNICODE 1.1+ (AutoHotkeyU64.exe) / Windows 11 x64
; Файл ДОЛЖЕН быть в UTF-8 с BOM — иначе кириллица отображается «иероглифами».
; =============================================================================
FileEncoding, UTF-8
SendMode Input
SetWorkingDir %A_ScriptDir%
SetTitleMatchMode, 2
CoordMode, Mouse, Screen

global g_ConfigPath := A_ScriptDir . "\config.ini"
global g_Hotkey := "F8"
global g_WindowTitle := "Microinvest"
global g_FieldName := "WindowsForms10.EDIT.app.0.19bf6b8_r8_ad110"
global g_FieldBarcode := "WindowsForms10.EDIT.app.0.19bf6b8_r8_ad19"
global g_FieldNtin := "WindowsForms10.EDIT.app.0.19bf6b8_r8_ad11"
global g_Python := "python"
global g_Timeout := 120000
global g_ResultFile := A_ScriptDir . "\logs\last_result.json"
global g_ResultTxt := A_ScriptDir . "\logs\last_result.txt"
global g_LogDir := A_ScriptDir . "\logs"
global g_ResolvedNameNN := ""
global g_ResolvedWinId := 0

LoadConfig()
RegisterHotkey()
return

LoadConfig() {
    global
    if !FileExist(g_ConfigPath) {
        MsgBox, 48, Microinvest Assistant, Файл config.ini не найден:`n%g_ConfigPath%
        return
    }
    IniRead, g_Hotkey, %g_ConfigPath%, hotkey, key, F8
    IniRead, g_WindowTitle, %g_ConfigPath%, microinvest, window_title, Microinvest
    IniRead, g_FieldName, %g_ConfigPath%, microinvest, field_name, %g_FieldName%
    IniRead, g_FieldBarcode, %g_ConfigPath%, microinvest, field_barcode, %g_FieldBarcode%
    IniRead, g_FieldNtin, %g_ConfigPath%, microinvest, field_ntin, %g_FieldNtin%
    IniRead, g_Python, %g_ConfigPath%, python, executable, python
    IniRead, g_Timeout, %g_ConfigPath%, python, timeout_ms, 120000
    IniRead, resultRel, %g_ConfigPath%, paths, result_file, logs\last_result.json
    IniRead, logRel, %g_ConfigPath%, paths, log_dir, logs

    if (SubStr(resultRel, 1, 1) = "\" || SubStr(resultRel, 2, 1) = ":")
        g_ResultFile := resultRel
    else
        g_ResultFile := A_ScriptDir . "\" . resultRel

    SplitPath, g_ResultFile, , resultDir
    g_ResultTxt := resultDir . "\last_result.txt"

    if (SubStr(logRel, 1, 1) = "\" || SubStr(logRel, 2, 1) = ":")
        g_LogDir := logRel
    else
        g_LogDir := A_ScriptDir . "\" . logRel

    if !FileExist(g_LogDir)
        FileCreateDir, %g_LogDir%
}

RegisterHotkey() {
    global g_Hotkey
    Hotkey, IfWinActive
    try {
        Hotkey, %g_Hotkey%, DoAssist, On
    } catch e {
        MsgBox, 16, Microinvest Assistant, Не удалось зарегистрировать горячую клавишу: %g_Hotkey%
    }
    Menu, Tray, Tip, Microinvest Parts Assistant (%g_Hotkey%)
    Menu, Tray, NoStandard
    Menu, Tray, Add, Запустить поиск, DoAssist
    Menu, Tray, Add, Диагностика полей, DoDiagnose
    Menu, Tray, Add, Перезагрузить настройки, ReloadConfig
    Menu, Tray, Add
    Menu, Tray, Add, Выход, ExitApp
}

ReloadConfig:
    LoadConfig()
    TrayTip, Microinvest Assistant, Настройки перезагружены. Клавиша: %g_Hotkey%, 2, 1
return

ExitApp:
ExitApp

DoAssist:
    Critical, Off
    AssistFill()
return

DoDiagnose:
    Critical, Off
    DiagnoseFields()
return

AssistFill() {
    global g_WindowTitle, g_FieldName, g_FieldBarcode, g_FieldNtin
    global g_Python, g_Timeout, g_ResultFile, g_ResultTxt, g_LogDir
    global g_ResolvedNameNN, g_ResolvedWinId

    ; 1) Сначала читаем из ТЕКУЩЕГО активного окна (карточка товара),
    ;    не переключаясь на главное окно Microinvest — иначе поле «Имя» кажется пустым.
    winId := FindCardWindow(g_FieldName)
    if (!winId) {
        MsgBox, 48, Microinvest Assistant, Не найдено окно карточки товара.`nОткройте карточку в Microinvest, кликните в поле «Имя» и нажмите F8 снова.`n`nЛибо: ПКМ по иконке в трее → Диагностика полей.
        return
    }

    nameText := ReadEditText(winId, g_FieldName, true)
    nameText := Trim(nameText)
    g_ResolvedWinId := winId

    if (nameText = "") {
        dump := DumpEditControls(winId)
        MsgBox, 48, Microinvest Assistant, Поле «Имя» не прочиталось (ClassNN мог измениться).`n`nКликните мышкой в поле «Имя» и нажмите F8 ещё раз.`nИли обновите field_name в config.ini по диагностике.`n`nНайденные EDIT с текстом:`n%dump%
        return
    }

    originalQuery := nameText

    if FileExist(g_ResultFile)
        FileDelete, %g_ResultFile%
    if FileExist(g_ResultTxt)
        FileDelete, %g_ResultTxt%

    scriptPath := A_ScriptDir . "\python\main.py"
    if !FileExist(scriptPath) {
        MsgBox, 16, Microinvest Assistant, Не найден python\main.py
        return
    }

    TrayTip, Microinvest Assistant, Идёт поиск: %originalQuery% ..., 3, 1

    cmd := """" . g_Python . """ """ . scriptPath . """ --query """ . EscapeArg(originalQuery) . """ --config """ . A_ScriptDir . "\config.ini"" --result """ . g_ResultFile . """"
    RunWait, %comspec% /c %cmd%, %A_ScriptDir%, Hide UseErrorLevel
    exitCode := ErrorLevel

    status := ""
    message := ""
    newName := ""
    barcode := ""
    ntin := ""
    ntinMissing := ""

    if FileExist(g_ResultTxt) {
        oldEnc := A_FileEncoding
        FileEncoding, UTF-16
        FileRead, txtData, %g_ResultTxt%
        FileEncoding, %oldEnc%
        status := IniGetSection(txtData, "status")
        message := IniGetSection(txtData, "message")
        newName := IniGetSection(txtData, "name")
        barcode := IniGetSection(txtData, "barcode")
        ntin := IniGetSection(txtData, "ntin")
        ntinMissing := IniGetSection(txtData, "ntin_missing")
    } else if FileExist(g_ResultFile) {
        oldEnc := A_FileEncoding
        FileEncoding, UTF-8
        FileRead, jsonText, %g_ResultFile%
        FileEncoding, %oldEnc%
        status := JsonGet(jsonText, "status")
        message := JsonGet(jsonText, "message")
        newName := JsonGet(jsonText, "name")
        barcode := JsonGet(jsonText, "barcode")
        ntin := JsonGet(jsonText, "ntin")
        ntinMissing := JsonGet(jsonText, "ntin_missing")
    } else {
        MsgBox, 16, Microinvest Assistant, Python не вернул результат.`nКод выхода: %exitCode%`nСм. логи в папке logs\
        return
    }

    if (status = "cancelled") {
        TrayTip, Microinvest Assistant, Выбор отменён. Данные не изменены., 2, 1
        return
    }

    if (status = "not_found" || status = "error" || newName = "") {
        if (message = "")
            message := "Товар по запросу " . originalQuery . " не найден."
        MsgBox, 48, Microinvest Assistant, %message%
        return
    }

    ; Вернуть фокус на карточку перед записью
    if WinExist("ahk_id " . winId)
        WinActivate, ahk_id %winId%

    if !WriteEditText(winId, g_FieldName, newName) {
        MsgBox, 16, Microinvest Assistant, Не удалось записать поле «Имя».`nПроверьте ClassNN в config.ini`nТекущий: %g_ResolvedNameNN%
        return
    }
    if (barcode = "")
        barcode := originalQuery
    WriteEditText(winId, g_FieldBarcode, barcode)

    if (ntin != "")
        WriteEditText(winId, g_FieldNtin, ntin)

    if (ntinMissing = "true" || ntinMissing = "1" || ntin = "") {
        MsgBox, 64, Microinvest Assistant, Название найдено, NTIN отсутствует.`n`nИмя: %newName%`nШтрихкод: %barcode%
    } else {
        TrayTip, Microinvest Assistant, Карточка заполнена.`n%newName%, 3, 1
    }
}

; --- Поиск окна карточки ----------------------------------------------------

FindCardWindow(preferredClassNN) {
    global g_WindowTitle

    ; A) Активное окно — самый частый случай (пользователь уже в карточке)
    WinGet, activeId, ID, A
    if (activeId && WindowHasEdit(activeId, preferredClassNN))
        return activeId
    if (activeId && ReadEditText(activeId, preferredClassNN, true) != "")
        return activeId
    if (activeId && GetFocusedEditText(activeId) != "")
        return activeId

    ; B) Все окна того же процесса (диалог карточки / MDI)
    if (activeId) {
        WinGet, pid, PID, ahk_id %activeId%
        found := FindWindowInPid(pid, preferredClassNN)
        if (found)
            return found
    }

    ; C) Окна с «Microinvest» в заголовке
    WinGet, idList, List, %g_WindowTitle%
    Loop, %idList% {
        thisId := idList%A_Index%
        if WindowHasEdit(thisId, preferredClassNN)
            return thisId
        if (ReadEditText(thisId, preferredClassNN, false) != "")
            return thisId
    }

    ; D) Любое видимое окно с нужным суффиксом _adNNN
    WinGet, idList2, List
    Loop, %idList2% {
        thisId := idList2%A_Index%
        if WindowHasEdit(thisId, preferredClassNN)
            return thisId
    }
    return 0
}

FindWindowInPid(pid, preferredClassNN) {
    WinGet, idList, List, ahk_pid %pid%
    Loop, %idList% {
        thisId := idList%A_Index%
        if WindowHasEdit(thisId, preferredClassNN)
            return thisId
        txt := ReadEditText(thisId, preferredClassNN, true)
        if (txt != "")
            return thisId
    }
    return 0
}

WindowHasEdit(winId, classNN) {
    nn := ResolveEditNN(winId, classNN)
    return nn != ""
}

; --- Чтение / запись Edit ---------------------------------------------------

ReadEditText(winId, classNN, allowFocusFallback := false) {
    global g_ResolvedNameNN
    nn := ResolveEditNN(winId, classNN)
    if (nn != "") {
        g_ResolvedNameNN := nn
        text := GetTextByNN(winId, nn)
        if (text != "")
            return text
    }

    if (allowFocusFallback) {
        text := GetFocusedEditText(winId)
        if (text != "")
            return text
    }
    return ""
}

GetFocusedEditText(winId) {
    ControlGetFocus, focusNN, ahk_id %winId%
    if (focusNN = "" || !InStr(focusNN, "EDIT"))
        return ""
    return GetTextByNN(winId, focusNN)
}

GetTextByNN(winId, classNN) {
    ; 1) Обычный ControlGetText
    text := ""
    ControlGetText, text, %classNN%, ahk_id %winId%
    if (text != "")
        return text

    ; 2) Unicode WM_GETTEXT — надёжнее для WinForms
    ControlGet, hCtrl, Hwnd, , %classNN%, ahk_id %winId%
    if (!hCtrl)
        return ""

    SendMessage, 0x000E, 0, 0, , ahk_id %hCtrl%  ; WM_GETTEXTLENGTH
    len := ErrorLevel
    if (len = "" || len = "FAIL" || len = 0) {
        ; Иногда length=0 ошибочно — пробуем буфер фиксированного размера
        len := 512
    }
    chars := len + 1
    VarSetCapacity(buf, chars * 2, 0)
    SendMessage, 0x000D, chars, &buf, , ahk_id %hCtrl%  ; WM_GETTEXT
    text := StrGet(&buf, "UTF-16")
    return text
}

WriteEditText(winId, classNN, value) {
    nn := ResolveEditNN(winId, classNN)
    if (nn = "")
        return false

    ControlGet, hwnd, Hwnd, , %nn%, ahk_id %winId%
    if (hwnd) {
        SendMessage, 0x000C, 0, &value, , ahk_id %hwnd%  ; WM_SETTEXT
        got := GetTextByNN(winId, nn)
        if (got = value)
            return true
    }

    ControlSetText, %nn%, %value%, ahk_id %winId%
    got := GetTextByNN(winId, nn)
    if (got = value)
        return true

    clipSaved := ClipboardAll
    Clipboard :=
    Clipboard := value
    ClipWait, 1
    ControlFocus, %nn%, ahk_id %winId%
    Sleep, 50
    Send, ^a
    Sleep, 30
    Send, ^v
    Sleep, 50
    Clipboard := clipSaved
    return true
}

; Ищет актуальный ClassNN: точное имя → тот же _adNNN → любой EDIT.app.*_adNNN
ResolveEditNN(winId, classNN) {
    if (classNN = "")
        return ""

    ; Точное совпадение
    ControlGet, hwnd, Hwnd, , %classNN%, ahk_id %winId%
    if (!ErrorLevel && hwnd)
        return classNN

    suffix := ExtractAdSuffix(classNN)
    if (suffix = "")
        return ""

    WinGet, ctrlList, ControlList, ahk_id %winId%
    exact := ""
    fuzzy := ""
    Loop, Parse, ctrlList, `n
    {
        ctrl := A_LoopField
        if !InStr(ctrl, "EDIT")
            continue
        if RegExMatch(ctrl, "i)" . suffix . "$") {
            ; Предпочитаем WindowsForms10.EDIT
            if InStr(ctrl, "WindowsForms") && InStr(ctrl, "EDIT")
                return ctrl
            if (exact = "")
                exact := ctrl
        }
        ; Иногда суффикс пишется иначе — сравнить только цифры ad
        if (fuzzy = "" && RegExMatch(suffix, "i)_ad(\d+)$", sm) && RegExMatch(ctrl, "i)_ad" . sm1 . "$"))
            fuzzy := ctrl
    }
    if (exact != "")
        return exact
    return fuzzy
}

ExtractAdSuffix(classNN) {
    if RegExMatch(classNN, "i)(_ad\d+)$", m)
        return m1
    return ""
}

DumpEditControls(winId) {
    WinGet, ctrlList, ControlList, ahk_id %winId%
    WinGetTitle, title, ahk_id %winId%
    out := "Окно: " . title . "`n"
    count := 0
    Loop, Parse, ctrlList, `n
    {
        ctrl := A_LoopField
        if !InStr(ctrl, "EDIT")
            continue
        val := GetTextByNN(winId, ctrl)
        val := Trim(val)
        if (val = "")
            continue
        count += 1
        if (count > 12) {
            out .= "...`n"
            break
        }
        ; Обрезать длинные значения
        show := val
        if (StrLen(show) > 40)
            show := SubStr(show, 1, 40) . "..."
        out .= ctrl . " = " . show . "`n"
    }
    if (count = 0)
        out .= "(нет EDIT с текстом в этом окне)`n"
    return out
}

DiagnoseFields() {
    global g_FieldName, g_FieldBarcode, g_FieldNtin, g_LogDir
    WinGet, winId, ID, A
    WinGetTitle, title, ahk_id %winId%
    dump := DumpEditControls(winId)
    focusTxt := GetFocusedEditText(winId)
    ControlGetFocus, focusNN, ahk_id %winId%
    nameTry := ReadEditText(winId, g_FieldName, true)

    report := "Заголовок: " . title . "`n"
    report .= "Фокус: " . focusNN . " = " . focusTxt . "`n"
    report .= "Чтение field_name: [" . nameTry . "]`n"
    report .= "Ищем: " . g_FieldName . "`n`n"
    report .= dump

    path := g_LogDir . "\diagnose_controls.txt"
    oldEnc := A_FileEncoding
    FileEncoding, UTF-8
    FileDelete, %path%
    FileAppend, %report%, %path%
    FileEncoding, %oldEnc%

    MsgBox, 64, Диагностика полей, %report%`n`nСохранено: %path%
}

EscapeArg(s) {
    StringReplace, s, s, ", \", All
    return s
}

IniGetSection(data, key) {
    pattern := "m)^" . key . "=(.*)$"
    if RegExMatch(data, pattern, m)
        return Trim(m1)
    return ""
}

JsonGet(json, key) {
    pattern := """" . key . """\s*:\s*""((?:\\.|[^""])*)"""
    if RegExMatch(json, pattern, m)
        return UnescapeJson(m1)
    pattern2 := """" . key . """\s*:\s*([^,}\s]+)"
    if RegExMatch(json, pattern2, m2) {
        v := Trim(m21)
        if (v = "null")
            return ""
        return v
    }
    return ""
}

UnescapeJson(s) {
    StringReplace, s, s, \", ", All
    StringReplace, s, s, \\, \, All
    StringReplace, s, s, \n, `n, All
    StringReplace, s, s, \r, `r, All
    StringReplace, s, s, \t, `t, All
    Loop {
        if !RegExMatch(s, "i)\\u([0-9a-f]{4})", m)
            break
        code := "0x" . m1
        ch := Chr(code)
        StringReplace, s, s, %m%, %ch%, All
    }
    return s
}
