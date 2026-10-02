import QtQuick
import qs.Commons
import qs.Ui
import "../Model.js" as Model

// Capability control: Sonar — separate outputs for games, chat, media and
// the rest, with the headset's ChatMix dial balancing game against chat.
//
// The switch restarts the sound server, which is said on it rather than left
// to surprise anyone: every stream drops for a moment. Below it, each app
// playing can be moved to a channel. The dial's position
// comes from a listener the service keeps running, not from the device's
// read, because the base station only reports the dial as it turns.
Column {
    id: root

    property var sonar: null
    // { game, chat, error } from the ChatMix listener, or null until the dial
    // has moved since the listener started.
    property var chatMix: null
    property QtObject bar: null
    property bool busy: false

    signal requested(bool on)
    signal appRequested(string name, string channel)
    signal volumeRequested(string channel, int volume)
    // While a knob is dragged, every level it passes through.
    signal volumeMoved(string channel, int volume)

    readonly property color foreground: bar ? bar.foreground : Color.foreground
    readonly property color muted: Qt.darker(foreground, 1.4)
    readonly property string fontFamily: bar ? bar.fontFamily : Style.font.family
    readonly property bool enabledNow: sonar !== null && sonar.enabled

    width: parent ? parent.width : implicitWidth
    spacing: Style.space(10)
    visible: sonar !== null
    opacity: busy ? 0.5 : 1.0
    enabled: !busy

    Behavior on opacity {
        NumberAnimation { duration: 120 }
    }

    Column {
        width: parent.width
        spacing: Style.space(6)

        SettingHeader {
            bar: root.bar
            group: true
            label: Model.capabilityLabel("sonar")
            value: root.enabledNow ? "On" : "Off"
        }

        ButtonGroup {
            width: parent.width
            spacing: Style.space(4)

            foreground: root.foreground
            background: root.bar ? root.bar.background : Color.background
            accent: Color.accent
            fontFamily: root.fontFamily
            fontSize: Style.font.bodySmall
            focusable: false

            options: [
                { value: "off", label: "Off", tooltip: "One output. Restarts audio for a moment." },
                { value: "on", label: "On", tooltip: "Game, Chat, Media and Aux outputs. Restarts audio for a moment." }
            ]
            value: root.enabledNow ? "on" : "off"

            onChanged: function (value) {
                root.requested(value === "on");
            }
        }

        Text {
            width: parent.width
            visible: root.enabledNow && !root.sonar.live
            text: "On, but the sound server has not made the outputs. They appear when the headset is plugged in."
            wrapMode: Text.WordWrap
            textFormat: Text.PlainText
            color: root.muted
            font.family: root.fontFamily
            font.pixelSize: Style.font.caption
        }
    }

    Column {
        width: parent.width
        spacing: Style.space(6)
        visible: root.enabledNow

        SettingHeader {
            bar: root.bar
            group: true
            label: "ChatMix"
            value: root.chatMix
                ? "Game " + root.chatMix.game + "% · Chat " + root.chatMix.chat + "%"
                : ""
        }

        // Game on the left, chat on the right, as the dial turns.
        Row {
            width: parent.width
            height: Style.space(4)
            spacing: Style.space(2)
            visible: root.chatMix !== null

            Rectangle {
                width: (parent.width - parent.spacing) / 2
                height: parent.height
                color: Qt.darker(root.foreground, 3)

                Rectangle {
                    anchors.right: parent.right
                    width: parent.width * (root.chatMix ? root.chatMix.game : 0) / 100
                    height: parent.height
                    color: root.foreground
                }
            }

            Rectangle {
                width: (parent.width - parent.spacing) / 2
                height: parent.height
                color: Qt.darker(root.foreground, 3)

                Rectangle {
                    anchors.left: parent.left
                    width: parent.width * (root.chatMix ? root.chatMix.chat : 0) / 100
                    height: parent.height
                    color: root.foreground
                }
            }
        }

        // Said every time, not only before the dial has moved: the balance
        // is the base station's, and nothing on this side can set it.
        Text {
            width: parent.width
            text: root.chatMix === null
                ? "Set with the ChatMix dial on the base station. It cannot be changed from here; turn the dial to show where it is."
                : "Set with the ChatMix dial on the base station. It cannot be changed from here."
            wrapMode: Text.WordWrap
            textFormat: Text.PlainText
            color: root.muted
            font.family: root.fontFamily
            font.pixelSize: Style.font.caption
        }

        Text {
            width: parent.width
            visible: root.chatMix !== null && (root.chatMix.error || "") !== ""
            text: root.chatMix ? root.chatMix.error : ""
            wrapMode: Text.WordWrap
            textFormat: Text.PlainText
            color: root.bar ? root.bar.urgent : Color.urgent
            font.family: root.fontFamily
            font.pixelSize: Style.font.caption
        }
    }

    // Each channel's own level, the one a mixer shows for it. The dial scales
    // on top of this rather than moving it, so a slider here stays where it
    // was put however the dial is turned.
    Column {
        width: parent.width
        spacing: Style.space(6)
        visible: root.enabledNow && root.sonar.live

        SettingHeader {
            bar: root.bar
            group: true
            label: "Audio Channels"
        }

        // A knob per channel, all four on one line.
        Row {
            id: dials
            width: parent.width

            Repeater {
                model: root.sonar ? root.sonar.channels : []

                delegate: VolumeDial {
                    required property var modelData

                    width: dials.width / Math.max(1, root.sonar.channels.length)
                    bar: root.bar
                    label: modelData.label
                    value: modelData.volume
                    tooltip: modelData.label + " volume. Drag up or down to change."
                    onMoved: function (v) {
                        root.volumeMoved(modelData.slug, v);
                    }
                    onReleased: function (v) {
                        root.volumeRequested(modelData.slug, v);
                    }
                }
            }
        }
    }

    // What is playing, and which channel each app plays to. By app rather
    // than by stream, because that is what WirePlumber remembers: a choice
    // here sticks the next time the app plays.
    Column {
        width: parent.width
        spacing: Style.space(8)
        visible: root.enabledNow && root.sonar.live

        SettingHeader {
            bar: root.bar
            group: true
            label: "App Routing"
        }

        Repeater {
            model: root.sonar ? root.sonar.apps : []

            delegate: Column {
                required property var modelData

                width: parent.width
                spacing: Style.space(4)

                Text {
                    width: parent.width
                    text: modelData.label
                    elide: Text.ElideRight
                    textFormat: Text.PlainText
                    color: root.foreground
                    font.family: root.fontFamily
                    font.pixelSize: Style.font.bodySmall
                }

                ButtonGroup {
                    width: parent.width
                    spacing: Style.space(4)

                    foreground: root.foreground
                    background: root.bar ? root.bar.background : Color.background
                    accent: Color.accent
                    fontFamily: root.fontFamily
                    fontSize: Style.font.caption
                    focusable: false

                    options: root.sonar.channels.map(function (c) {
                        return {
                            value: c.slug,
                            label: c.label,
                            tooltip: c.mixed ? "On the ChatMix dial." : "Not affected by the ChatMix dial."
                        };
                    })
                    // Empty when the app plays somewhere that is not a
                    // channel, so no button claims it.
                    value: modelData.channel

                    onChanged: function (value) {
                        root.appRequested(modelData.name, value);
                    }
                }
            }
        }

        Text {
            width: parent.width
            visible: root.sonar !== null && root.sonar.apps.length === 0
            text: "Nothing is playing. Apps show here while they play."
            wrapMode: Text.WordWrap
            textFormat: Text.PlainText
            color: root.muted
            font.family: root.fontFamily
            font.pixelSize: Style.font.caption
        }

        Text {
            width: parent.width
            text: "New apps play to Game automatically."
            wrapMode: Text.WordWrap
            textFormat: Text.PlainText
            color: root.muted
            font.family: root.fontFamily
            font.pixelSize: Style.font.caption
        }
    }
}
