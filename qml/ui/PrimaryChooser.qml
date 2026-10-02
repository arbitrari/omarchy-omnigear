import QtQuick
import qs.Commons
import qs.Ui

// Picks which device of one kind speaks for the bar.
//
// One chooser per category, not one for the whole panel: "the mouse" and "the
// keyboard" are separate questions, and the bar will eventually report a
// battery for each. Adding the next kind is another instance of this, not a
// rewrite of it.
//
// Hidden when there is nothing to choose between — a single device of a kind
// needs no preference.
Item {
    id: root

    property string label: ""
    // The devices of this one category, already filtered.
    property var devices: []
    // The chosen device id; empty means the widget decides for itself.
    property string current: ""
    property QtObject bar: null

    signal chosen(string deviceId)

    readonly property color foreground: bar ? bar.foreground : Color.foreground
    readonly property string fontFamily: bar ? bar.fontFamily : Style.font.family

    width: parent ? parent.width : implicitWidth
    visible: devices.length > 1
    height: visible ? picker.height : 0

    Text {
        anchors.left: parent.left
        anchors.verticalCenter: picker.verticalCenter
        text: root.label.toUpperCase()
        textFormat: Text.PlainText
        color: root.foreground
        font.family: root.fontFamily
        font.pixelSize: Style.font.bodySmall
        font.bold: true
    }

    Dropdown {
        id: picker
        anchors.right: parent.right
        width: Math.round(parent.width * 0.62)
        showLabel: false
        fontFamily: root.fontFamily

        // "Automatic" is a real option, not the absence of one: an empty id is
        // what the widget stores when it should decide for itself.
        options: [
            {
                value: "",
                label: "Automatic"
            }
        ].concat(root.devices.map(function (device) {
            return {
                value: device.id,
                label: device.name
            };
        }))
        value: root.current

        onChanged: function (value) {
            root.chosen(value);
        }
    }
}
