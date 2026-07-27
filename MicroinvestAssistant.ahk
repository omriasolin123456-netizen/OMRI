; =============================================================================
; Microinvest Склад Pro — помощник заполнения карточек автозапчастей
; AutoHotkey 1.1+ / Windows 11 x64
; Горячая клавиша и ClassNN полей читаются из config.ini
; =============================================================================
#NoEnv
#SingleInstance Force
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
global g_LogDir := A_ScriptDir . "\logs"

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

AssistFill() {
    global g_WindowTitle, g_FieldName, g_FieldBarcode, g_FieldNtin
    global g_Python, g_Timeout, g_ResultFile, g_LogDir

    WinGet, winId, ID, A
    WinGetTitle, activeTitle, ahk_id %winId%
    if !InStr(activeTitle, g_WindowTitle) {
        if !WinExist(g_WindowTitle) {
            MsgBox, 48, Microinvest Assistant, Окно Microinvest не найдено.`nОткройте карточку товара и повторите.
            return
        }
        WinActivate, %g_WindowTitle%
        WinWaitActive, %g_WindowTitle%, , 2
        WinGet, winId, ID, A
    }

    nameText := GetControlText(winId, g_FieldName)
    nameText := Trim(nameText)
    if (nameText = "") {
        MsgBox, 48, Microinvest Assistant, Поле «Имя» пустое.`nОтсканируйте код или введите номер запчасти.
        return
    }

    ; Сохраняем исходное значение — оно станет штрихкодом
    originalQuery := nameText

    ; Удаляем предыдущий результат
    if FileExist(g_ResultFile)
        FileDelete, %g_ResultFile%

    scriptPath := A_ScriptDir . "\python\main.py"
    if !FileExist(scriptPath) {
        MsgBox, 16, Microinvest Assistant, Не найден python\main.py
        return
    }

    TrayTip, Microinvest Assistant, Идёт поиск: %originalQuery% ..., 3, 1

    ; Передаём исходный запрос и путь к config
    cmd := """" . g_Python . """ """ . scriptPath . """ --query """ . EscapeArg(originalQuery) . """ --config """ . A_ScriptDir . "\config.ini"" --result """ . g_ResultFile . """"
    RunWait, %comspec% /c %cmd%, %A_ScriptDir%, Hide UseErrorLevel
    exitCode := ErrorLevel

    if !FileExist(g_ResultFile) {
        MsgBox, 16, Microinvest Assistant, Python не вернул результат.`nКод выхода: %exitCode%`nСм. логи в папке logs\
        return
    }

    FileRead, jsonText, %g_ResultFile%
    status := JsonGet(jsonText, "status")
    message := JsonGet(jsonText, "message")
    newName := JsonGet(jsonText, "name")
    barcode := JsonGet(jsonText, "barcode")
    ntin := JsonGet(jsonText, "ntin")
    ntinMissing := JsonGet(jsonText, "ntin_missing")

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

    ; Успех — заполняем поля. Ничего не меняем при пустом имени.
    if !SetControlText(winId, g_FieldName, newName) {
        MsgBox, 16, Microinvest Assistant, Не удалось записать поле «Имя».`nПроверьте ClassNN в config.ini
        return
    }
    if (barcode = "")
        barcode := originalQuery
    SetControlText(winId, g_FieldBarcode, barcode)

    if (ntin != "")
        SetControlText(winId, g_FieldNtin, ntin)

    if (ntinMissing = "true" || ntinMissing = "1" || ntin = "") {
        MsgBox, 64, Microinvest Assistant, Название найдено, NTIN отсутствует.`n`nИмя: %newName%`nШтрихкод: %barcode%
    } else {
        TrayTip, Microinvest Assistant, Карточка заполнена.`n%newName%, 3, 1
    }
}

GetControlText(winId, classNN) {
    text := ""
    try {
        ControlGetText, text, %classNN%, ahk_id %winId%
    } catch e {
        text := ""
    }
    if (text = "") {
        ; Попытка найти похожий ClassNN (меняется суффикс _rN_)
        matched := FindSimilarEdit(winId, classNN)
        if (matched != "")
            ControlGetText, text, %matched%, ahk_id %winId%
    }
    return text
}

SetControlText(winId, classNN, value) {
    target := classNN
    ControlGetText, probe, %classNN%, ahk_id %winId%
    if (ErrorLevel) {
        matched := FindSimilarEdit(winId, classNN)
        if (matched = "")
            return false
        target := matched
    }
    ControlSetText, %target%, %value%, ahk_id %winId%
    ; Дополнительно через EM_REPLACESEL не требуется для WinForms Edit
    return true
}

; Ищет Edit с тем же окончанием (например _ad110), если _r8_ изменился
FindSimilarEdit(winId, classNN) {
    suffix := ""
    if RegExMatch(classNN, "i)_ad\d+$", m)
        suffix := m
    else
        return ""

    WinGet, ctrlList, ControlList, ahk_id %winId%
    Loop, Parse, ctrlList, `n
    {
        ctrl := A_LoopField
        if InStr(ctrl, "EDIT") && RegExMatch(ctrl, "i)" . suffix . "$")
            return ctrl
    }
    return ""
}

EscapeArg(s) {
    StringReplace, s, s, ", \", All
    return s
}

; Простой JSON-парсер для плоских строковых полей
JsonGet(json, key) {
    pattern := """" . key . """\s*:\s*""((?:\\.|[^""])*)"""
    if RegExMatch(json, pattern, m)
        return UnescapeJson(m1)
    ; число / bool / null
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
    return s
}
