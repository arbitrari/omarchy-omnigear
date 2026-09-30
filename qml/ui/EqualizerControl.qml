import QtQuick
import qs.Commons
import qs.Ui
import "../Model.js" as Model

// Capability control: a headset's equalizer.
//
// A preset, and the bands it sets. Moving a band on a preset that keeps no
// bands of its own moves the headset onto Manual, carrying the rest over —
// the driver does that, so this only ever asks for the one band.
//
// When the headset will not apply an equalizer at all — a WH-1000XM3 over
// LDAC or aptX — the controls are replaced by the reason. Offering sliders
// that the headset refuses would look like a broken panel rather than a
// codec limit.
Column {
    id: root

    property var equalizer: null
    property QtObject bar: null
    property bool busy: false

    signal presetRequested(string preset)
    signal bandRequested(string band, int level)

    readonly property color foreground: bar ? bar.foreground : Color.foreground
    readonly property string fontFamily: bar ? bar.fontFamily : Style.font.family
    readonly property bool available: equalizer ? equalizer.available : false

    width: parent ? parent.width : implicitWidth
    spacing: Style.space(10)
    visible: equalizer !== null
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
            label: Model.capabilityLabel("equalizer")
            value: root.available ? Model.eqPresetLabel(root.equalizer) : "Unavailable"
        }

        Text {
            width: parent.width
            visible: !root.available
            text: root.equalizer ? Model.sentenceCase(root.equalizer.unavailable) + "." : ""
            textFormat: Text.PlainText
            wrapMode: Text.WordWrap
            color: Qt.darker(root.foreground, 1.4)
            font.family: root.fontFamily
            font.pixelSize: Style.font.caption
        }

        Dropdown {
            width: parent.width
            visible: root.available
            showLabel: false
            fontFamily: root.fontFamily
            options: root.equalizer ? root.equalizer.presets.map(function (preset) {
                return { value: preset.slug, label: preset.label };
            }) : []
            value: root.equalizer ? root.equalizer.preset : ""

            onChanged: function (value) {
                root.presetRequested(value);
            }
        }
    }

    Repeater {
        model: root.available && root.equalizer ? root.equalizer.bands : []

        delegate: Column {
            id: band
            required property var modelData

            width: root.width
            spacing: Style.space(4)

            SettingHeader {
                bar: root.bar
                label: band.modelData.label
                value: Model.signedLevel(band.modelData.value)
            }

            SquaredSlider {
                width: parent.width
                height: Style.spacing.controlHeight
                bar: root.bar
                minimum: root.equalizer.min
                maximum: root.equalizer.max
                value: band.modelData.value
                step: 1
                integer: true
                onReleased: function (v) {
                    root.bandRequested(band.modelData.slug, Math.round(v));
                }
            }
        }
    }
}
