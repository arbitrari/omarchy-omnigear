package hidraw

// Naming wireless receivers.
//
// A dongle will not say what it is. Every Logitech receiver reports the USB
// product string "USB Receiver" regardless of whether it is a Lightspeed
// gaming dongle, a Unifying one or a cheap Nano, so the only way to tell them
// apart is the USB product id.
//
// The table is transcribed from Solaar's KNOWN_RECEIVERS
// (logitech_receiver/base_usb.py, GPL-2.0-or-later), which is the maintained
// reference for these ids. Only the id, the kind and the display name are
// taken; none of its code is used. An id missing here is not an error — it
// degrades to a generic name that prints the id, so an unknown dongle can be
// reported and added.

// ReceiverKind is the family a dongle belongs to. The kind, not the model, is
// what a user recognises: "Lightspeed" is on the box, "C54D" is not.
type ReceiverKind string

const (
	Lightspeed  ReceiverKind = "lightspeed"
	Unifying    ReceiverKind = "unifying"
	Bolt        ReceiverKind = "bolt"
	Nano        ReceiverKind = "nano"
	Legacy27MHz ReceiverKind = "27mhz"
	// UnknownReceiver is a dongle that is not in the table.
	UnknownReceiver ReceiverKind = "receiver"
)

// Label is how the kind reads in the UI.
func (k ReceiverKind) Label() string {
	switch k {
	case Lightspeed:
		return "Lightspeed"
	case Unifying:
		return "Unifying"
	case Bolt:
		return "Bolt"
	case Nano:
		return "Nano"
	case Legacy27MHz:
		return "27 MHz"
	default:
		return "Wireless"
	}
}

const (
	vendorLogitech = 0x046D
	vendorLenovo   = 0x17EF
)

type receiverID struct {
	Vendor  uint16
	Product uint16
}

var knownReceivers = map[receiverID]ReceiverKind{
	{vendorLogitech, 0xC548}: Bolt,

	{vendorLogitech, 0xC52B}: Unifying,
	{vendorLogitech, 0xC532}: Unifying,

	{vendorLogitech, 0xC518}: Nano,
	{vendorLogitech, 0xC51A}: Nano,
	{vendorLogitech, 0xC51B}: Nano,
	{vendorLogitech, 0xC521}: Nano,
	{vendorLogitech, 0xC525}: Nano,
	{vendorLogitech, 0xC526}: Nano,
	{vendorLogitech, 0xC52E}: Nano,
	{vendorLogitech, 0xC52F}: Nano,
	{vendorLogitech, 0xC531}: Nano,
	{vendorLogitech, 0xC534}: Nano,
	{vendorLogitech, 0xC535}: Nano, // branded as Dell
	{vendorLogitech, 0xC537}: Nano,
	{vendorLenovo, 0x6042}:   Nano, // branded as Lenovo

	{vendorLogitech, 0xC539}: Lightspeed,
	{vendorLogitech, 0xC53A}: Lightspeed,
	{vendorLogitech, 0xC53D}: Lightspeed,
	{vendorLogitech, 0xC53F}: Lightspeed,
	{vendorLogitech, 0xC541}: Lightspeed,
	{vendorLogitech, 0xC545}: Lightspeed,
	{vendorLogitech, 0xC547}: Lightspeed,
	{vendorLogitech, 0xC54D}: Lightspeed,

	{vendorLogitech, 0xC517}: Legacy27MHz,
}

// ReceiverKindOf names the dongle a device is paired to. The second result is
// false for an id the table does not know.
func ReceiverKindOf(vendor, product uint16) (ReceiverKind, bool) {
	kind, known := knownReceivers[receiverID{vendor, product}]
	if !known {
		return UnknownReceiver, false
	}
	return kind, true
}
