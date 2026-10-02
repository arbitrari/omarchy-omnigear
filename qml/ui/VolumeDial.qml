import QtQuick
import qs.Commons
import qs.Ui

// A volume knob: a three-quarter arc with the level in the middle and a name
// beneath, small enough that several sit side by side where sliders would
// each need a row.
//
// Dragged up and down rather than around. Following the pointer round a
// circle this size is fiddly, and up-for-louder is how a knob works in every
// audio program anyone is likely to have used. The wheel is passed on, as it
// is for the sliders: the panel scrolls, and scrolling past a knob must not
// change it.
//
// Drawn in the squared slider's colours, flat-ended, so the two read as one
// family.
Item {
    id: root

    property QtObject bar: null
    property string label: ""
    property int value: 0
    property int minimum: 0
    property int maximum: 100
    property string tooltip: "Drag up or down to change."

    // What is shown while dragging, before the release is sent.
    property int liveValue: value
    property bool dragging: false

    signal released(int value)

    readonly property color foreground: bar ? bar.foreground : Color.foreground
    readonly property color trackColor: bar ? Style.selectedFillFor(bar.foreground, Color.accent) : "#333"
    readonly property string fontFamily: bar ? bar.fontFamily : Style.font.family

    // A drag of this many pixels covers the whole range.
    readonly property real dragRange: Style.space(150)
    readonly property real size: Math.min(width, Style.space(64))
    readonly property real thickness: Math.max(4, Math.round(size * 0.09))

    implicitHeight: dial.height + Style.space(4) + name.implicitHeight

    onValueChanged: if (!dragging) liveValue = value
    onLiveValueChanged: arc.requestPaint()
    onForegroundChanged: arc.requestPaint()
    onTrackColorChanged: arc.requestPaint()

    Item {
        id: dial
        anchors.horizontalCenter: parent.horizontalCenter
        width: root.size
        height: root.size

        Canvas {
            id: arc
            anchors.fill: parent

            onPaint: {
                var ctx = getContext("2d");
                ctx.reset();
                var r = (Math.min(width, height) - root.thickness) / 2;
                var cx = width / 2;
                var cy = height / 2;
                // From lower left round the top to lower right: 270 degrees,
                // the gap at the bottom.
                var start = 0.75 * Math.PI;
                var sweep = 1.5 * Math.PI;
                var span = Math.max(1, root.maximum - root.minimum);
                var progress = Math.max(0, Math.min(1, (root.liveValue - root.minimum) / span));

                ctx.lineWidth = root.thickness;
                ctx.lineCap = "butt";

                ctx.strokeStyle = root.trackColor;
                ctx.beginPath();
                ctx.arc(cx, cy, r, start, start + sweep, false);
                ctx.stroke();

                if (progress > 0) {
                    ctx.strokeStyle = root.foreground;
                    ctx.beginPath();
                    ctx.arc(cx, cy, r, start, start + sweep * progress, false);
                    ctx.stroke();
                }
            }
        }

        // The real figure, even past the arc's end: a mixer can take a
        // channel over 100%, and the knob should not pretend otherwise.
        Text {
            anchors.centerIn: parent
            text: root.liveValue + "%"
            textFormat: Text.PlainText
            color: root.foreground
            font.family: root.fontFamily
            font.pixelSize: Style.font.bodySmall
            font.bold: true
        }

        MouseArea {
            id: mouse
            anchors.fill: parent
            hoverEnabled: true
            preventStealing: true
            cursorShape: Qt.SizeVerCursor

            property real startY: 0
            property int startValue: 0

            onPressed: function (event) {
                startY = event.y;
                startValue = Math.min(root.liveValue, root.maximum);
                root.dragging = true;
            }
            onPositionChanged: function (event) {
                if (!root.dragging)
                    return;
                var units = (startY - event.y) / root.dragRange * (root.maximum - root.minimum);
                root.liveValue = Math.max(root.minimum,
                    Math.min(root.maximum, Math.round(startValue + units)));
            }
            onReleased: {
                root.dragging = false;
                if (root.liveValue !== root.value)
                    root.released(root.liveValue);
            }
            onCanceled: {
                root.dragging = false;
                root.liveValue = root.value;
            }
            onWheel: function (wheel) {
                wheel.accepted = false;
            }
        }

        PanelToolTip {
            visible: mouse.containsMouse && !root.dragging && root.tooltip !== ""
            text: root.tooltip
            fontFamily: root.fontFamily
        }
    }

    Text {
        id: name
        anchors.top: dial.bottom
        anchors.topMargin: Style.space(4)
        anchors.horizontalCenter: parent.horizontalCenter
        text: root.label
        textFormat: Text.PlainText
        color: Qt.darker(root.foreground, 1.4)
        font.family: root.fontFamily
        font.pixelSize: Style.font.caption
        font.bold: true
    }
}
