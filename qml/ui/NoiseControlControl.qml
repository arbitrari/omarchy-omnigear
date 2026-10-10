import QtQuick
import qs.Commons
import qs.Ui
import "../Model.js" as Model

// Capability control: what a headset does with the sound of the room.
//
// Three modes, and two settings that belong to only one of them. The level
// and the voice filter are hidden outside ambient sound rather than greyed
// out, because the headset does not even report them there: noise cancelling
// reads both back as zero, and a slider showing that would be a lie. A
// headset with no voice filter at all reports it as null, and gets no switch.
Column {
    id: root

    property var noiseControl: null
    property QtObject bar: null
    property bool busy: false

    signal modeRequested(string mode)
    signal levelRequested(int level)
    signal voiceRequested(bool on)

    readonly property color foreground: bar ? bar.foreground : Color.foreground
    readonly property string mode: noiseControl ? noiseControl.mode : ""
    readonly property bool ambient: mode === "ambient"
    readonly property string ambientLabel: Model.noiseModeLabel("ambient", noiseControl)

    width: parent ? parent.width : implicitWidth
    spacing: Style.space(10)
    visible: noiseControl !== null
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
            label: Model.capabilityLabel("noise-control")
            value: Model.noiseModeLabel(root.mode, root.noiseControl)
        }

        ButtonGroup {
            width: parent.width
            spacing: Style.space(4)

            foreground: root.foreground
            background: root.bar ? root.bar.background : Color.background
            accent: Color.accent
            fontFamily: root.bar ? root.bar.fontFamily : Style.font.family
            fontSize: Style.font.bodySmall
            focusable: false

            options: [
                {
                    value: "noise-cancelling",
                    label: Model.noiseModeLabel("noise-cancelling"),
                    tooltip: "Cancels the sound around you."
                },
                {
                    value: "ambient",
                    label: root.ambientLabel,
                    tooltip: "Lets the sound around you through the microphones."
                },
                {
                    value: "off",
                    label: Model.noiseModeLabel("off"),
                    tooltip: "Neither: the earcups alone."
                }
            ]
            value: root.mode

            onChanged: function (value) {
                root.modeRequested(value);
            }
        }
    }

    Column {
        width: parent.width
        spacing: Style.space(6)
        visible: root.ambient

        SettingHeader {
            bar: root.bar
            // "Ambient Sound Level" says nothing "Ambient Level" does not.
            label: root.ambientLabel === "Ambient Sound" ? "Ambient Level" : root.ambientLabel + " Level"
            value: root.noiseControl ? String(root.noiseControl.ambientLevel) : ""
        }

        SquaredSlider {
            width: parent.width
            height: Style.spacing.controlHeight
            bar: root.bar
            minimum: root.noiseControl ? root.noiseControl.minAmbientLevel : 1
            maximum: root.noiseControl ? root.noiseControl.maxAmbientLevel : 20
            value: root.noiseControl ? root.noiseControl.ambientLevel : 1
            step: 1
            integer: true
            onReleased: function (v) {
                root.levelRequested(Math.round(v));
            }
        }
    }

    Column {
        width: parent.width
        spacing: Style.space(6)
        visible: root.ambient && root.noiseControl !== null
            && root.noiseControl.focusOnVoice !== null

        SettingHeader {
            bar: root.bar
            label: "Focus On Voice"
            value: root.noiseControl && root.noiseControl.focusOnVoice ? "On" : "Off"
        }

        ButtonGroup {
            width: parent.width
            spacing: Style.space(4)

            foreground: root.foreground
            background: root.bar ? root.bar.background : Color.background
            accent: Color.accent
            fontFamily: root.bar ? root.bar.fontFamily : Style.font.family
            fontSize: Style.font.bodySmall
            focusable: false

            options: [
                { value: "off", label: "Off", tooltip: "Let in all of the room." },
                { value: "on", label: "On", tooltip: "Let in voices, and filter out the rest." }
            ]
            value: root.noiseControl && root.noiseControl.focusOnVoice ? "on" : "off"

            onChanged: function (value) {
                root.voiceRequested(value === "on");
            }
        }
    }
}
