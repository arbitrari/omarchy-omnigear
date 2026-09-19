import QtQuick
import QtQuick.Effects
import qs.Commons
import "../Model.js" as Model

// The little picture beside a device's name.
//
// Normally the category glyph — a mouse, a keyboard, a headset. A device whose
// catalog entry names a style gets that instead, so a model that looks like
// something in particular can say so without any code here knowing which model
// it is.
Item {
    id: root

    property var device: null
    property QtObject bar: null
    property real size: Style.font.heading

    readonly property color foreground: bar ? bar.foreground : Color.foreground
    readonly property color background: bar ? bar.background : Color.background
    readonly property string style: device ? device.icon : ""

    implicitWidth: style === "mouse-two-tone" ? drawn.width : glyph.implicitWidth
    implicitHeight: style === "mouse-two-tone" ? drawn.height : glyph.implicitHeight

    Text {
        id: glyph
        anchors.centerIn: parent
        visible: root.style !== "mouse-two-tone"
        text: Model.categoryIcon(root.device ? root.device.category : "")
        textFormat: Text.PlainText
        color: root.foreground
        font.family: "monospace"
        font.pixelSize: root.size
    }

    // A pale-shelled mouse with dark click surfaces.
    //
    // Two SVG layers on one grid, each tinted separately: the shell takes the
    // foreground, the clicks and wheel the background. Drawing it with
    // rectangles could not give the silhouette a curve, and baking the colours
    // into one file would ignore the theme.
    Item {
        id: drawn
        anchors.centerIn: parent
        visible: root.style === "mouse-two-tone"
        // Sized to land exactly where the font glyph would.
        //
        // `size` is a font pixel size, which sets the em box; the glyph's ink
        // is 914 of the font's 1000 units tall and 668 wide, so the drawing has
        // to be scaled by that or it comes out larger than the glyph it
        // replaces.
        height: Math.round(root.size * 914 / 1000)
        width: Math.round(height * 668 / 914)

        component Layer: Item {
            id: layer
            property url art
            property color tint

            anchors.fill: parent

            Image {
                id: source
                anchors.fill: parent
                source: layer.art
                sourceSize.width: Math.round(width * Screen.devicePixelRatio)
                sourceSize.height: Math.round(height * Screen.devicePixelRatio)
                fillMode: Image.PreserveAspectFit
                visible: false
                layer.enabled: true
            }

            MultiEffect {
                anchors.fill: source
                source: source
                colorization: 1.0
                colorizationColor: layer.tint
            }
        }

        Layer {
            art: Qt.resolvedUrl("../../assets/icons/mouse-shell.svg")
            tint: root.foreground
        }

        Layer {
            art: Qt.resolvedUrl("../../assets/icons/mouse-clicks.svg")
            tint: root.background
        }
    }
}
