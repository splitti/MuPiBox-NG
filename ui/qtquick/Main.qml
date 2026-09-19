import QtQuick 2.15
import QtQuick.Window 2.15
import QtQuick.VirtualKeyboard

Window {
    id: root
    visible: true
    visibility: Window.FullScreen
    color: root.backgroundColor
    title: "MuPiBox"

    FontLoader {
        id: retroFont
        source: "assets/PressStart2P-Regular.ttf"
    }

    property real uiScale: Math.min(width / 800, height / 480)
    property var apiCandidates: ["http://127.0.0.1:8090", "http://127.0.0.1:8080"]
    property int apiIndex: 0
    property string apiBase: apiCandidates[apiIndex]
    property bool backendOnline: false
    property bool pending: false
    property bool statusRequestRunning: false
    property string clockText: "--:--"
    property var categories: []
    property string homeSignature: ""
    property string uiSize: "normal"
    property string themeName: "modern-dark"
    property bool largeUI: uiSize === "large"
    property bool retroTheme: themeName === "arcade-8bit"
    property bool networkOnline: true
    property bool playerView: false
    property bool wifiPanelVisible: false
    property bool wifiBusy: false
    property var wifiNetworks: []
    property string wifiSelectedSSID: ""
    property string wifiSelectedSecurity: ""
    property string wifiSelectedInterface: ""
    property string wifiPassword: ""
    property int wifiHoldMs: 0
    property color backgroundColor: retroTheme ? "#1A1666" : "#0B0D11"
    property color statusColor: retroTheme ? "#25207A" : "#0A0C10"
    property color panelColor: retroTheme ? "#40318D" : "#202632"
    property color cardColor: retroTheme ? "#302878" : "#141820"
    property color textColor: retroTheme ? "#FFFFFF" : "#F7F7FA"
    property color mutedColor: retroTheme ? "#C7C2FF" : "#9AA3B2"
    property color accentColor: retroTheme ? "#8B7DDB" : "#F59ACA"
    property color accentPressedColor: retroTheme ? "#B8AFFF" : "#FFB8D8"
    property color lineColor: retroTheme ? "#B8AFFF" : "#2A313D"
    property color accentTextColor: retroTheme ? "#17124F" : "#2C1F2E"
    property string uiFont: retroTheme && retroFont.status === FontLoader.Ready ? retroFont.name : (retroTheme ? "DejaVu Sans Mono" : "DejaVu Sans")
    property int uiRestartGeneration: -1
    property var wifiStatus: ({"connected": false, "quality_percent": 0, "signal_dbm": 0, "interface": ""})
    property var batteryStatus: ({"available": false, "percent": 0, "charging": false})
    property var playerState: ({
        "queue": [],
        "index": 0,
        "position": 0,
        "duration": 0,
        "volume": 30,
        "max_volume": 60,
        "state": "stopped",
        "folder": "",
        "folder_id": "",
        "cover": ""
    })
    property string transientMessage: ""
    property var spotifyState: ({
        "connected": false, "playing": false, "paused": false,
        "volume": 0, "volume_steps": 100, "track": null
    })
    // Arbitration (internal/server) already keeps local playback and Spotify
    // mutually exclusive; this only decides which one the single player bar
    // displays during the brief moments both could otherwise look inactive.
    property bool spotifyActive: spotifyState.connected && (spotifyState.playing || spotifyState.paused)
    property bool showSpotifyBar: spotifyActive && root.playerState.state !== "playing"

    function localized(labels, fallback) {
        if (!labels) return fallback || ""
        return labels.de || labels.en || fallback || ""
    }

    function categoryItems(category) {
        var result = []
        var rows = category && category.rows ? category.rows : []
        for (var rowIndex = 0; rowIndex < rows.length; ++rowIndex) {
            var items = rows[rowIndex].items || []
            for (var itemIndex = 0; itemIndex < items.length; ++itemIndex) {
                if (networkOnline || items[itemIndex].offline_available) result.push(items[itemIndex])
            }
        }
        return result
    }

    function signalLevel(percent) {
        var value = Math.max(0, Math.min(100, Number(percent || 0)))
        return value >= 75 ? 4 : (value >= 50 ? 3 : (value >= 25 ? 2 : (value > 0 ? 1 : 0)))
    }

    function signalColor(percent) {
        var level = signalLevel(percent)
        return level >= 3 ? "#43c86a" : (level === 2 ? "#f2cf4a" : "#ef4b5f")
    }

    function batteryColor(percent) {
        var value = Math.max(0, Math.min(100, Number(percent || 0)))
        return value <= 15 ? "#ef4b5f" : (value <= 25 ? "#f2cf4a" : (value <= 50 ? "#b7c94b" : (value <= 75 ? "#8bd66a" : "#43c86a")))
    }

    function wifiSecurityRequired(security) {
        var value = String(security || "").toLowerCase()
        return value !== "" && value !== "--" && value !== "open" && value !== "offen"
    }

    function coverUrl(path) {
        if (!path) return ""
        if (path.indexOf("http://") === 0 || path.indexOf("https://") === 0 || path.indexOf("file:") === 0)
            return path
        return apiBase + (path.charAt(0) === "/" ? "" : "/") + path
    }

    function currentTrack() {
        var queue = playerState.queue || []
        var index = Number(playerState.index || 0)
        return index >= 0 && index < queue.length ? queue[index] : null
    }

    function formatTime(seconds) {
        var value = Math.max(0, Math.floor(Number(seconds || 0)))
        return Math.floor(value / 60) + ":" + String(value % 60).padStart(2, "0")
    }

    function showMessage(message) {
        transientMessage = message || ""
        messageTimer.restart()
    }

    function openWifiPanel() {
        wifiPanelVisible = true
        wifiSelectedSSID = ""
        wifiSelectedSecurity = ""
        wifiSelectedInterface = ""
        wifiPassword = ""
        scanWifiNetworks()
    }

    function scanWifiNetworks() {
        if (wifiBusy) return
        wifiBusy = true
        requestJson("GET", "/api/connectivity/wifi", null, function(data) {
            wifiNetworks = data && data.networks ? data.networks : []
            wifiBusy = false
        }, function() {
            wifiBusy = false
            showMessage("WLAN-Suche nicht verfügbar.")
        })
    }

    function connectSelectedWifi() {
        if (wifiBusy || wifiSelectedSSID === "") return
        var secure = wifiSecurityRequired(wifiSelectedSecurity)
        if (secure && wifiPassword.length < 8) { showMessage("WLAN-Passwort muss mindestens 8 Zeichen haben."); return }
        wifiBusy = true
        requestJson("POST", "/api/connectivity/wifi/connect", {"ssid": wifiSelectedSSID, "password": wifiPassword, "interface": wifiSelectedInterface}, function() {
            wifiBusy = false
            wifiPassword = ""
            wifiPanelVisible = false
            Qt.inputMethod.hide()
            showMessage("WLAN wird verbunden …")
            refreshSystem()
        }, function() {
            wifiBusy = false
            showMessage("WLAN-Verbindung fehlgeschlagen.")
        })
    }

    function requestJson(method, path, payload, done, failed) {
        var xhr = new XMLHttpRequest()
        xhr.open(method, apiBase + path)
        xhr.setRequestHeader("Accept", "application/json")
        if (payload !== undefined && payload !== null)
            xhr.setRequestHeader("Content-Type", "application/json")
        xhr.onreadystatechange = function() {
            if (xhr.readyState !== XMLHttpRequest.DONE) return
            if (xhr.status >= 200 && xhr.status < 300) {
                backendOnline = true
                try {
                    done(JSON.parse(xhr.responseText))
                } catch (error) {
                    if (failed) failed()
                }
            } else if (failed) {
                failed()
            }
        }
        try {
            xhr.send(payload !== undefined && payload !== null ? JSON.stringify(payload) : "")
        } catch (error) {
            if (failed) failed()
        }
    }

    function tryApi(index, done, failed) {
        apiIndex = index
        apiBase = apiCandidates[index]
        requestJson("GET", "/api/health", null, function() {
            done()
        }, function() {
            if (index + 1 < apiCandidates.length)
                tryApi(index + 1, done, failed)
            else
                failed()
        })
    }

    function refreshHome() {
        requestJson("GET", "/api/home", null, function(data) {
            var nextCategories = data && data.categories ? data.categories : []
            var signature = JSON.stringify(nextCategories)
            if (signature !== homeSignature) {
                homeSignature = signature
                categories = nextCategories
            }
        }, function() {
            backendOnline = false
        })
    }

    function refreshInfo() {
        requestJson("GET", "/api/info", null, function(data) {
            uiSize = data && data.display && data.display.ui_size ? data.display.ui_size : "normal"
            themeName = data && data.theme ? data.theme : "modern-dark"
        }, function() {})
    }

    function refreshUIState() {
        requestJson("GET", "/api/ui-state", null, function(data) {
            var generation = Number(data && data.restart_generation || 0)
            if (uiRestartGeneration < 0) uiRestartGeneration = generation
            else if (generation !== uiRestartGeneration) Qt.quit()
        }, function() {})
    }

    function refreshSystem() {
        requestJson("GET", "/api/system", null, function(data) {
            networkOnline = data && data.online !== undefined ? data.online : true
            wifiStatus = data && data.wifi ? data.wifi : {"connected": false, "quality_percent": 0}
            batteryStatus = data && data.battery ? data.battery : {"available": false, "percent": 0, "charging": false}
        }, function() {
            networkOnline = false
            wifiStatus = {"connected": false, "quality_percent": 0}
            batteryStatus = {"available": false, "percent": 0, "charging": false}
        })
    }

    function refreshStatus() {
        if (statusRequestRunning) return
        statusRequestRunning = true
        requestJson("GET", "/api/status", null, function(data) {
            playerState = data || playerState
            statusRequestRunning = false
        }, function() {
            backendOnline = false
            statusRequestRunning = false
            tryApi(0, function() {
                refreshHome()
            }, function() {})
        })
    }

    function command(payload) {
        if (pending || !backendOnline) return
        pending = true
        requestJson("POST", "/api/command", payload, function(data) {
            playerState = data || playerState
            if (payload && payload.action === "folder" && largeUI) playerView = true
            pending = false
        }, function() {
            pending = false
            showMessage("Befehl konnte nicht ausgeführt werden.")
        })
    }

    property bool spotifyPending: false
    function refreshSpotify() {
        requestJson("GET", "/api/spotify/status", null, function(data) {
            spotifyState = data || spotifyState
        }, function() {})
    }
    function spotifyCommand(payload) {
        if (spotifyPending || !backendOnline) return
        spotifyPending = true
        requestJson("POST", "/api/spotify/command", payload, function(data) {
            spotifyState = data || spotifyState
            spotifyPending = false
        }, function() {
            spotifyPending = false
            showMessage("Spotify-Befehl konnte nicht ausgeführt werden.")
        })
    }

    // Unified accessors so the single player bar can show either the local
    // player or an active Spotify Connect session (see showSpotifyBar) --
    // arbitration in internal/server already keeps the two mutually
    // exclusive, this just decides what the shared bar renders/sends.
    function currentDisplayTitle() {
        if (root.showSpotifyBar) return (spotifyState.track && spotifyState.track.name) || "Spotify Connect"
        var track = root.currentTrack()
        return track ? track.title : "Such dir etwas aus"
    }
    function currentDisplaySubtitle() {
        if (root.showSpotifyBar) {
            var artists = spotifyState.track && spotifyState.track.artists
            return (artists && artists.length ? artists.join(", ") : "Spotify Connect")
        }
        return root.playerState.folder || "Deine Medien warten auf dich."
    }
    function currentDisplayStatusLine() {
        if (root.showSpotifyBar) return spotifyState.paused ? "Spotify · Pausiert" : "Spotify · Wiedergabe"
        var queue = root.playerState.queue || []
        if (!queue.length) return ""
        var states = {"playing": "Wiedergabe", "paused": "Pausiert", "stopped": "Gestoppt", "error": "Fehler"}
        return String(Number(root.playerState.index || 0) + 1) + " / " + String(queue.length) + " · " + (states[root.playerState.state] || "")
    }
    function currentCoverUrl() {
        if (root.showSpotifyBar) return (spotifyState.track && spotifyState.track.cover) || ""
        return root.coverUrl(root.playerState.cover || "")
    }
    function transportEnabled() {
        if (root.showSpotifyBar) return root.backendOnline && spotifyState.connected
        return root.backendOnline && (root.playerState.queue || []).length > 0
    }
    function toggleLabel() {
        if (root.showSpotifyBar) return spotifyState.playing ? "Ⅱ" : "▶"
        return root.playerState.state === "playing" ? "Ⅱ" : "▶"
    }
    function onPreviousTapped() {
        if (root.showSpotifyBar) root.spotifyCommand({"action": "previous"})
        else root.command({"action": "previous"})
    }
    function onToggleTapped() {
        if (root.showSpotifyBar) root.spotifyCommand({"action": spotifyState.playing ? "pause" : "resume"})
        else root.command({"action": "toggle"})
    }
    function onNextTapped() {
        if (root.showSpotifyBar) root.spotifyCommand({"action": "next"})
        else root.command({"action": "next"})
    }
    function currentPositionSeconds() {
        if (root.showSpotifyBar) return (spotifyState.track ? Number(spotifyState.track.position_ms || 0) : 0) / 1000
        return Number(root.playerState.position || 0)
    }
    function currentDurationSeconds() {
        if (root.showSpotifyBar) return (spotifyState.track ? Number(spotifyState.track.duration_ms || 0) : 0) / 1000
        return Number(root.playerState.duration || 0)
    }
    function seekEnabled() {
        if (root.showSpotifyBar) return root.backendOnline && spotifyState.connected && root.currentDurationSeconds() > 0
        return root.backendOnline && Number(root.playerState.duration || 0) > 0
    }
    function onSeekTapped(localX, trackWidth) {
        var duration = root.currentDurationSeconds()
        if (root.showSpotifyBar) root.spotifyCommand({"action": "seek", "value": Math.round(localX / trackWidth * duration * 1000)})
        else root.command({"action": "seek", "value": Math.round(localX / trackWidth * duration)})
    }
    function currentVolume() {
        if (root.showSpotifyBar) return Number(spotifyState.volume || 0)
        return Number(root.playerState.volume || 0)
    }
    function currentMaxVolume() {
        if (root.showSpotifyBar) return Number(spotifyState.volume_steps || 100)
        return Number(root.playerState.max_volume || 60)
    }
    function onVolumeTapped(localX, trackWidth) {
        var max = root.currentMaxVolume()
        var value = Math.round(localX / trackWidth * max)
        if (root.showSpotifyBar) root.spotifyCommand({"action": "volume", "value": value})
        else root.command({"action": "volume", "value": value})
    }

    function updateClock() {
        clockText = Qt.formatTime(new Date(), "hh:mm")
    }

    Component.onCompleted: {
        updateClock()
        tryApi(0, function() {
            refreshHome()
            refreshInfo()
            refreshSystem()
            refreshUIState()
            refreshStatus()
            refreshSpotify()
        }, function() {
            backendOnline = false
        })
    }

    Timer { interval: 750; running: true; repeat: true; onTriggered: root.refreshStatus() }
    Timer { interval: 1000; running: true; repeat: true; onTriggered: root.refreshSpotify() }
    Timer {
        interval: 2000
        running: true
        repeat: true
        onTriggered: {
            root.refreshHome()
            root.refreshInfo()
            root.refreshSystem()
            root.refreshUIState()
        }
    }
    Timer { interval: 30000; running: true; repeat: true; onTriggered: root.updateClock() }
    Timer { id: messageTimer; interval: 6000; repeat: false; onTriggered: root.transientMessage = "" }

    Item {
        id: canvas
        width: 800
        height: 480
        anchors.centerIn: parent
        scale: root.uiScale

        Rectangle {
            anchors.fill: parent
            color: root.backgroundColor
        }

        Rectangle {
            id: statusBar
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: parent.top
            height: root.largeUI ? 42 : 34
            color: root.statusColor
            border.color: root.lineColor

            Row {
                anchors.left: parent.left
                anchors.leftMargin: 10
                anchors.verticalCenter: parent.verticalCenter
                spacing: 7

                Rectangle {
                    width: root.largeUI ? 28 : 22
                    height: root.largeUI ? 28 : 22
                    radius: root.retroTheme ? 0 : (root.largeUI ? 9 : 7)
                    color: root.accentColor
                    Text {
                        anchors.centerIn: parent
                        text: "m"
                        color: root.accentTextColor
                        font.pixelSize: root.largeUI ? 19 : 15
                        font.family: root.uiFont
                        font.bold: true
                    }
                }

                Text {
                    anchors.verticalCenter: parent.verticalCenter
                    text: "MuPiBox"
                    color: root.textColor
                    font.pixelSize: root.largeUI ? 16 : 13
                    font.family: root.uiFont
                    font.bold: true
                }
            }

            Row {
                anchors.right: parent.right
                anchors.rightMargin: 10
                anchors.verticalCenter: parent.verticalCenter
                spacing: 12

                Item {
                    width: 26
                    height: 18
                    opacity: root.wifiStatus.connected ? 1 : 0.45

                    Row {
                        anchors.fill: parent
                        spacing: 2
                        Repeater {
                            model: 4
                            delegate: Rectangle {
                                width: 4
                                height: 5 + index * 3
                                anchors.bottom: parent.bottom
                                radius: root.retroTheme ? 0 : 1
                                color: index < root.signalLevel(root.wifiStatus.quality_percent) ? root.signalColor(root.wifiStatus.quality_percent) : "#343b47"
                            }
                        }
                    }

                    Text {
                        visible: root.wifiBusy
                        anchors.centerIn: parent
                        text: "↻"
                        color: root.accentColor
                        font.pixelSize: 20
                        font.family: root.uiFont
                        NumberAnimation on rotation {
                            running: root.wifiBusy
                            loops: Animation.Infinite
                            from: 0
                            to: 360
                            duration: 700
                        }
                    }

                    Rectangle {
                        anchors.left: parent.left
                        anchors.bottom: parent.bottom
                        anchors.bottomMargin: -7
                        width: parent.width * Math.min(1, root.wifiHoldMs / 900)
                        height: 3
                        radius: root.retroTheme ? 0 : 1
                        color: root.accentColor
                        visible: wifiStatusTouch.pressed
                    }

                    MouseArea {
                        id: wifiStatusTouch
                        anchors.fill: parent
                        anchors.margins: -8
                        pressAndHoldInterval: 900
                        onPressed: {
                            root.wifiHoldMs = 0
                            wifiHoldProgress.start()
                        }
                        onReleased: {
                            wifiHoldProgress.stop()
                            root.wifiHoldMs = 0
                        }
                        onCanceled: {
                            wifiHoldProgress.stop()
                            root.wifiHoldMs = 0
                        }
                        onPressAndHold: {
                            wifiHoldProgress.stop()
                            root.wifiHoldMs = 0
                            root.openWifiPanel()
                        }
                    }

                    Timer {
                        id: wifiHoldProgress
                        interval: 30
                        repeat: true
                        onTriggered: root.wifiHoldMs = Math.min(900, root.wifiHoldMs + interval)
                    }
                }

                Item {
                    width: 34
                    height: 18
                    opacity: root.batteryStatus.available ? 1 : 0.45

                    Rectangle {
                        id: batteryBody
                        x: 0
                        y: 1
                        width: 28
                        height: 16
                        radius: root.retroTheme ? 0 : 3
                        color: "transparent"
                        border.width: 2
                        border.color: "#7d8795"

                        Rectangle {
                            x: 2
                            anchors.verticalCenter: parent.verticalCenter
                            width: root.batteryStatus.available ? (parent.width - 4) * Math.max(0, Math.min(100, Number(root.batteryStatus.percent || 0))) / 100 : 0
                            height: parent.height - 4
                            radius: root.retroTheme ? 0 : 1
                            color: root.batteryColor(root.batteryStatus.percent)
                        }

                        Text {
                            visible: root.batteryStatus.available && root.batteryStatus.charging
                            anchors.centerIn: parent
                            text: "⚡"
                            color: "#ffffff"
                            font.pixelSize: 12
                            font.family: root.uiFont
                            style: Text.Outline
                            styleColor: "#222222"
                        }
                    }

                    Rectangle {
                        x: 29
                        y: 6
                        width: 3
                        height: 6
                        radius: root.retroTheme ? 0 : 1
                        color: "#7d8795"
                    }
                }

                Item {
                    width: 48
                    height: 28

                    Text {
                        anchors.centerIn: parent
                        text: root.clockText
                        color: root.textColor
                        font.pixelSize: 11
                        font.family: root.uiFont
                    }
                }
            }
        }

        Flickable {
            id: contentFlick
            anchors.top: statusBar.bottom
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: playerBar.top
            contentWidth: width
            contentHeight: Math.max(height, categoryColumn.implicitHeight + 18)
            clip: true
            boundsBehavior: Flickable.StopAtBounds
            flickDeceleration: 1800

            Column {
                id: categoryColumn
                width: contentFlick.width
                y: 10
                spacing: 12

                Text {
                    visible: root.categories.length === 0
                    width: parent.width - 28
                    x: 14
                    text: root.backendOnline ? "Noch keine Kategorien eingerichtet." : "MuPiBox verbindet sich …"
                    color: root.mutedColor
                    font.pixelSize: 13
                    font.family: root.uiFont
                }

                Repeater {
                    model: root.categories

                    delegate: Item {
                        width: categoryColumn.width
                        height: visibleItems.length ? categoryContent.implicitHeight : 0
                        visible: visibleItems.length > 0
                        property var categoryData: modelData
                        property var visibleItems: root.categoryItems(categoryData)

                        Column {
                            id: categoryContent
                            width: parent.width
                            spacing: 5

                            Text {
                                x: 14
                                width: parent.width - 28
                                text: root.localized(categoryData.labels, categoryData.id)
                                color: root.textColor
                                font.pixelSize: root.largeUI ? 28 : 20
                                font.family: root.uiFont
                                font.bold: true
                                elide: Text.ElideRight
                            }

                            Repeater {
                                model: [{"items": visibleItems}]

                                delegate: Item {
                                    width: categoryContent.width
                                    height: root.largeUI ? 148 : 110
                                    property var rowData: modelData

                                    Text {
                                        visible: false
                                        x: 14
                                        y: 0
                                        width: parent.width - 120
                                        text: root.localized(rowData.labels, rowData.id)
                                        color: root.textColor
                                        font.pixelSize: root.largeUI ? 17 : 12
                                        font.family: root.uiFont
                                        font.bold: true
                                        elide: Text.ElideRight
                                    }

                                    Text {
                                        visible: false
                                        anchors.right: parent.right
                                        anchors.rightMargin: 14
                                        y: 2
                                        text: String((rowData.items || []).length) + " Inhalte"
                                        color: root.mutedColor
                                        font.pixelSize: root.largeUI ? 12 : 9
                                        font.family: root.uiFont
                                    }

                                    Flickable {
                                        id: mediaFlick
                                        x: 0
                                        y: 0
                                        width: parent.width
                                        height: root.largeUI ? 148 : 110
                                        contentWidth: Math.max(width, mediaRow.width + 28)
                                        contentHeight: height
                                        clip: true
                                        boundsBehavior: Flickable.StopAtBounds
                                        flickDeceleration: 1800

                                        Row {
                                            id: mediaRow
                                            x: 14
                                            spacing: root.largeUI ? 14 : 10

                                            Text {
                                                visible: (rowData.items || []).length === 0
                                                width: 180
                                                height: 100
                                                text: "Noch keine Inhalte."
                                                color: root.mutedColor
                                                font.pixelSize: 12
                                                font.family: root.uiFont
                                            }

                                            Repeater {
                                                model: rowData.items || []

                                                delegate: Rectangle {
                                                    width: root.largeUI ? 176 : 132
                                                    height: root.largeUI ? 144 : 106
                                                    radius: root.retroTheme ? 0 : (root.largeUI ? 14 : 12)
                                                    color: mediaTouch.pressed ? root.panelColor : root.cardColor
                                                    border.width: root.playerState.folder_id === String(mediaData.command && mediaData.command.folder_id || "") ? 2 : 1
                                                    border.color: root.playerState.folder_id === String(mediaData.command && mediaData.command.folder_id || "") ? root.accentColor : "transparent"
                                                    clip: true
                                                    property var mediaData: modelData

                                                    Rectangle {
                                                        id: coverBackground
                                                        anchors.left: parent.left
                                                        anchors.right: parent.right
                                                        anchors.top: parent.top
                                                        height: root.largeUI ? 104 : 76
                                                        color: index % 3 === 0 ? "#50405e" : (index % 3 === 1 ? "#285a57" : "#755141")

                                                        Image {
                                                            id: mediaCover
                                                            anchors.fill: parent
                                                            source: root.coverUrl(mediaData.cover || "")
                                                            fillMode: Image.PreserveAspectCrop
                                                            asynchronous: true
                                                            cache: true
                                                            visible: source !== ""
                                                        }

                                                        Text {
                                                            anchors.centerIn: parent
                                                            visible: mediaCover.source === ""
                                                            text: "♫"
                                                            color: "#e8d9ef"
                                                            font.pixelSize: 34
                                                            font.family: root.uiFont
                                                        }
                                                    }

                                                    Text {
                                                        x: 9
                                                        y: root.largeUI ? 110 : 80
                                                        width: parent.width - 18
                                                        text: mediaData.title || "Ohne Titel"
                                                        color: root.textColor
                                                        font.pixelSize: root.largeUI ? 16 : 11
                                                        font.family: root.uiFont
                                                        font.bold: true
                                                        elide: Text.ElideRight
                                                    }

                                                    Text {
                                                        x: 9
                                                        y: root.largeUI ? 130 : 94
                                                        width: parent.width - 18
                                                        text: mediaData.subtitle || mediaData.kind || ""
                                                        color: root.mutedColor
                                                        font.pixelSize: root.largeUI ? 11 : 8
                                                        font.family: root.uiFont
                                                        elide: Text.ElideRight
                                                    }

                                                    MouseArea {
                                                        id: mediaTouch
                                                        anchors.fill: parent
                                                        enabled: root.backendOnline && !root.pending && !!mediaData.command
                                                        onClicked: root.command(mediaData.command)
                                                    }
                                                }
                                            }
                                        }
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }

        Rectangle {
            id: playerBar
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            height: root.largeUI ? 104 : 88
            color: root.panelColor
            border.color: root.lineColor

            Rectangle {
                id: miniArt
                x: 10
                anchors.verticalCenter: parent.verticalCenter
                width: root.largeUI ? 66 : 54
                height: root.largeUI ? 66 : 54
                radius: root.retroTheme ? 0 : (root.largeUI ? 13 : 10)
                color: "#594568"
                clip: true

                Image {
                    id: nowCover
                    anchors.fill: parent
                    source: root.currentCoverUrl()
                    fillMode: Image.PreserveAspectCrop
                    asynchronous: true
                    visible: source !== ""
                }

                Text {
                    anchors.centerIn: parent
                    visible: nowCover.source === ""
                    text: "♫"
                    color: "#e8d9ef"
                    font.pixelSize: 28
                    font.family: root.uiFont
                }
            }

            Column {
                x: root.largeUI ? 86 : 76
                anchors.verticalCenter: parent.verticalCenter
                width: 205
                spacing: 3

                Text {
                    width: parent.width
                    text: root.currentDisplayTitle()
                    color: root.textColor
                    font.pixelSize: root.largeUI ? 16 : 13
                    font.family: root.uiFont
                    font.bold: true
                    elide: Text.ElideRight
                }
                Text {
                    width: parent.width
                    text: root.currentDisplaySubtitle()
                    color: root.mutedColor
                    font.pixelSize: 9
                    font.family: root.uiFont
                    elide: Text.ElideRight
                }
                Text {
                    width: parent.width
                    text: root.currentDisplayStatusLine()
                    color: root.mutedColor
                    font.pixelSize: 9
                    font.family: root.uiFont
                    elide: Text.ElideRight
                }
            }

            Row {
                id: transport
                x: 292
                anchors.verticalCenter: parent.verticalCenter
                spacing: 7

                Rectangle {
                    width: root.largeUI ? 50 : 42; height: root.largeUI ? 50 : 42; radius: root.retroTheme ? 0 : width / 2
                    color: previousTouch.pressed ? root.lineColor : root.panelColor
                    opacity: root.transportEnabled() ? 1 : 0.4
                    Text { anchors.centerIn: parent; text: "❮❮"; color: root.textColor; font.pixelSize: 14 }
                    MouseArea {
                        id: previousTouch
                        anchors.fill: parent
                        enabled: root.transportEnabled()
                        onClicked: root.onPreviousTapped()
                    }
                }

                Rectangle {
                    width: root.largeUI ? 58 : 48; height: root.largeUI ? 58 : 48; radius: root.retroTheme ? 0 : width / 2
                    color: toggleTouch.pressed ? root.accentPressedColor : root.accentColor
                    opacity: root.transportEnabled() ? 1 : 0.4
                    Text {
                        anchors.centerIn: parent
                        text: root.toggleLabel()
                        color: root.accentTextColor
                        font.pixelSize: 18
                        font.family: root.uiFont
                    }
                    MouseArea {
                        id: toggleTouch
                        anchors.fill: parent
                        enabled: root.transportEnabled()
                        onClicked: root.onToggleTapped()
                    }
                }

                Rectangle {
                    width: root.largeUI ? 50 : 42; height: root.largeUI ? 50 : 42; radius: root.retroTheme ? 0 : width / 2
                    color: nextTouch.pressed ? root.lineColor : root.panelColor
                    opacity: root.transportEnabled() ? 1 : 0.4
                    Text { anchors.centerIn: parent; text: "❯❯"; color: root.textColor; font.pixelSize: 14 }
                    MouseArea {
                        id: nextTouch
                        anchors.fill: parent
                        enabled: root.transportEnabled()
                        onClicked: root.onNextTapped()
                    }
                }
            }

            Item {
                x: 460
                y: root.largeUI ? 20 : 12
                width: 330
                height: 64

                Row {
                    width: parent.width
                    height: 27
                    spacing: 6

                    Text {
                        width: 28
                        anchors.verticalCenter: parent.verticalCenter
                        text: root.formatTime(root.currentPositionSeconds())
                        color: root.mutedColor
                        font.pixelSize: 8
                        font.family: root.uiFont
                    }

                    Rectangle {
                        id: seekTrack
                        width: 248
                        height: 4
                        anchors.verticalCenter: parent.verticalCenter
                        radius: root.retroTheme ? 0 : 2
                        color: root.lineColor

                        Rectangle {
                            height: parent.height
                            radius: parent.radius
                            color: root.accentColor
                            width: parent.width * Math.min(1, root.currentPositionSeconds() / Math.max(1, root.currentDurationSeconds()))
                        }

                        MouseArea {
                            anchors.fill: parent
                            anchors.margins: -10
                            enabled: root.seekEnabled()
                            onReleased: function(mouse) {
                                var localX = Math.max(0, Math.min(seekTrack.width, mouse.x + 10))
                                root.onSeekTapped(localX, seekTrack.width)
                            }
                        }
                    }

                    Text {
                        width: 36
                        anchors.verticalCenter: parent.verticalCenter
                        horizontalAlignment: Text.AlignRight
                        text: root.formatTime(root.currentDurationSeconds())
                        color: root.mutedColor
                        font.pixelSize: 8
                        font.family: root.uiFont
                    }
                }

                Row {
                    y: 34
                    width: parent.width
                    height: 28
                    spacing: 7

                    Text {
                        width: 18
                        anchors.verticalCenter: parent.verticalCenter
                        text: "🔊"
                        color: root.mutedColor
                        font.pixelSize: 11
                        font.family: root.uiFont
                    }

                    Rectangle {
                        id: volumeTrack
                        width: 270
                        height: 4
                        anchors.verticalCenter: parent.verticalCenter
                        radius: root.retroTheme ? 0 : 2
                        color: root.lineColor

                        Rectangle {
                            height: parent.height
                            radius: parent.radius
                            color: root.accentColor
                            width: parent.width * Math.min(1, root.currentVolume() / Math.max(1, root.currentMaxVolume()))
                        }

                        MouseArea {
                            anchors.fill: parent
                            anchors.margins: -10
                            enabled: root.backendOnline
                            onReleased: function(mouse) {
                                var localX = Math.max(0, Math.min(volumeTrack.width, mouse.x + 10))
                                root.onVolumeTapped(localX, volumeTrack.width)
                            }
                        }
                    }

                    Text {
                        width: 28
                        anchors.verticalCenter: parent.verticalCenter
                        horizontalAlignment: Text.AlignRight
                        text: String(root.currentVolume())
                        color: root.mutedColor
                        font.pixelSize: 9
                        font.family: root.uiFont
                    }
                }
            }
        }

        Rectangle {
            id: wifiPanel
            visible: root.wifiPanelVisible
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: statusBar.bottom
            anchors.bottom: parent.bottom
            color: root.backgroundColor
            z: 85

            Text { x: 20; y: 12; text: "WLAN"; color: root.textColor; font.pixelSize: 26; font.family: root.uiFont; font.bold: true }
            Rectangle {
                x: 650; y: 8; width: 58; height: 44; radius: root.retroTheme ? 0 : 11; color: wifiScanTouch.pressed ? root.lineColor : root.panelColor
                Text {
                    anchors.centerIn: parent; text: "↻"; color: root.textColor; font.pixelSize: 25; font.family: root.uiFont
                    NumberAnimation on rotation { running: root.wifiBusy; loops: Animation.Infinite; from: 0; to: 360; duration: 700 }
                }
                MouseArea { id: wifiScanTouch; anchors.fill: parent; enabled: !root.wifiBusy; onClicked: root.scanWifiNetworks() }
            }
            Rectangle {
                x: 720; y: 8; width: 58; height: 44; radius: root.retroTheme ? 0 : 11; color: wifiCloseTouch.pressed ? root.lineColor : root.panelColor
                Text { anchors.centerIn: parent; text: "×"; color: root.textColor; font.pixelSize: 27; font.family: root.uiFont }
                MouseArea { id: wifiCloseTouch; anchors.fill: parent; onClicked: { Qt.inputMethod.hide(); root.wifiPassword = ""; root.wifiSelectedSecurity = ""; root.wifiSelectedInterface = ""; root.wifiPanelVisible = false } }
            }

            Rectangle {
                x: 18; y: 62; width: 330; height: 354; radius: root.retroTheme ? 0 : 14; color: root.cardColor; border.color: root.lineColor
                Text { visible: root.wifiBusy; anchors.centerIn: parent; text: "Suche …"; color: root.mutedColor; font.pixelSize: 20; font.family: root.uiFont }
                Text { visible: !root.wifiBusy && root.wifiNetworks.length === 0; anchors.centerIn: parent; text: "Keine Netze gefunden"; color: root.mutedColor; font.pixelSize: 17; font.family: root.uiFont }
                ListView {
                    anchors.fill: parent; anchors.margins: 8; clip: true; spacing: 5; model: root.wifiNetworks
                    delegate: Rectangle {
                        property var networkData: modelData
                        width: ListView.view.width; height: 54; radius: root.retroTheme ? 0 : 10
                        color: root.wifiSelectedSSID === networkData.ssid ? root.accentColor : (wifiNetworkTouch.pressed ? root.panelColor : root.backgroundColor)
                        border.color: networkData.connected ? "#43c86a" : root.lineColor
                        Text { x: 12; y: 7; width: 225; text: (networkData.connected ? "✓ " : "") + networkData.ssid; color: root.wifiSelectedSSID === networkData.ssid ? root.accentTextColor : root.textColor; font.pixelSize: 17; font.family: root.uiFont; font.bold: true; elide: Text.ElideRight }
                        Text { x: 12; y: 31; width: 225; text: (networkData.security || "Offen") + (networkData.interface ? " · " + networkData.interface : ""); color: root.wifiSelectedSSID === networkData.ssid ? root.accentTextColor : root.mutedColor; font.pixelSize: 10; font.family: root.uiFont; elide: Text.ElideRight }
                        Row {
                            anchors.right: parent.right; anchors.rightMargin: 10; anchors.bottom: parent.bottom; anchors.bottomMargin: 10; spacing: 2
                            Repeater { model: 4; delegate: Rectangle { width: 5; height: 6 + index * 4; anchors.bottom: parent.bottom; color: index < root.signalLevel(networkData.signal_percent) ? root.signalColor(networkData.signal_percent) : "#343b47" } }
                        }
                        MouseArea {
                            id: wifiNetworkTouch
                            anchors.fill: parent
                            onClicked: {
                                root.wifiSelectedSSID = networkData.ssid
                                root.wifiSelectedSecurity = networkData.security || ""
                                root.wifiSelectedInterface = networkData.interface || ""
                                root.wifiPassword = ""
                                if (root.wifiSecurityRequired(root.wifiSelectedSecurity)) wifiPasswordInput.forceActiveFocus()
                                else Qt.inputMethod.hide()
                            }
                        }
                    }
                }
            }

            Item {
                x: 370; y: 62; width: 410; height: 354
                Text { x: 0; y: 0; width: parent.width; text: root.wifiSelectedSSID === "" ? "WLAN auswählen" : root.wifiSelectedSSID; color: root.textColor; font.pixelSize: 20; font.family: root.uiFont; font.bold: true; elide: Text.ElideRight }
                Text { x: 0; y: 30; width: parent.width; text: root.wifiSelectedSSID === "" ? "" : (root.wifiSelectedSecurity || "Offen") + (root.wifiSelectedInterface ? " · " + root.wifiSelectedInterface : ""); color: root.mutedColor; font.pixelSize: 12; font.family: root.uiFont; elide: Text.ElideRight }
                Rectangle {
                    x: 0; y: 54; width: parent.width; height: 54; radius: root.retroTheme ? 0 : 9; color: root.cardColor; border.color: wifiPasswordInput.activeFocus ? root.accentColor : root.lineColor
                    opacity: root.wifiSelectedSSID === "" || !root.wifiSecurityRequired(root.wifiSelectedSecurity) ? 0.45 : 1
                    TextInput {
                        id: wifiPasswordInput
                        anchors.fill: parent
                        anchors.margins: 12
                        text: root.wifiPassword
                        onTextChanged: if (text !== root.wifiPassword) root.wifiPassword = text.slice(0,63)
                        echoMode: TextInput.Password
                        inputMethodHints: Qt.ImhSensitiveData | Qt.ImhNoPredictiveText
                        color: root.textColor
                        font.pixelSize: 20
                        font.family: root.uiFont
                        enabled: root.wifiSelectedSSID !== "" && root.wifiSecurityRequired(root.wifiSelectedSecurity)
                        clip: true
                        Keys.onReturnPressed: root.connectSelectedWifi()
                    }
                    Text {
                        visible: wifiPasswordInput.text === "" && !wifiPasswordInput.activeFocus
                        anchors.fill: parent
                        anchors.margins: 12
                        text: root.wifiSelectedSSID !== "" && !root.wifiSecurityRequired(root.wifiSelectedSecurity) ? "Kein Passwort erforderlich" : "Passwort"
                        color: root.mutedColor
                        font.pixelSize: 17
                        font.family: root.uiFont
                        verticalAlignment: Text.AlignVCenter
                    }
                }
                Text { x: 0; y: 122; width: parent.width; text: root.wifiSecurityRequired(root.wifiSelectedSecurity) ? "Zum Eingeben das Passwortfeld antippen." : "Offenes WLAN – direkte Verbindung möglich."; color: root.mutedColor; font.pixelSize: 13; font.family: root.uiFont; wrapMode: Text.WordWrap }
                Rectangle {
                    x: 0; y: 170; width: parent.width; height: 58; radius: root.retroTheme ? 0 : 10; color: connectWifiTouch.pressed ? root.accentPressedColor : root.accentColor; opacity: root.wifiSelectedSSID === "" || root.wifiBusy ? 0.45 : 1
                    Text { anchors.centerIn: parent; text: root.wifiBusy ? "Bitte warten …" : "Verbinden"; color: root.accentTextColor; font.pixelSize: 18; font.family: root.uiFont; font.bold: true }
                    MouseArea { id: connectWifiTouch; anchors.fill: parent; enabled: root.wifiSelectedSSID !== "" && !root.wifiBusy; onClicked: root.connectSelectedWifi() }
                }
            }
        }

        Rectangle {
            id: playerViewLayer
            // !showSpotifyBar: an active Spotify session must not stay
            // hidden behind a stale local-folder overlay left open from an
            // earlier session (the local queue can still be non-empty even
            // though Spotify, not the local player, is now the active
            // source) -- the persistent bottom playerBar already renders
            // Spotify's controls once this overlay steps aside.
            visible: root.largeUI && root.playerView && (root.playerState.queue || []).length > 0 && !root.showSpotifyBar
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: statusBar.bottom
            anchors.bottom: parent.bottom
            color: root.backgroundColor
            z: 60

            Rectangle {
                x: 14; y: 12; width: 60; height: 52; radius: root.retroTheme ? 0 : 13
                color: playerBackTouch.pressed ? root.lineColor : root.panelColor
                Text { anchors.centerIn: parent; text: "←"; color: root.textColor; font.pixelSize: 30; font.family: root.uiFont; font.bold: true }
                MouseArea { id: playerBackTouch; anchors.fill: parent; onClicked: root.playerView = false }
            }

            Text {
                x: 90; y: 13; width: 560; height: 32
                text: { var track = root.currentTrack(); return track ? track.title : root.playerState.folder }
                color: root.textColor; font.pixelSize: 25; font.family: root.uiFont; font.bold: true; elide: Text.ElideRight
            }
            Text {
                x: 90; y: 43; width: 560; height: 22
                text: root.playerState.folder || ""
                color: root.mutedColor; font.pixelSize: 14; font.family: root.uiFont; elide: Text.ElideRight
            }
            Text {
                anchors.right: parent.right; anchors.rightMargin: 18; y: 20
                text: String(Number(root.playerState.index || 0) + 1) + " / " + String((root.playerState.queue || []).length)
                color: root.mutedColor; font.pixelSize: 15
            }

            Rectangle {
                x: 24; y: 78; width: 300; height: 300; radius: root.retroTheme ? 0 : 18; color: "#594568"; clip: true
                Image { id: playerCover; anchors.fill: parent; source: root.coverUrl(root.playerState.cover || ""); fillMode: Image.PreserveAspectCrop; asynchronous: true; visible: source !== "" }
                Text { anchors.centerIn: parent; visible: playerCover.source === ""; text: "♫"; color: "#e8d9ef"; font.pixelSize: 82; font.family: root.uiFont }
            }

            Item {
                x: 350; y: 78; width: 426; height: 320

                Text { x: 0; y: 0; text: root.formatTime(root.playerState.position); color: root.textColor; font.pixelSize: 14 }
                Text { anchors.right: parent.right; y: 0; text: root.formatTime(root.playerState.duration); color: root.textColor; font.pixelSize: 14 }
                Rectangle {
                    id: playerSeekTrack
                    x: 0; y: 28; width: parent.width; height: 10; radius: root.retroTheme ? 0 : 5; color: root.lineColor
                    Rectangle { height: parent.height; radius: root.retroTheme ? 0 : 5; color: root.accentColor; width: parent.width * Math.min(1, Number(root.playerState.position || 0) / Math.max(1, Number(root.playerState.duration || 0))) }
                    MouseArea {
                        anchors.fill: parent; anchors.margins: -12
                        enabled: Number(root.playerState.duration || 0) > 0
                        onReleased: function(mouse) {
                            var localX = Math.max(0, Math.min(playerSeekTrack.width, mouse.x + 12))
                            root.command({"action": "seek", "value": Math.round(localX / playerSeekTrack.width * Number(root.playerState.duration || 0))})
                        }
                    }
                }

                Row {
                    x: 0; y: 62; spacing: 12
                    Rectangle {
                        width: 126; height: 82; radius: root.retroTheme ? 0 : 17; color: backTenTouch.pressed ? root.lineColor : root.panelColor
                        Text { anchors.centerIn: parent; text: "−10"; color: root.textColor; font.pixelSize: 28; font.family: root.uiFont; font.bold: true }
                        MouseArea { id: backTenTouch; anchors.fill: parent; onClicked: root.command({"action":"seek","value":Math.max(0,Number(root.playerState.position||0)-10)}) }
                    }
                    Rectangle {
                        width: 126; height: 82; radius: root.retroTheme ? 0 : 17; color: playLargeTouch.pressed ? root.accentPressedColor : root.accentColor
                        Text { anchors.centerIn: parent; text: root.playerState.state === "playing" ? "Ⅱ" : "▶"; color: root.accentTextColor; font.pixelSize: 35; font.family: root.uiFont; font.bold: true }
                        MouseArea { id: playLargeTouch; anchors.fill: parent; onClicked: root.command({"action":"toggle"}) }
                    }
                    Rectangle {
                        width: 126; height: 82; radius: root.retroTheme ? 0 : 17; color: forwardTenTouch.pressed ? root.lineColor : root.panelColor
                        Text { anchors.centerIn: parent; text: "+10"; color: root.textColor; font.pixelSize: 28; font.family: root.uiFont; font.bold: true }
                        MouseArea { id: forwardTenTouch; anchors.fill: parent; onClicked: root.command({"action":"seek","value":Math.min(Number(root.playerState.duration||0),Number(root.playerState.position||0)+10)}) }
                    }
                }

                Row {
                    x: 0; y: 162; spacing: 18
                    Rectangle {
                        width: 195; height: 64; radius: root.retroTheme ? 0 : 15; color: previousLargeTouch.pressed ? root.lineColor : root.panelColor
                        Text { anchors.centerIn: parent; text: "❮❮"; color: root.textColor; font.pixelSize: 25 }
                        MouseArea { id: previousLargeTouch; anchors.fill: parent; onClicked: root.command({"action":"previous"}) }
                    }
                    Rectangle {
                        width: 195; height: 64; radius: root.retroTheme ? 0 : 15; color: nextLargeTouch.pressed ? root.lineColor : root.panelColor
                        Text { anchors.centerIn: parent; text: "❯❯"; color: root.textColor; font.pixelSize: 25 }
                        MouseArea { id: nextLargeTouch; anchors.fill: parent; onClicked: root.command({"action":"next"}) }
                    }
                }

                Row {
                    x: 0; y: 250; width: parent.width; height: 52; spacing: 12
                    Text { width: 28; anchors.verticalCenter: parent.verticalCenter; text: "🔊"; color: root.textColor; font.pixelSize: 20 }
                    Rectangle {
                        id: playerVolumeTrack
                        width: 332; height: 10; anchors.verticalCenter: parent.verticalCenter; radius: root.retroTheme ? 0 : 5; color: root.lineColor
                        Rectangle { height: parent.height; radius: root.retroTheme ? 0 : 5; color: root.accentColor; width: parent.width * Math.min(1, Number(root.playerState.volume || 0) / Math.max(1, Number(root.playerState.max_volume || 60))) }
                        MouseArea {
                            anchors.fill: parent; anchors.margins: -14
                            onReleased: function(mouse) {
                                var localX = Math.max(0, Math.min(playerVolumeTrack.width, mouse.x + 14))
                                root.command({"action":"volume","value":Math.round(localX / playerVolumeTrack.width * Number(root.playerState.max_volume || 60))})
                            }
                        }
                    }
                    Text { width: 42; anchors.verticalCenter: parent.verticalCenter; text: String(root.playerState.volume || 0); color: root.textColor; font.pixelSize: 16 }
                }
            }
        }
        Rectangle {
            visible: root.transientMessage !== ""
            anchors.left: parent.left
            anchors.leftMargin: 12
            anchors.right: parent.right
            anchors.rightMargin: 12
            anchors.bottom: playerBar.top
            anchors.bottomMargin: 10
            height: 42
            radius: root.retroTheme ? 0 : 10
            color: "#682f45"
            border.color: "#ffacc9"
            z: 80
            Text {
                anchors.fill: parent
                anchors.margins: 10
                text: root.transientMessage
                color: root.textColor
                font.pixelSize: 12
                font.family: root.uiFont
                verticalAlignment: Text.AlignVCenter
                elide: Text.ElideRight
            }
        }

        Rectangle {
            id: splash
            anchors.fill: parent
            z: 100
            color: "#250735"
            opacity: 1
            visible: opacity > 0.01

            Image {
                anchors.fill: parent
                source: "assets/mupibox-startscreen.jpg"
                fillMode: Image.PreserveAspectCrop
                smooth: true
            }

            Behavior on opacity { NumberAnimation { duration: 450 } }
            Timer {
                interval: 2300
                running: true
                repeat: false
                onTriggered: splash.opacity = 0
            }
        }
    }

    InputPanel {
        id: virtualKeyboard
        z: 200
        x: 0
        width: root.width
        y: active ? root.height - height : root.height
        Behavior on y { NumberAnimation { duration: 160; easing.type: Easing.OutQuad } }
    }
}
