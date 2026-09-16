import QtQuick 2.15
import QtQuick.Window 2.15

Window {
    id: root
    visible: true
    visibility: Window.FullScreen
    color: "#120817"
    title: "MuPiBox"

    // The UI is designed in a stable 800x480 logical coordinate system and
    // scaled as one unit to the actual display. This keeps the first hardware
    // build usable on different DSI/HDMI panels without hard-coding a mode.
    property real uiScale: Math.min(width / 800, height / 480)
    property var apiCandidates: ["http://127.0.0.1:8090", "http://127.0.0.1:8080"]
    property int apiIndex: 0
    property string apiBase: apiCandidates[apiIndex]
    property bool backendOnline: false
    property string backendState: "Verbinde …"
    property string clockText: "--:--"
    property var categories: []

    function localized(labels, fallback) {
        if (!labels) return fallback || ""
        return labels.de || labels.en || fallback || ""
    }

    function getJson(path, done, failed) {
        var xhr = new XMLHttpRequest()
        xhr.open("GET", apiBase + path)
        xhr.onreadystatechange = function() {
            if (xhr.readyState !== XMLHttpRequest.DONE) return
            if (xhr.status >= 200 && xhr.status < 300) {
                backendOnline = true
                backendState = "Verbunden"
                try { done(JSON.parse(xhr.responseText)) } catch (e) { if (failed) failed() }
            } else if (failed) {
                failed()
            }
        }
        try { xhr.send() } catch (e) { if (failed) failed() }
    }

    function refreshLegacyLibrary(failed) {
        getJson("/api/library", function(folders) {
            var result = []
            for (var i = 0; folders && i < folders.length; ++i) {
                var folder = folders[i]
                result.push({"id": folder.id, "labels": {"de": folder.name, "en": folder.name}})
            }
            categories = result
            backendOnline = true
            backendState = "Verbunden"
        }, failed)
    }

    function refreshAt(index) {
        apiIndex = index
        apiBase = apiCandidates[index]
        getJson("/api/home", function(data) {
            if (data && data.categories) categories = data.categories
        }, function() {
            // Compatibility with the earlier foundation branch while /api/home is being merged.
            refreshLegacyLibrary(function() {
                if (index + 1 < apiCandidates.length) {
                    refreshAt(index + 1)
                } else {
                    backendOnline = false
                    backendState = "Backend offline"
                }
            })
        })
    }

    function refresh() {
        refreshAt(apiIndex)
    }

    function updateClock() {
        clockText = Qt.formatTime(new Date(), "hh:mm")
    }

    Component.onCompleted: {
        updateClock()
        refreshAt(0)
    }

    Timer { interval: 3000; running: true; repeat: true; onTriggered: root.refresh() }
    Timer { interval: 30000; running: true; repeat: true; onTriggered: root.updateClock() }

    Item {
        id: canvas
        width: 800
        height: 480
        anchors.centerIn: parent
        scale: root.uiScale

        Rectangle {
            anchors.fill: parent
            color: "#16091d"
        }

        Rectangle {
            id: topBar
            anchors.top: parent.top
            anchors.left: parent.left
            anchors.right: parent.right
            height: 46
            color: "#250f31"

            Text {
                anchors.left: parent.left
                anchors.leftMargin: 18
                anchors.verticalCenter: parent.verticalCenter
                text: "MuPiBox"
                color: "#ffd3c7"
                font.pixelSize: 22
                font.bold: true
            }

            Rectangle {
                anchors.centerIn: parent
                width: stateText.implicitWidth + 24
                height: 27
                radius: 14
                color: root.backendOnline ? "#3a2750" : "#5a233a"
                Text {
                    id: stateText
                    anchors.centerIn: parent
                    text: root.backendState
                    color: "white"
                    font.pixelSize: 13
                }
            }

            Text {
                anchors.right: parent.right
                anchors.rightMargin: 18
                anchors.verticalCenter: parent.verticalCenter
                text: root.clockText
                color: "white"
                font.pixelSize: 20
                font.bold: true
            }
        }

        Column {
            anchors.top: topBar.bottom
            anchors.topMargin: 28
            anchors.left: parent.left
            anchors.leftMargin: 24
            anchors.right: parent.right
            anchors.rightMargin: 24
            spacing: 18

            Text {
                text: "Was möchtest du hören?"
                color: "#fff7fb"
                font.pixelSize: 30
                font.bold: true
            }

            Text {
                text: root.categories.length ? "Deine Inhalte sind geladen." : "MuPiBox bereitet deine Inhalte vor …"
                color: "#bfaec8"
                font.pixelSize: 17
            }

            Flickable {
                width: parent.width
                height: 150
                contentWidth: categoryRow.width
                contentHeight: height
                clip: true
                boundsBehavior: Flickable.StopAtBounds

                Row {
                    id: categoryRow
                    spacing: 14

                    Repeater {
                        model: root.categories
                        delegate: Rectangle {
                            width: 170
                            height: 120
                            radius: 16
                            color: touch.pressed ? "#674078" : "#392348"
                            border.color: "#5b3b6d"
                            property var categoryData: modelData

                            Text {
                                anchors.centerIn: parent
                                width: parent.width - 24
                                text: root.localized(categoryData.labels, categoryData.id)
                                horizontalAlignment: Text.AlignHCenter
                                wrapMode: Text.WordWrap
                                color: "#fff8fc"
                                font.pixelSize: 22
                                font.bold: true
                            }

                            MouseArea { id: touch; anchors.fill: parent }
                        }
                    }
                }
            }

            Text {
                text: "Native Qt-Quick-Oberfläche · kein Browser"
                color: "#8f7a9a"
                font.pixelSize: 14
            }
        }

        Rectangle {
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            height: 76
            color: "#21102a"
            border.color: "#3a2545"

            Text {
                anchors.left: parent.left
                anchors.leftMargin: 22
                anchors.verticalCenter: parent.verticalCenter
                text: "♫  Such dir etwas aus"
                color: "white"
                font.pixelSize: 18
                font.bold: true
            }

            Rectangle {
                width: 54
                height: 54
                radius: 27
                anchors.right: parent.right
                anchors.rightMargin: 18
                anchors.verticalCenter: parent.verticalCenter
                color: "#ffd3c7"
                Text { anchors.centerIn: parent; text: "▶"; color: "#24142b"; font.pixelSize: 24 }
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
            Timer { interval: 2300; running: true; repeat: false; onTriggered: splash.opacity = 0 }
        }
    }
}
