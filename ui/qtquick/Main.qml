import QtQuick 2.15
import QtQuick.Window 2.15

Window {
    id: root
    visible: true
    visibility: Window.FullScreen
    color: "#0b0d11"
    title: "MuPiBox"

    property real uiScale: Math.min(width / 800, height / 480)
    property var apiCandidates: ["http://127.0.0.1:8090", "http://127.0.0.1:8080"]
    property int apiIndex: 0
    property string apiBase: apiCandidates[apiIndex]
    property bool backendOnline: false
    property bool pending: false
    property bool statusRequestRunning: false
    property string backendState: "Verbinde …"
    property string clockText: "--:--"
    property var categories: []
    property string homeSignature: ""
    property string uiSize: "normal"
    property bool largeUI: uiSize === "large"
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
    property bool adminHintVisible: false

    function localized(labels, fallback) {
        if (!labels) return fallback || ""
        return labels.de || labels.en || fallback || ""
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
                backendState = "Verbunden"
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
            backendState = "Offline"
        })
    }

    function refreshInfo() {
        requestJson("GET", "/api/info", null, function(data) {
            uiSize = data && data.display && data.display.ui_size ? data.display.ui_size : "normal"
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
            wifiStatus = data && data.wifi ? data.wifi : {"connected": false, "quality_percent": 0}
            batteryStatus = data && data.battery ? data.battery : {"available": false, "percent": 0, "charging": false}
        }, function() {
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
            backendState = "Offline"
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
            pending = false
        }, function() {
            pending = false
            showMessage("Befehl konnte nicht ausgeführt werden.")
        })
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
        }, function() {
            backendOnline = false
            backendState = "Offline"
        })
    }

    Timer { interval: 750; running: true; repeat: true; onTriggered: root.refreshStatus() }
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
            color: "#0b0d11"
        }

        Rectangle {
            id: statusBar
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: parent.top
            height: root.largeUI ? 42 : 34
            color: "#0a0c10"
            border.color: "#20242c"

            Row {
                anchors.left: parent.left
                anchors.leftMargin: 10
                anchors.verticalCenter: parent.verticalCenter
                spacing: 7

                Rectangle {
                    width: root.largeUI ? 28 : 22
                    height: root.largeUI ? 28 : 22
                    radius: root.largeUI ? 9 : 7
                    color: "#f59aca"
                    Text {
                        anchors.centerIn: parent
                        text: "m"
                        color: "#312331"
                        font.pixelSize: root.largeUI ? 19 : 15
                        font.bold: true
                    }
                }

                Text {
                    anchors.verticalCenter: parent.verticalCenter
                    text: "MuPiBox"
                    color: "#f7f7fa"
                    font.pixelSize: root.largeUI ? 16 : 13
                    font.bold: true
                }
            }

            Row {
                anchors.right: parent.right
                anchors.rightMargin: 10
                anchors.verticalCenter: parent.verticalCenter
                spacing: 12

                Text {
                    text: root.backendState
                    color: root.backendOnline ? "#9aa3b2" : "#f59aca"
                    font.pixelSize: 10
                }
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
                                radius: 1
                                color: index < root.signalLevel(root.wifiStatus.quality_percent) ? root.signalColor(root.wifiStatus.quality_percent) : "#343b47"
                            }
                        }
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
                        radius: 3
                        color: "transparent"
                        border.width: 2
                        border.color: "#7d8795"

                        Rectangle {
                            x: 2
                            anchors.verticalCenter: parent.verticalCenter
                            width: root.batteryStatus.available ? (parent.width - 4) * Math.max(0, Math.min(100, Number(root.batteryStatus.percent || 0))) / 100 : 0
                            height: parent.height - 4
                            radius: 1
                            color: root.batteryColor(root.batteryStatus.percent)
                        }

                        Text {
                            visible: root.batteryStatus.available && root.batteryStatus.charging
                            anchors.centerIn: parent
                            text: "⚡"
                            color: "#ffffff"
                            font.pixelSize: 12
                            style: Text.Outline
                            styleColor: "#222222"
                        }
                    }

                    Rectangle {
                        x: 29
                        y: 6
                        width: 3
                        height: 6
                        radius: 1
                        color: "#7d8795"
                    }
                }

                Rectangle {
                    width: 48
                    height: 28
                    radius: 7
                    color: clockTouch.pressed ? "#202632" : "transparent"

                    Text {
                        anchors.centerIn: parent
                        text: root.clockText
                        color: "#d8dce4"
                        font.pixelSize: 11
                    }

                    Rectangle {
                        anchors.left: parent.left
                        anchors.leftMargin: 4
                        anchors.bottom: parent.bottom
                        anchors.bottomMargin: 1
                        height: 2
                        width: clockTouch.pressed ? (parent.width - 8) * Math.min(1, clockTouch.heldMs / 5000) : 0
                        radius: 1
                        color: "#f59aca"
                    }

                    MouseArea {
                        id: clockTouch
                        anchors.fill: parent
                        property int heldMs: 0
                        onPressed: {
                            heldMs = 0
                            holdProgress.start()
                        }
                        onReleased: {
                            holdProgress.stop()
                            heldMs = 0
                        }
                        onCanceled: {
                            holdProgress.stop()
                            heldMs = 0
                        }
                    }

                    Timer {
                        id: holdProgress
                        interval: 50
                        repeat: true
                        onTriggered: {
                            clockTouch.heldMs += interval
                            if (clockTouch.heldMs >= 5000) {
                                stop()
                                root.adminHintVisible = true
                                clockTouch.heldMs = 0
                            }
                        }
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
                    color: "#9aa3b2"
                    font.pixelSize: 13
                }

                Repeater {
                    model: root.categories

                    delegate: Item {
                        width: categoryColumn.width
                        height: categoryContent.implicitHeight
                        property var categoryData: modelData

                        Column {
                            id: categoryContent
                            width: parent.width
                            spacing: 5

                            Text {
                                x: 14
                                width: parent.width - 28
                                text: root.localized(categoryData.labels, categoryData.id)
                                color: "#f7f7fa"
                                font.pixelSize: root.largeUI ? 28 : 20
                                font.bold: true
                                elide: Text.ElideRight
                            }

                            Repeater {
                                model: categoryData.rows || []

                                delegate: Item {
                                    width: categoryContent.width
                                    height: root.largeUI ? 178 : 132
                                    property var rowData: modelData

                                    Text {
                                        x: 14
                                        y: 0
                                        width: parent.width - 120
                                        text: root.localized(rowData.labels, rowData.id)
                                        color: "#d8dce4"
                                        font.pixelSize: root.largeUI ? 17 : 12
                                        font.bold: true
                                        elide: Text.ElideRight
                                    }

                                    Text {
                                        anchors.right: parent.right
                                        anchors.rightMargin: 14
                                        y: 2
                                        text: String((rowData.items || []).length) + " Inhalte"
                                        color: "#9aa3b2"
                                        font.pixelSize: root.largeUI ? 12 : 9
                                    }

                                    Flickable {
                                        id: mediaFlick
                                        x: 0
                                        y: root.largeUI ? 30 : 22
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
                                                color: "#9aa3b2"
                                                font.pixelSize: 12
                                            }

                                            Repeater {
                                                model: rowData.items || []

                                                delegate: Rectangle {
                                                    width: root.largeUI ? 176 : 132
                                                    height: root.largeUI ? 144 : 106
                                                    radius: root.largeUI ? 14 : 12
                                                    color: mediaTouch.pressed ? "#202632" : "#141820"
                                                    border.width: root.playerState.folder_id === String(mediaData.command && mediaData.command.folder_id || "") ? 2 : 1
                                                    border.color: root.playerState.folder_id === String(mediaData.command && mediaData.command.folder_id || "") ? "#f59aca" : "transparent"
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
                                                        }
                                                    }

                                                    Text {
                                                        x: 9
                                                        y: root.largeUI ? 110 : 80
                                                        width: parent.width - 18
                                                        text: mediaData.title || "Ohne Titel"
                                                        color: "#f7f7fa"
                                                        font.pixelSize: root.largeUI ? 16 : 11
                                                        font.bold: true
                                                        elide: Text.ElideRight
                                                    }

                                                    Text {
                                                        x: 9
                                                        y: root.largeUI ? 130 : 94
                                                        width: parent.width - 18
                                                        text: mediaData.subtitle || mediaData.kind || ""
                                                        color: "#9aa3b2"
                                                        font.pixelSize: root.largeUI ? 11 : 8
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
            color: "#11151c"
            border.color: "#2a313d"

            Rectangle {
                id: miniArt
                x: 10
                anchors.verticalCenter: parent.verticalCenter
                width: root.largeUI ? 66 : 54
                height: root.largeUI ? 66 : 54
                radius: root.largeUI ? 13 : 10
                color: "#594568"
                clip: true

                Image {
                    id: nowCover
                    anchors.fill: parent
                    source: root.coverUrl(root.playerState.cover || "")
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
                }
            }

            Column {
                x: root.largeUI ? 86 : 76
                anchors.verticalCenter: parent.verticalCenter
                width: 205
                spacing: 3

                Text {
                    width: parent.width
                    text: {
                        var track = root.currentTrack()
                        return track ? track.title : "Such dir etwas aus"
                    }
                    color: "#f7f7fa"
                    font.pixelSize: root.largeUI ? 16 : 13
                    font.bold: true
                    elide: Text.ElideRight
                }
                Text {
                    width: parent.width
                    text: root.playerState.folder || "Deine Medien warten auf dich."
                    color: "#9aa3b2"
                    font.pixelSize: 9
                    elide: Text.ElideRight
                }
                Text {
                    width: parent.width
                    text: {
                        var queue = root.playerState.queue || []
                        if (!queue.length) return ""
                        var states = {"playing": "Wiedergabe", "paused": "Pausiert", "stopped": "Gestoppt", "error": "Fehler"}
                        return String(Number(root.playerState.index || 0) + 1) + " / " + String(queue.length) + " · " + (states[root.playerState.state] || "")
                    }
                    color: "#9aa3b2"
                    font.pixelSize: 9
                    elide: Text.ElideRight
                }
            }

            Row {
                id: transport
                x: 292
                anchors.verticalCenter: parent.verticalCenter
                spacing: 7

                Rectangle {
                    width: root.largeUI ? 50 : 42; height: root.largeUI ? 50 : 42; radius: width / 2
                    color: previousTouch.pressed ? "#303745" : "#202632"
                    opacity: root.backendOnline && (root.playerState.queue || []).length ? 1 : 0.4
                    Text { anchors.centerIn: parent; text: "❮❮"; color: "#f7f7fa"; font.pixelSize: 14 }
                    MouseArea {
                        id: previousTouch
                        anchors.fill: parent
                        enabled: root.backendOnline && (root.playerState.queue || []).length > 0
                        onClicked: root.command({"action": "previous"})
                    }
                }

                Rectangle {
                    width: root.largeUI ? 58 : 48; height: root.largeUI ? 58 : 48; radius: width / 2
                    color: toggleTouch.pressed ? "#ffb8d8" : "#f59aca"
                    opacity: root.backendOnline && (root.playerState.queue || []).length ? 1 : 0.4
                    Text {
                        anchors.centerIn: parent
                        text: root.playerState.state === "playing" ? "Ⅱ" : "▶"
                        color: "#2c1f2e"
                        font.pixelSize: 18
                    }
                    MouseArea {
                        id: toggleTouch
                        anchors.fill: parent
                        enabled: root.backendOnline && (root.playerState.queue || []).length > 0
                        onClicked: root.command({"action": "toggle"})
                    }
                }

                Rectangle {
                    width: root.largeUI ? 50 : 42; height: root.largeUI ? 50 : 42; radius: width / 2
                    color: nextTouch.pressed ? "#303745" : "#202632"
                    opacity: root.backendOnline && (root.playerState.queue || []).length ? 1 : 0.4
                    Text { anchors.centerIn: parent; text: "❯❯"; color: "#f7f7fa"; font.pixelSize: 14 }
                    MouseArea {
                        id: nextTouch
                        anchors.fill: parent
                        enabled: root.backendOnline && (root.playerState.queue || []).length > 0
                        onClicked: root.command({"action": "next"})
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
                        text: root.formatTime(root.playerState.position)
                        color: "#9aa3b2"
                        font.pixelSize: 8
                    }

                    Rectangle {
                        id: seekTrack
                        width: 248
                        height: 4
                        anchors.verticalCenter: parent.verticalCenter
                        radius: 2
                        color: "#303745"

                        Rectangle {
                            height: parent.height
                            radius: parent.radius
                            color: "#f59aca"
                            width: parent.width * Math.min(1, Number(root.playerState.position || 0) / Math.max(1, Number(root.playerState.duration || 0)))
                        }

                        MouseArea {
                            anchors.fill: parent
                            anchors.margins: -10
                            enabled: root.backendOnline && Number(root.playerState.duration || 0) > 0
                            onReleased: function(mouse) {
                                var localX = Math.max(0, Math.min(seekTrack.width, mouse.x + 10))
                                root.command({"action": "seek", "value": Math.round(localX / seekTrack.width * Number(root.playerState.duration || 0))})
                            }
                        }
                    }

                    Text {
                        width: 36
                        anchors.verticalCenter: parent.verticalCenter
                        horizontalAlignment: Text.AlignRight
                        text: root.formatTime(root.playerState.duration)
                        color: "#9aa3b2"
                        font.pixelSize: 8
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
                        color: "#9aa3b2"
                        font.pixelSize: 11
                    }

                    Rectangle {
                        id: volumeTrack
                        width: 270
                        height: 4
                        anchors.verticalCenter: parent.verticalCenter
                        radius: 2
                        color: "#303745"

                        Rectangle {
                            height: parent.height
                            radius: parent.radius
                            color: "#f59aca"
                            width: parent.width * Math.min(1, Number(root.playerState.volume || 0) / Math.max(1, Number(root.playerState.max_volume || 60)))
                        }

                        MouseArea {
                            anchors.fill: parent
                            anchors.margins: -10
                            enabled: root.backendOnline
                            onReleased: function(mouse) {
                                var localX = Math.max(0, Math.min(volumeTrack.width, mouse.x + 10))
                                root.command({"action": "volume", "value": Math.round(localX / volumeTrack.width * Number(root.playerState.max_volume || 60))})
                            }
                        }
                    }

                    Text {
                        width: 28
                        anchors.verticalCenter: parent.verticalCenter
                        horizontalAlignment: Text.AlignRight
                        text: String(root.playerState.volume || 0)
                        color: "#9aa3b2"
                        font.pixelSize: 9
                    }
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
            radius: 10
            color: "#682f45"
            border.color: "#ffacc9"
            z: 80
            Text {
                anchors.fill: parent
                anchors.margins: 10
                text: root.transientMessage
                color: "#f7f7fa"
                font.pixelSize: 12
                verticalAlignment: Text.AlignVCenter
                elide: Text.ElideRight
            }
        }

        Rectangle {
            visible: root.adminHintVisible
            anchors.fill: parent
            color: "#cc0b0d11"
            z: 90

            Rectangle {
                anchors.centerIn: parent
                width: 500
                height: 190
                radius: 18
                color: "#161a22"
                border.color: "#2a313d"

                Column {
                    anchors.fill: parent
                    anchors.margins: 22
                    spacing: 14
                    Text { text: "Administration"; color: "#f7f7fa"; font.pixelSize: 24; font.bold: true }
                    Text {
                        width: parent.width
                        text: "Öffne auf Handy oder Computer:\nhttp://<Box-IP>:8090/admin/"
                        color: "#d8dce4"
                        font.pixelSize: 17
                        wrapMode: Text.WordWrap
                    }
                    Rectangle {
                        width: 130; height: 42; radius: 12
                        color: closeAdmin.pressed ? "#ffb8d8" : "#f59aca"
                        Text { anchors.centerIn: parent; text: "Schließen"; color: "#2c1f2e"; font.pixelSize: 14; font.bold: true }
                        MouseArea { id: closeAdmin; anchors.fill: parent; onClicked: root.adminHintVisible = false }
                    }
                }
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
}
