import QtQuick
import qs.Commons
import qs.Ui
import "../Model.js" as Model

// Capability control: each Sonar channel's own equalizer, applied in software.
//
// While Sonar is on, the base station hands its equalizer to the computer —
// its menu says the EQ is "on Sonar" — so the curves live with the channels
// instead: bass on Game, clarity on Chat. One channel is shown at a time;
// four sets of ten sliders would bury everything else.
Column {
    id: root

    property var sonar: null
    property QtObject bar: null
    property bool busy: false

    signal presetRequested(string channel, string preset)
    signal bandRequested(string channel, string band, int level)

    // Which channel's equalizer is shown.
    property string channel: "game"
    readonly property var channelData: {
        var channels = sonar ? sonar.channels : [];
        for (var i = 0; i < channels.length; i++) {
            if (channels[i].slug === channel)
                return channels[i];
        }
        return channels.length > 0 ? channels[0] : null;
    }
    readonly property bool usable: sonar !== null && sonar.enabled && sonar.live

    readonly property color foreground: bar ? bar.foreground : Color.foreground
    readonly property string fontFamily: bar ? bar.fontFamily : Style.font.family

    width: parent ? parent.width : implicitWidth
    spacing: Style.space(10)
    visible: sonar !== null

    // Said rather than hidden: an Equalizer tab with nothing in it would look
    // broken, and the fix is one tab over.
    Text {
        width: parent.width
        visible: !root.usable
        text: "The equalizer works on the Sonar channels. Turn Sonar on in the Sonar tab to use it."
        wrapMode: Text.WordWrap
        textFormat: Text.PlainText
        color: Qt.darker(root.foreground, 1.4)
        font.family: root.fontFamily
        font.pixelSize: Style.font.caption
    }

    Column {
        width: parent.width
        spacing: Style.space(6)
        visible: root.usable

        SettingHeader {
            bar: root.bar
            group: true
            label: "Audio Channel"
            value: root.channelData ? root.channelData.label : ""
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

            options: root.sonar ? root.sonar.channels.map(function (c) {
                return { value: c.slug, label: c.label, tooltip: "Show " + c.label + "'s equalizer." };
            }) : []
            value: root.channelData ? root.channelData.slug : ""

            onChanged: function (value) {
                root.channel = value;
            }
        }
    }

    EqualizerControl {
        bar: root.bar
        busy: root.busy
        visible: root.usable && root.channelData !== null && root.channelData.equalizer !== null
        equalizer: root.channelData ? root.channelData.equalizer : null
        onPresetRequested: function (preset) {
            root.presetRequested(root.channelData.slug, preset);
        }
        onBandRequested: function (band, level) {
            root.bandRequested(root.channelData.slug, band, level);
        }
    }
}
