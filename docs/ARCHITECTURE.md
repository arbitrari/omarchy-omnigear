# Architecture

This document explains how OmniGear is built and why. Most sections record
something learned on real hardware, with the bytes that showed it. To add a
device, follow [Add support for a device you own](ADDING-A-DEVICE.md) and come
back here for the sections that match your device.

- [Two processes, one seam](#two-processes-one-seam)
- [Where the code lives](#where-the-code-lives)
- [The CLI contract](#the-cli-contract)
- [Finding devices](#finding-devices)
- [Power: polling, sleep, and off](#power-polling-sleep-and-off)
- [HID++ devices](#hid-devices)
- [Bluetooth headphones](#bluetooth-headphones)
- [Headsets behind a base station](#headsets-behind-a-base-station)
- [Sonar](#sonar)
- [Why Go](#why-go)

## Two processes, one seam

```
  qml/            the Omarchy bar widget. Draws things, talks to no hardware.
    │
    │  JSON over stdout, one object per invocation
    ▼
  cmd/omnigear/   a Go CLI. Owns every byte that reaches a device.
    │
    ▼
  internal/       transports, drivers, and the device catalog
```

QML runs inside `omarchy-shell`, which lives as long as the desktop. A blocking
read on a wedged USB device there would freeze the desktop. The CLI is a
short-lived process with bounded timeouts, so the worst a broken device can do
is make one poll come back empty.

The top of the repository is the plugin that Omarchy loads: `manifest.json`,
`qml/`, `assets/`, and `udev/`. The Go program lives under `cmd/` and builds to
`bin/omnigear`, which is where the QML looks. `bin/` is not committed.

## Where the code lives

### The catalog mirrors the README

The tree goes category, then brand, then model:

```
internal/devices/
  mice/
    logitech/
      logitech.go                 every Logitech mouse, as catalog entries
      prox2superstrike.go         what is specific to this one model
    razer/  corsair/  glorious/  finalmouse/
  keyboards/
    logitech/  razer/  keychron/
  headsets/
    steelseries/  logitech/  razer/  hyperx/
    sony/  apple/  google/  nothing/  oneplus/
```

A device's runtime id is the same path, so a bug report names its own source
directory:

```
mouse/logitech/pro-x2-superstrike#5f-ba-c9-65
└─┬─┘ └───┬──┘ └────────┬───────┘ └────┬────┘
category  brand       model        serial
```

### Three layers under `internal/`

| Layer | Knows about | Reused by |
|---|---|---|
| `transport/` | wire formats, such as hidraw nodes and HID++ 2.0 | every brand that speaks it |
| `drivers/` | how to read and write a family of devices | every model in that family |
| `devices/` | which models exist and what they can do | nothing |

`transport/hidpp` is the HID++ protocol and nothing else, so a Logitech
keyboard can use it unchanged. `drivers/logitech` turns the protocol into
battery, DPI, and polling-rate reads. That split is why most new Logitech mice
need one catalog entry and no new code.

### Capabilities are the contract

A catalog entry declares what a model can do:

```go
Capabilities: []model.Capability{model.CapBattery, model.CapDPI, model.CapPollingRate},
```

Everything downstream keys off that list. The driver probes only what is
claimed. The CLI emits state only for what was probed. The UI renders one
control per capability. Adding a device that uses existing capabilities needs
no UI work.

## The CLI contract

### Every command prints one JSON object

Failures are objects too, so the QML never parses stderr:

```jsonc
{ "ok": true, "schema": 1, "devices": [ … ] }
{ "ok": false, "error": "no connected device matches 'foo'" }
```

`schema` goes up when the shape changes in a way that breaks a reader.
`omnigear watch` is the one exception. It streams one object per line for as
long as it runs. See [Sonar](#sonar).

### Writes are verified, never assumed

A device can accept a write and ignore it. `omnigear set` reads the device back
and compares:

| Outcome | Result |
|---|---|
| The value matches the request | `ok` |
| The value changed, but rounded | `ok`, with `applied` and a `note` |
| The value did not change | an error: "accepted the change but kept …" |

The UI shows what the hardware says, not what it was told.

One setting is exempt. Switching the Easy-Switch host (`0x1814`) succeeds by
making the device leave, so the verifying read finds nothing.
`SettingKey.Verifiable` marks it, `cmdSet` stops at "the device accepted it",
and the QML treats an `ok` reply with no device as a cue to re-read. A future
write with the same shape belongs there too. Nothing else does.

## Finding devices

### Permissions fail with no symptom

hidraw nodes are `crw-------` and root-only by default. Without a udev rule,
every device enumerates, the kernel drives it as a mouse, and OmniGear cannot
open it. Nothing errors. The device is just missing.

`udev/60-omnigear.rules` tags the vendors this project drives with `uaccess`,
so logind gives the node to whoever is logged in at the seat. The ACL follows
the session, which is safer than a world-writable mode or a group. The rule is
scoped by vendor. A rule over all of `SUBSYSTEM=="hidraw"` would give every HID
device on the machine, firmware-update endpoints included, to any process in
the session.

This was found the hard way. The project worked on a machine that had Solaar
installed, and Solaar's udev rule was granting the access. After Solaar was
removed, devices that were already plugged in kept working, because a node
keeps the ACL it got when it was added. Every newly plugged device vanished.

Because the symptom is absence, `discovery.Unreadable` looks for the cause:
nodes that carry HID++ and return `EACCES` on open. `list` reports them and the
panel prints the fix. A node that is only busy is not reported, because no udev
rule would help.

### One device owns several nodes, and some are stale

A device owns several hidraw nodes, sometimes under more than one USB id at
once. Plug a wireless mouse in by cable while its dongle is still in, and both
sets of nodes exist. Only one set carries the mouse. The other answers nothing,
which shows up as `connect: device did not answer`.

`discovery.choose` filters the nodes, cheapest test first:

1. **The report descriptor.** `hidpp.Speaks` looks for the HID++ report ids in
   `report_descriptor`. Of the four nodes a wired PRO X2 SUPERSTRIKE owns,
   exactly one declares them. Nothing is opened.
2. **A ping.** If more than one node is left, all of them are pinged at once
   and the first answer wins. A live device replies in milliseconds, and a
   stale node costs the whole probe budget.

A driver for another protocol recognizes its node by implementing
`model.Speaker`.

Nodes are grouped by serial, not by USB id, and the serial is normalized first.
The same mouse reports `5f-ba-c9-65` through its dongle and `5FBAC965` over
USB. Without normalizing, plugging in a cable would split one mouse into two
devices.

### Devices behind a receiver the kernel did not expand

The kernel expands a receiver it recognizes into one hidraw node per paired
device, and those nodes match by USB id. A receiver it does not recognize stays
one node, and the devices behind it have no USB id to match. A Logi Bolt
(`046d:c548`) is this case, because `hid-logitech-dj` has no entry for it and
`hid-generic` binds instead.

`discovery.behindReceivers` finds those devices:

- It scans only receivers the kernel did not expand. Scanning an expanded one
  would find each device twice, under two identities.
- It probes the slots concurrently. An empty slot costs the whole wake timeout,
  so probing six slots in series would make every poll crawl.
- The receiver's own count (HID++ 1.0 register `0x02`) ends the scan once that
  many devices have answered. A receiver that answers strangely falls back to
  scanning every slot.

Such a device identifies itself by name, so its catalog entry carries `Names`
instead of `USB`. `model.Device.Index` addresses it on the receiver's node.

### Uncatalogued devices still show up

A HID++ device that is not in the catalog still appears. Its entry is built
from what it says about itself: its name, how it is attached, and a capability
list derived from its feature table. The driver keys off the capability list,
so it reads and writes the device unchanged. That is why battery and DPI work
on an unknown Logitech mouse.

The entry is marked `Discovered` and `SupportUnsupported`, and the panel tints
and labels its card. "The device advertises this feature" is a weaker claim
than "someone confirmed it works on this model".

Only HID++ devices are found this way, on purpose. Other devices cannot be
identified from outside without guessing:

- The report descriptor does not say. A Keychron Q3 keyboard declares a mouse
  collection for its mouse-keys and would be offered as a mouse.
- The kernel's input capabilities do not say either. The same keyboard
  publishes an input device named "… Mouse" with `REL_X`, `REL_Y`, and
  `BTN_LEFT`.
- Both tests miss the opposite case. An MX Master 3S behind an unexpanded Bolt
  receiver declares no usages and publishes no input device.

A HID++ device states its own name and lists its own features. That is
evidence, not inference. Everything else is left to `omnigear report`.

### `omnigear report` writes the support request

`omnigear report [device]` writes the issue that someone without the hardware
needs to add support. With a device, it includes the device's identity, its
whole feature table, and what the driver read. With no device, it lists every
hidraw node on the machine. That is the path for a device that speaks no
protocol here, such as a Razer mouse.

The reply carries the text and a prefilled GitHub issue link. The link is
capped at 8000 characters, because a full feature table and state dump encode
to more than ten thousand and a query string that long is rejected before it
reaches GitHub. Past the cap, the link is trimmed and says where the rest is.
The text is always whole.

The body contains device ids, what answered, the kernel, and the OmniGear
version. It contains no hostname, username, or serial.

### Other programs share the node

A hidraw node delivers every reply to every open reader. A second HID++ client
on the same mouse, such as Solaar, libratbag and Piper, or a vendor plugin,
sees OmniGear's replies mixed with its own. Its writes can revert OmniGear's
moments after they land. The symptom is a device that reads correctly but will
not keep a setting.

`omnigear probe` lists who has each node open:

```bash
omnigear probe
# → { "path": "/dev/hidraw8", "openedBy": [{"pid": 1234, "name": "solaar"}] }
```

A write that does not stick names the other client:

```
device accepted the change but kept 4000 — solaar (pid 1234) also has
/dev/hidraw8 open, which can revert writes
```

Every device in `list` and `get` carries a `conflicts` array, and the panel
shows a red banner that names what it found:

```jsonc
"conflicts": [{ "pid": 1625, "process": "solaar", "label": "Solaar", "known": true }]
```

- `hidraw.Holders` takes every path in one `/proc` walk. The bar polls on a
  timer, and the walk over every pid and descriptor is the whole cost.
- `contenderLabels` maps known processes to the names a user knows, such as
  "Solaar". The kernel caps `comm` at 15 characters, so long names are keyed by
  their truncation: `openrazer-daemon` arrives as `openrazer-daemo`.
- An unknown holder is still reported, with `known: false`.
- OmniGear's own processes are filtered out. Otherwise a command run by hand
  during a poll would trigger the warning.

Only same-user processes are visible in `/proc`, so an empty list means
"nothing found", not "nothing there". The panel shows nothing when the list is
empty. A green "no conflicts" badge would claim more than is known.

## Power: polling, sleep, and off

### Polling wakes the mouse

A wireless mouse must power its radio to answer a request, so every poll wakes
it. One `list` of two mice is 63 HID++ round trips, counted in the transport.
On the default 20-second timer that is about 11,300 an hour, and the mouse never
sleeps. Nothing errors. The battery drains faster. A user noticed only because
their mouse "only falling asleep shortly".

The bar does not need those reads. `hid-logitech-hidpp` keeps a power_supply
per device, fed by reports the device sends on its own. `omnigear battery`
reads charge from sysfs and sends the hardware nothing. It takes 0.003 s,
against 1.149 s for a full read, and wakes nothing.

The timer has two jobs. If the panel is open, or the last full read is more
than fifteen minutes old, it does a full read. Otherwise it reads charge from
the kernel and folds it into the devices from the last full read.

- The merge takes only what the kernel knows. A device on the older `0x1000`
  battery feature gives `capacity_level` as a word and no percentage.
  Overwriting a real figure with "unknown" would show `--` on the bar between
  full reads. A fifteen-minute-old number is better, because charge moves
  slowly.
- The cheap path covers only devices the kernel drives directly. A device
  behind an unexpanded receiver has no power_supply, so the occasional full
  read picks it up.

With the panel shut, an MX Master 3 left alone for 150 seconds answered the
next poll in 3.42 s and reported `presence: asleep`. Under the old timer it
never needed the wake sweep.

### Sleeping devices need a long first ping

A sleeping MX Master 3S took more than half a second to answer its first ping.
The 600 ms call timeout reported it absent, and the 150 ms probe timeout had no
chance. The wake window is 2.5 s, for the first ping only. Once awake, the
device answers in milliseconds. A window that is too short can also return
garbled data. A SUPERSTRIKE once reported its name as `YYYYYYYY`.

Paying 2.5 s on every index would be its own bug. A mouse on index 1 would
spend the whole window finding out that `0xFF` is silent, on every read. `Open`
sweeps the indexes quickly first, and sweeps again patiently only if nothing
answered. The indexes are asked concurrently, so a sweep costs one timeout, not
seven. `discovery.choose` does the same when it picks between nodes.

### Sleep can only be reported after the fact

A request wakes the device, so every answer comes from an awake device. What
can be observed is the cost of waking. A device that ignores the quick sweep
and answers the patient one was asleep. `Device.Woken` records which sweep
won, and the state reports `presence: "asleep"`. The label means "this was
asleep when the poll reached it", which also explains why the poll was slow.

### Off and unreachable come from the kernel

`hid-logitech-hidpp` publishes a power_supply per device. Its `online`
attribute tracks the wireless link: 1 while connected, including idle and
asleep, and 0 once the device is switched off or out of range.
`hidraw.Node.LinkOnline` reads it with one sysfs read and wakes nothing.

Before this, a switched-off mouse cost the whole wake budget on every poll:
2.7 s, every 20 seconds. Now the driver asks the kernel first and returns at
once. On a switched-off MX Master 3, a read went from 2.72 s to 0.03 s, and a
full `list` with a live mouse beside it went from about 1.2 s to 0.36 s.

The kernel knows nothing about a device bound by `hid-generic` or behind an
unexpanded receiver. `LinkOnline` then says it does not know, nothing is
skipped, and a silent device is reported as `unreachable`, not off.

### A device that is off still has a node

The dongle stays plugged in, so the kernel still lists the mouse. Discovery
finds it and nothing answers. A timeout sets `connected: false` and adds
nothing to `errors`, because "Disconnected" says it better. Any other failure,
such as a permission problem or a broken node, is still reported. The panel
shows the device as a name and `Disconnected`, with no controls, and the bar
never picks it.

## HID++ devices

### Working out a wire format

`omnigear probe` lists every hidraw node, whether the catalog claims it, and
which HID++ features the device implements. `omnigear call` sends one raw
request:

```bash
omnigear probe
omnigear call pro-x2-superstrike 0x2202 5 00 00
# → { "featureIndex": 9, "hex": "00 0F A0 0F A0 0F A0 0F A0 02 …" }
```

Every decoder in `drivers/logitech` carries the bytes it was read from in a doc
comment. Keep it that way. These formats are not documented in public, and the
bytes are the only proof.

`omnigear call` sends whatever it is given. A function index does not say
whether it reads or writes. Read a feature's whole function list before you
call into it, and probe unknown functions only on hardware you can afford to
reconfigure. [Easy-Switch](#easy-switch-and-a-warning-about-probing) shows what
goes wrong.

### Sensor and report rate (PRO X2 SUPERSTRIKE)

- `setSensorDpi` rejects a lift-off distance of `0`. The write must carry the
  device's current LOD.
- Feature `0x8061` is per link. Every function but one takes a connection
  type. Asking about the wrong link is not an error. It answers about the
  other link. Reading connection 0 on a mouse at 2000 Hz on its dongle returns
  1000, and caps the offered rates at 1000. `connectionArg` picks the link from
  `hidraw.Node.Bus` and `.Link`.
- `setReportRate` is the exception. It takes the index only. If you pass a
  connection byte, the device reads it as the index and sets a rate nobody
  asked for.

### Bluetooth changes the rules

An MX Master 3S behaves like three different devices:

| | Dongle | Bluetooth |
|---|---|---|
| Node | none of its own | its own, product `046d:b034` |
| Identified by | the name it reports | USB id |
| HID++ reports | short and long | long only |
| Shares the node with mouse input | no | yes |

Each difference first looked like "the device is not there":

- **A short report goes nowhere.** Over Bluetooth the descriptor declares only
  report id `0x11`. A short `0x10` request is not refused. It is never
  delivered. `hidpp.reports` reads the descriptor and uses long reports for the
  whole exchange.
- **A reply can be buried.** When HID++ shares a node with mouse input, a mouse
  in use sends more than a hundred movement reports a second. The wait for a
  reply is bounded by time, not by a count of reports to skip.
- **`Speaks` requires only the long report.** Requiring both would rule out
  every Bluetooth device.

### Onboard mode refuses writes

Feature `0x8100` says who owns a device's settings. In onboard mode the device
runs the profile in its own memory and refuses software writes. In host mode
software owns them. The refusal arrives as error `0x05`, "logitech internal",
which explains nothing, so `writeDPI` reads the mode first and fails with the
real reason.

The mode depends on the connection. A PRO X2 SUPERSTRIKE is in host mode on its
dongle and onboard mode over USB.

`omnigear set <device> profile-mode onboard|host` and a panel toggle change the
mode. It is the one write that changes how the device behaves when OmniGear is
not running, so it happens only as an explicit choice. A failed DPI write says
why and stops. It never flips the device into host mode on its own.

### Battery: percentage or steps

Some devices report which of a few steps they are on, and the number attached
to a step is a boundary, not a reading. An MX Master 3 says `20` when it means
the third of four levels. Showing "20%" invents precision.

The device says which kind it is:

```
0x1004 fn 0 getCapabilities → [supportedLevels, flags, …]
                              flag bit 0 set: it measures a state of charge
0x1000 fn 1 getCapability   → [levels, flags, …, criticalLevel]
                              a few levels means steps, not percent
```

From the two mice this was written against:

```
PRO X2 SUPERSTRIKE  0x1004 fn0 → 0F 0F 02        flags 0x0F, bit 0 set
MX Master 3         0x1000 fn1 → 04 04 C0 78 05  four levels, critical at 5
MX Master 3         0x1000 fn0 → 14 05 00        on level 20, next step 5
```

A device that measures gets `percent`. A device that steps gets `level` and no
percentage, and the UI shows a word. The device's own critical level anchors
the bottom of the mapping.

Everything downstream keys off `batteryScore`: the real percentage where there
is one, an approximate rank for a step otherwise. It picks the icon, picks the
lowest device to speak for the bar, and sizes the bar label, because
"Critical" is wider than "91%". The score is never displayed.

### Reassigning buttons

Feature `0x1B04` holds a table of controls. Each has a control id, a group, and
a mask of the groups it accepts a mapping from. The driver reads the buttons
and their allowed targets from the device, so a new button layout needs no
code.

An MX Master 3S reports eight controls. Five are reprogrammable: middle, back,
forward, gesture, and wheel mode. Left and right have no reprogrammable bit,
which is why nothing here can leave a mouse unable to click.

The masks differ per model. On a 3S every reprogrammable button accepts all
seven targets. On an MX Master 3, back and forward accept only left, right,
back, and forward. "Remap forward to middle" works on one and is refused on the
other, with no special case.

- **A remap of zero is ignored.** Writing `remap = 0x0000` is accepted and does
  nothing. To reset a button, map the control to itself. An untouched device
  reads `0x0000`, so both values count as default on read. `ParseSetting` turns
  `default` into the button's own id, so a reset is not reported as a snapped
  value.
- **The flags byte is zero on write.** Divert and persist each have a "change
  this" bit. With those bits clear, the device leaves both alone. That gives a
  read-modify-write for free, and the driver cannot divert a button by
  accident. A diverted button stops working.
- **Keys and values come from the device.** `button-<slug>` is the only setting
  whose keys and values come from hardware, so `cmdSet` checks both against the
  read it already did. Asking for a button the device will not reassign names
  the ones it will.

The read costs about 170 ms per poll: one `getCount`, one `getCidInfo` per
control, and one `getCidReporting` per reprogrammable control. That is 14 round
trips on a 3S. The CLI is a fresh process each time and has nowhere to cache.

### A diverted wheel is a dead wheel

Both wheel features can send their movement as HID++ notifications instead of
scroll events. OmniGear does not read those notifications, so a diverted wheel
stops working unless another client listens.

The driver never touches the divert bit of the hi-res wheel (`0x2121`). A
read-modify-write carries it over. The thumbwheel (`0x2150`) gets a control,
because the mouse this was written on arrived already diverted and its
thumbwheel did nothing. Writing reporting mode 0 fixed it. The device reports
no error in that state, so the control is the only way to see and undo it.

The thumbwheel's invert flag is not exposed. Setting it sticks (`fn 1` reads
back `00 01`) and changes nothing about scroll direction, so it seems to apply
only to the diverted stream. Writes carry the byte over unchanged.

### Easy-Switch, and a warning about probing

An MX device pairs with three hosts. `0x1814` owns the count, the current slot,
and the switch. `0x1815` adds per-slot pairing status and each host's stored
name.

A device can have `0x1814` without `0x1815`. The MX Master 3 switches hosts and
has no `0x1815`. The first version reported every slot on it as unpaired,
because "the call failed" and "the slot is empty" looked the same.
`Hosts.PairingKnown` separates them, and the panel offers every slot on such a
device. Slots are 0-based on the wire and 1-based above the driver, matching
the buttons on the underside.

The warning is about `0x1815`. Fn 3 reads a host's name, and fn 4 writes it
with whatever bytes it gets. Calling fn 4 with no name bytes, to see what it
returns, stores an empty name. That happened on a real mouse, and the name had
to be written back:

```bash
omnigear call mx-master-3s 0x1815 4 00 00 6d 65 67 61 74 72 6f 6e  # "megatron"
```

## Bluetooth headphones

### Headphones are not HID

A Sony WH-1000XM3 never appears under hidraw. It is an audio sink, a headset
profile, and a vendor RFCOMM service, and none of those is a node in `/dev`. A
headset takes a different route through every layer:

| | Mice and keyboards | Headphones |
|---|---|---|
| Found by | hidraw enumeration | BlueZ over D-Bus (`transport/bluez`) |
| Identified by | USB id from sysfs | vendor and product from the Device ID record (`Modalias`) |
| Addressed by | `Device.Node` | `Device.Address`, the MAC |
| Spoken to | HID++ on the node | Sony's protocol on RFCOMM (`transport/rfcomm`, `transport/mdr`) |
| Cheap battery | kernel power_supply | BlueZ `Battery1`, fed over the hands-free link |

The catalog does not care. The XM3's modalias `usb:v054Cp0CD3` matches as USB
id `054C:0CD3`.

The RFCOMM channel is looked up on each connection by asking the headset's SDP
server for the service UUID. An XM3 answered channel 15.

An RFCOMM service refuses a second client with `EBUSY`. The error names nobody,
so the card says that another program has the control channel, not which one.

Straight after a mode change the headset plays a voice prompt, and a connection
opened during it sometimes gets no answer. `mdr.Open` retries the init once.

### Noise control writes the whole record

Noise cancelling, ambient sound, the ambient level, and focus on voice are one
message, so a write reads first and carries the other fields over.

- While noise cancelling, the headset reports the ambient level and voice
  filter as zero. Ambient at level 0 is off, so a switch from noise cancelling
  to ambient must supply a level. The driver asks for the full level.
- The on byte must be written as `11`. Written as `01`, as the headset reports
  it, the mode changes and the new level is ignored.

### The codec decides whether the XM3 equalizer works

Over LDAC or aptX an XM3 still reports its equalizer, but refuses every change
with `99 01 01 01`. Its answer to "is the equalizer available" (`52 01`) is
`00` either way. The codec record (`18 00`) is what tells. Each value was read
with PipeWire switched to that codec: SBC `01`, AAC `02`, LDAC `10`, aptX `20`,
and aptX HD `21`. The panel names the codec in the way instead of showing
sliders that do nothing.

### The codec belongs to the host

Both ends offer what they support and PipeWire picks, as the card profile.
`a2dp-sink-aac` is AAC. `drivers/a2dp` switches it through `transport/pactl`
and says nothing to the headset, so any Bluetooth headset can reuse it. The
codec is named only in the profile description, because the default profile
is plain `a2dp-sink` whatever the codec. `set-card-profile` returns before the
switch lands, so the write waits until the card reports the new profile.

A switch removes the audio output for a moment, and a media player whose
output disappears pauses. Spotify paused half a second in, before the switch
finished. `transport/mpris` records which players are playing before the switch
and resumes the ones that paused, over the session bus. It repeats play until
the player reports playing, because Spotify ignored a play sent too soon after
a pause. A player that was already paused, or kept playing, is left alone.

### Headphones on the bar

A mouse and a headset each get their own reading and their own primary
(`primaryMouse`, `primaryHeadset`). A dying headset is not less urgent because
the mouse is lower.

Headphones come and go more often than a mouse, and with the panel shut full
reads are fifteen minutes apart. `omnigear battery` lists every connected
headset, with `percent: -1` when BlueZ has no charge, and the bar does a full
read when that set changes. Otherwise headphones just switched on would be
missing from the bar for up to fifteen minutes.

### The WH-1000XM4 is an XM3 with exceptions

It publishes the same control service (`96cc203e…`) and answers every read
with the same layout, so it shares the driver as `drivers/sony.MDRXM4`. The
differences:

- It ignores the XM3's band write (`58 01 a0 06 …`), over AAC and LDAC alike.
  Sent to preset `ff`, the selected one, the bands are taken. They go to that
  preset if it is editable, otherwise to manual with the headset moved onto
  it.
- It accepts equalizer changes over LDAC, so its equalizer is not gated on the
  codec.
- **Auto power off.** Both use one record (`f6 04`, set with `f8 04`). An XM4
  keeps only `11 00` (never) and `10 00` (when taken off), and reads anything
  else back as `10 00`. An XM3 keeps `00 00` to `03 03` for its timers and
  `11 00` for never, and ignores anything else. The driver lists each model's
  choices, and the panel draws what it gets under a Power tab.
- **Touch panel.** The switch is in `d6 d2` on an XM3 and `d6 d1` on an XM4.
  Each model answers the other record too, with a value it will not change, so
  a read of the wrong one looks plausible. The driver is told which per model.

## Headsets behind a base station

### The Arctis Nova Pro Wireless

OmniGear never talks to the headset directly. The base station is a USB device
of its own, and the headset's status and settings are whatever the base
station reports. It owns two hidraw nodes, and only one is useful:

| Interface | Report descriptor | Carries |
|---|---|---|
| 3 | consumer control, report `01` | media keys |
| 4 | vendor pages `0xFFC0` and `0xFF00`, reports `06` and `07` | the protocol |

`drivers/steelseries` implements `model.Speaker` and picks interface 4 by the
`0xFFC0` usage page in its descriptor. Nothing is opened.

The protocol (`transport/arctis`) is two queries and a few one-byte setters.
`06 b0` returns the base station's status record and `06 20` returns an audio
settings record. Each setter's field was found by writing it and watching the
records change, then restoring every setting. A write gets no answer, so each
one is verified by reading back. Report `07` is the base station speaking
unprompted, mixed in with replies, so a call waits on time, not on a report
count.

Linux-Arctis-Manager's device description (GPL-3.0) was the starting point for
which commands exist. The noise mode (`bd`) and transparency level (`b9`)
setters are not in it. They came from the setter layout: `bf`, `c1`, and `c3`
write bytes 11, 12, and 13 of the status record, so `bd` writes byte 10, the
noise mode.

### What the base station makes possible

- **Asking wakes nothing.** The base station is powered by its cable and keeps
  the headset's status, so `omnigear battery` asks it with one query. A driver
  that can do this implements `model.BatteryReader`.
- **Off is known, not guessed.** The status record says whether the headset is
  on the link, so a switched-off headset is `presence: off` without a timeout.
  The battery reply lists the base station either way, with `connected`, and
  the bar treats a change as an arrival or a departure.
- **The link is the connection.** The base station is in the receiver table as
  a `BaseStation`, so the card says 2.4 GHz, not Wired.
- **The port is the base station's.** Byte 2 of the settings record says
  which of its two USB-C ports it is listening on, and the card names it,
  USB-1 or USB-2, beside the link.

### Switching the wireless mode drops the headset

The status record reports the headset off at once. The base station sends
`07 b5` and `07 b7` as it re-pairs, and the headset is back some seconds later.
The write waits up to fifteen seconds for it, so the panel does not show a
headset that is off after a change that worked.

### The screen is OmniGear's to turn off

The base station's OLED has no off and no timeout. `06 85 NN` sets its
brightness from `01` to `0a`, and `00` is the dimmest, not dark. So OmniGear
draws the screen dark instead. `06 93` is a feature report, 1024 bytes, that
draws a 64 by 64 tile at a given column and row, one bit a pixel. Two all-zero
tiles, at columns `00` and `40`, blank the whole screen, and it stays dark
until something changes it. `06 95` hands the screen back, and the base
station draws its own at once. ggoled was the starting point for the framing.

Touching the base station also brings its screen back, with no request. Each
step of the volume knob sends `07 25 NN` (`NN` is byte 3 of the settings
record), the dial sends `07 45`, and the mute button sends `07 bb`. A power-up
draws the screen too.

None of this can be read back, so the state lives in OmniGear's own files:

- `~/.config/omnigear/screen.json` holds the timeout, `never` by default,
  and the screensaver, `blank` by default.
- `$XDG_RUNTIME_DIR/omnigear/screen.json` holds whether OmniGear believes the
  screen is off, and when that last changed. `set` and `watch` both write it,
  whole and atomically. Each write records a real event, so the last one wins.

`set` verifies a screen write against these files, not the hardware.

`watch` runs the countdown, since it is already listening. It starts the
countdown on connect, and restarts it on every report `07`, which also means
the screen is back. When the timeout passes, it blanks the screen, and the
panel shows it off at its next full read. `watch` rereads both files at least
every five seconds. A timeout chosen in the panel takes effect without a
restart and starts the countdown again. Turning the screen on from the panel
counts as a touch.

A press of the knob without a turn brings the screen back but sends nothing.
The belief stays off until the next report, which corrects it.

### Off shows a screensaver

Off draws the chosen screensaver: Blank, a DVD-style bouncing logo, or the
Omarchy or OmniGear wordmark. The screen is an OLED, and a picture that stays
lit leaves a ghost, so every screensaver but Blank moves. The art bounces off
the edges on a diagonal, a pixel each way every frame for the DVD logo and
every third frame for the wordmarks, about 8 pixels a second. It is narrower
than the screen by a different amount each way, so in time it crosses all of
it.

A frame is the same two `06 93` tiles, and each takes 18 ms to send, so a
frame takes 36 ms however little of it changed. The bus allows 28 frames a
second. The screensaver draws 25, which keeps the link about nine tenths
busy: 10 looked jerky beside it on the base station. Each frame is due 40 ms
after the last, counted from the first, so the time a frame takes to send
does not slow the rate. A frame drawn late skips to the one that is due.

Only `watch` moves a screensaver, since it is the one process that keeps
running. `set screen off` still draws the screen dark, at once, and `watch`
puts the screensaver on it at its next read of the belief, within five
seconds. When the timeout passes, `watch` starts the screensaver itself.
Blank is drawn once, as before.

While a screensaver moves, `watch` reads the base station's reports between
frames, so a touch is seen within one frame. The base station has already
drawn its own screen by then, but a frame sent at the same moment can cover it
again, so `watch` stops and sends `06 95` anyway. It rereads the files at most
every 500 ms. A belief that has turned on, from the panel, stops it and sends
`06 95` again for the same reason. A different screensaver takes over from its
first frame.

A press of the knob sends nothing, so it brings the base station's screen back
only until the next frame covers it.

`scripts/screensaver-art` draws the art as 1-bit PBM files, which the driver
embeds. It needs ImageMagick and librsvg, and reads the Omarchy wordmark from
the installed Omarchy. Nothing at runtime does.

### Arctis Manager takes the interface away

Arctis Manager drives the base station through libusb, which unbinds usbhid
from interface 4 and binds usbfs. The protocol's hidraw node disappears while
it runs, and only the media-key node is left. The driver looks for an
interface bound to usbfs and names whoever holds the USB device open. Unlike a
shared hidraw node, nothing can be read at all.

## Sonar

SteelSeries Sonar gives a headset separate outputs for game, chat, media, and
everything else, and the base station's ChatMix dial balances game against
chat. OmniGear builds it in two halves that meet at the sound server.

### The outputs are PipeWire filter-chains

Each channel is a virtual sink that applications play into, and it plays into
the headset's output. The channels are declared in a drop-in,
`~/.config/pipewire/pipewire.conf.d/omnigear-sonar.conf`, so PipeWire creates
them at login whether or not the bar is running, and an application keeps its
channel across reboots. Sonar is on when the drop-in exists. PipeWire reads a
drop-in only at startup, so switching Sonar restarts PipeWire and every stream
drops for a moment. The switch says so.

The headset's output is found by the USB ids of its sound card, which are the
base station's own, so the catalog entry is enough.

- **PipeWire comes back in two halves.** The loopbacks appear at once. The
  headset's output waits for WirePlumber to find the sound card, a second or so
  later. Waiting on the channels alone returned a half-started sound server,
  and the next request failed with "Not supported". The wait is for both.
- **The loopback cannot fall back.** Its playback side is passive, so an idle
  channel does not hold the headset open. `node.dont-fallback` is set, because
  with the headset unplugged and Game as the default output, Game would fall
  back into itself. `node.linger` is set too. Without it, losing the headset's
  output destroys the channel, and it stays gone until PipeWire restarts. With
  it, the channel waits unlinked and relinks when the headset returns.

### Each app picks a channel

The Sonar tab lists what is playing and moves it with
`omnigear set <device> sonar-app-<app> <channel>`. A music player can open a
new stream for every track, and WirePlumber remembers an output per application
name, so the choice applies to the application and outlives the stream. A move
is asynchronous, so the write waits for it to land before the verifying read.
The list is as fresh as the last full read, which is every poll while the panel
is open.

An app names its own stream, often after its audio library. Discord's voice
plays as "WEBRTC VoiceEngine" and Valheim as "FMOD Ex App". The list shows a
better name where it can find one:

1. A process Steam launched carries `SteamAppId` in its environment, and
   Steam's manifest for that app has the store name.
2. Otherwise, the stream's process binary is matched to a desktop entry, by
   window class and then by the program its `Exec` line runs.

Only the label changes. Routing still uses the stream's own name, because that
is what WirePlumber remembers. Two games on the same engine, both
"FMOD Ex App", share one remembered channel.

### Each channel has its own equalizer

While Sonar is on, the base station hands its equalizer to the computer. Its
own menu says the EQ is "on Sonar" and will not change it. So each channel is a
filter-chain of ten octave-wide peaking filters, and `pw-cli set-param` changes
a gain live with no restart.

The chain cannot report a change reliably. A change made before the chain has
played anything since PipeWire started is held and applied when audio starts,
but reads back as the old value until then. The curves therefore live in
`~/.config/omnigear/sonar-eq.json`. A read reports that file, and each change
rewrites the drop-in from it without a restart, so the curve survives the next
PipeWire start. Moving a band on a fixed preset first copies the preset's curve
to Custom, as Sony's equalizer does.

### Volume, mute, and the dial

The dial scales each loopback's stream, not its sink. The sink's volume is the
listener's own, set in any mixer or with a knob in the Sonar tab
(`sonar-volume-<channel>`). The dial works on top of it, so turning toward Game
lowers chat without losing chat's level, and neither moves the other's number.

The knobs apply while they are dragged, which `set` cannot keep up with. `set`
reads the whole device before and after, and the panel greys out while it runs,
which cancels the drag. A dragged knob uses `omnigear sonar-volume` instead,
which only tells the sound server and takes about ten milliseconds. The newest
level is held while one is in flight. Letting go sends an ordinary `set`, so
the knob ends on a verified value. A double-click mutes the channel's sink
(`sonar-mute-<channel>`) and leaves its volume alone.

### `omnigear watch` is the one long-lived process

The dial is reported only as it turns: `07 45 gg cc`, game and chat each 0 to
100. Turning toward one side lowers the other, never both. There is no reading
to ask for, so something must be listening. `omnigear watch` runs as long as
the bar does, applies each position, and prints it as one JSON line for the
panel.

- It finds the base station by USB id and report descriptor only, and opens
  nothing else. Waiting for a base station does not ping every mouse every few
  seconds.
- The dial reports about every 40 ms while turning, faster than `pactl` runs.
  One worker applies the newest position and drops the rest.
- It sets `PR_SET_PDEATHSIG`, so a crashed shell takes it down instead of
  orphaning it.

The base station reports the dial only when the computer has told it Sonar is
on. `06 8d 01` shows the Sonar icon on its screen, and `06 49 01` enables
ChatMix, so the dial starts reporting. Neither is answered, and neither shows
in a record that can be read. The base station is powered by its cable and
forgets both on any replug, reboot, or cable move. So `watch` announces Sonar
on each connect, from whether the drop-in exists, and the Sonar switch
announces it on each change. A replug always re-enumerates the node, which
ends one `watch` and starts the next. nova-chatmix-linux's command table was
the starting point for these commands.

The base station never says where the dial is, only that it moved. WirePlumber
restores stream volumes by node name, so the last position applied comes back
after a restart.

The mute button is announced the same way. `07 bb 01` is muted and `07 bb 00`
is live, sent on each press. Byte 9 of the status record says the same, and
`bb` is that byte's place in the setter layout. The values were confirmed by
pressing once from live and checking the light. An earlier attempt, read from
hurried presses, had them reversed. `watch` prints a `mic` event on each
change, so the bar's red microphone appears on the press, not at the next
poll. `watch` also reads the mute once at start, and takes it from any status
reply on the node, so a missed press is corrected at the next battery poll. A
full read reports it as `micMuted`, under the `mic-mute` capability, only while
the headset is on.

## Why Go

The first version was Rust. Nothing here needs what Rust offers: a 20-second
poll, a few hundred bytes of I/O, and little concurrency. The project needs to
be readable by its maintainer and cheap to add a device to. The catalog is
data, and it reads better as plain Go structs than as
`&'static [Capability]`. Cross-compiling for another architecture takes one
environment variable.

Go has no exhaustive matching, so a new `Capability` does not make the
compiler point at every place that must handle it.
[Path B](ADDING-A-DEVICE.md#path-b-add-a-capability) lists those places. If the
capability list keeps growing, add a test that walks the catalog.
