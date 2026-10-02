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
    // The best codec the equalizer works over, when the codec can be switched
    // from the panel: { slug, label }, or null. Offered as a one-press fix
    // when the equalizer is unavailable.
    property var fixCodec: null

    signal presetRequested(string preset)
    signal bandRequested(string band, int level)
    signal codecRequested(string codec)

    readonly property color foreground: bar ? bar.foreground : Color.foreground
    readonly property string fontFamily: bar ? bar.fontFamily : Style.font.family
    readonly property bool available: equalizer ? equalizer.available : false

    width: parent ? parent.width : implicitWidth
    spacing: Style.space(10)
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

        Button {
            visible: !root.available && root.fixCodec !== null
            text: root.fixCodec ? "Switch to " + root.fixCodec.label : ""
            foreground: root.foreground
            background: root.bar ? root.bar.background : Color.background
            accent: Color.accent
            fontFamily: root.fontFamily
            fontSize: Style.font.bodySmall
            focusable: false
            onClicked: root.codecRequested(root.fixCodec.slug)
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

    // The bands side by side, low to high, the way an equalizer is drawn
    // everywhere else, so the shape of the curve can be read at a glance.
    Row {
        id: bands
        width: parent.width
        visible: root.available && root.equalizer !== null

        readonly property int count: root.equalizer ? root.equalizer.bands.length : 0
        readonly property real sliderLength: Style.space(140)

        Repeater {
            model: bands.visible ? root.equalizer.bands : []

            delegate: Column {
                id: band
                required property var modelData

                width: bands.count > 0 ? bands.width / bands.count : 0
                spacing: Style.space(6)

                Text {
                    width: parent.width
                    horizontalAlignment: Text.AlignHCenter
                    text: Model.signedLevel(band.modelData.value)
                    textFormat: Text.PlainText
                    color: root.foreground
                    font.family: root.fontFamily
                    font.pixelSize: Style.font.bodySmall
                }

                // The vendored slider only runs left to right, and is kept
                // byte-for-byte close to upstream, so it is turned rather than
                // taught a second orientation. A quarter turn anticlockwise
                // puts the minimum at the bottom; the mouse is mapped through
                // the rotation, so dragging still goes the way it looks.
                Item {
                    width: parent.width
                    height: bands.sliderLength

                    SquaredSlider {
                        anchors.centerIn: parent
                        width: parent.height
                        height: Style.spacing.controlHeight
                        rotation: -90
                        bar: root.bar
                        minimum: root.equalizer.min
                        maximum: root.equalizer.max
                        value: band.modelData.value
                        step: 1
                        integer: true
                        // A notch at each end and one at the middle, so flat
                        // is visible without a number.
                        tickCount: 3
                        onReleased: function (v) {
                            root.bandRequested(band.modelData.slug, Math.round(v));
                        }
                    }
                }

                Text {
                    width: parent.width
                    horizontalAlignment: Text.AlignHCenter
                    wrapMode: Text.WordWrap
                    text: band.modelData.label
                    textFormat: Text.PlainText
                    color: Qt.darker(root.foreground, 1.4)
                    font.family: root.fontFamily
                    font.pixelSize: Style.font.caption
                    font.bold: true
                }
            }
        }
    }
}
