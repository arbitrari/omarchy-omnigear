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
//
// With no device at all — the moment before the first read comes back — it is
// the OmniGear mark. The category glyph has a fallback for an unknown kind of
// thing, but it draws a tablet, and a tablet on the bar while the plugin is
// starting says something untrue about hardware rather than "not yet".
Item {
    id: root

    property var device: null
    property QtObject bar: null
    property real size: Style.font.heading

    readonly property color foreground: bar ? bar.foreground : Color.foreground
    readonly property color background: bar ? bar.background : Color.background
    readonly property string style: device ? device.icon : ""
    // Nothing to draw a device for yet.
    readonly property bool loading: !device

    implicitWidth: {
        if (loading)
            return mark.width;
        return style === "mouse-two-tone" ? drawn.width : glyph.implicitWidth;
    }
    implicitHeight: {
        if (loading)
            return mark.height;
        return style === "mouse-two-tone" ? drawn.height : glyph.implicitHeight;
    }

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

    // The O and G of the wordmark.
    //
    // Set well below the em. A wordmark is two letters where the other icons
    // are one picture, so matching their height makes it more than twice
    // their width and it takes over the bar. At seven tenths it reads as a
    // mark rather than as a banner, and the counters in the O and G still
    // survive: the letterforms are ten units of stroke on a sixty-unit cap,
    // so at nine pixels the strokes are still over a pixel and a half.
    readonly property real markScale: 0.7

    Item {
        id: mark
        anchors.centerIn: parent
        visible: root.loading
        height: Math.round(root.size * root.markScale)
        width: Math.round(height * 140 / 60)

        Layer {
            art: Qt.resolvedUrl("../../assets/icons/omnigear-og.svg")
            tint: root.foreground
        }
    }

    Text {
        id: glyph
        anchors.centerIn: parent
        visible: !root.loading && root.style !== "mouse-two-tone"
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
        visible: !root.loading && root.style === "mouse-two-tone"
        // Sized to land exactly where the font glyph would.
        //
        // `size` is a font pixel size, which sets the em box; the glyph's ink
        // is 914 of the font's 1000 units tall and 668 wide, so the drawing has
        // to be scaled by that or it comes out larger than the glyph it
        // replaces.
        height: Math.round(root.size * 914 / 1000)
        width: Math.round(height * 668 / 914)

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
