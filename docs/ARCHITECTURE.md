# Architecture

OmniGear is two pieces with one seam between them:

```
  qml/          the Omarchy bar widget — draws things, talks to no hardware
    │
    │  JSON over stdout, one object per invocation
    ▼
  omnigear      a Go CLI — owns every byte that reaches a device
```

The seam is deliberate. QML runs inside the long-lived `omarchy-shell` process,
where a blocking read on a wedged USB device would freeze the desktop. The CLI
is a short-lived process with bounded timeouts; the worst a broken device can do
is make one poll come back empty.

## Finding a device in the source

The tree mirrors the README's support table — category, then brand, then model:

```
internal/devices/
  mice/
    logitech/
      logitech.go                 every Logitech mouse, as catalog entries
      prox2superstrike.go         what is specific to this one model
    razer/
    corsair/  glorious/  finalmouse/
  keyboards/
    logitech/  razer/  keychron/
  headsets/
    steelseries/  logitech/  razer/  hyperx/
    sony/  apple/  google/  nothing/  oneplus/
```

A device's runtime id is that same path, so a bug report names its own source
directory:

```
mouse/logitech/pro-x2-superstrike#5f-ba-c9-65
└─┬─┘ └───┬──┘ └────────┬───────┘ └────┬────┘
category  brand       model        serial
```

## The three layers under `internal/`

| Layer         | Knows about                              | Reused by                       |
|---------------|------------------------------------------|---------------------------------|
| `transport/`  | wire formats — hidraw nodes, HID++ 2.0   | every brand that speaks it      |
| `drivers/`    | how to read/write a family of devices    | every model in that family      |
| `devices/`    | which models exist and what they can do  | —                               |

The split is what keeps the catalog cheap. `transport/hidpp` is the HID++
protocol and nothing else, so a Logitech *keyboard* will use it unchanged.
`drivers/logitech` turns that protocol into battery/DPI/polling-rate
reads, so most new Logitech mice are a catalog entry and no new code.

## Capabilities are the contract

A catalog entry declares what a model can do:

```go
Capabilities: []model.Capability{model.CapBattery, model.CapDPI, model.CapPollingRate},
```

Everything downstream keys off that list. The driver only probes what is
claimed. The CLI only emits state for what was probed. The UI renders one
control per capability. **Adding a device does not mean writing UI** — that is
the whole point of the arrangement.

## Adding a model

For a model whose family already has a driver:

1. Add a `model.Entry` to `internal/devices/<category>/<brand>/<brand>.go`.
2. Give it the USB ids you have *observed* — run `omnigear probe` with the
   device plugged in. Never guess an id; a wrong one binds a driver to
   somebody else's hardware.
3. Set `Support` honestly: `SupportPartial` until every capability it claims
   works.

For a model that needs new behaviour, add a file beside it
(`prox2superstrike.go`) and, if a whole new protocol is involved, a
`transport/` package and a `drivers/` package.

## Adding a brand

A brand that does not speak an existing protocol needs a transport. The shape to
follow is `transport/hidraw` + `transport/hidpp`: enumeration and framing in
the transport, meaning in the driver.

## Working out a wire format

`omnigear probe` lists every hidraw node, whether the catalog claims it, and
which HID++ features the device actually implements. `omnigear call` then sends
one raw request:

```bash
omnigear probe
omnigear call pro-x2-superstrike 0x2202 5 00 00
# → { "featureIndex": 9, "hex": "00 0F A0 0F A0 0F A0 0F A0 02 …" }
```

Every decoder in `drivers/logitech` carries the bytes it was read off
in a doc comment. Keep that up: these formats are not documented anywhere
public, and the bytes are the only proof.

Two things learned the hard way on the PRO X2 SUPERSTRIKE, both noted in the
source:

- `setSensorDpi` rejects a lift-off distance of `0`. The write has to carry the
  device's current LOD, not a placeholder.
- Feature 0x8061 is *per-link*. Every function but one takes a connection type,
  and asking about the wrong one is not an error — it answers about the other
  link. Reading connection 0 on a mouse running 2000 Hz on its dongle returns a
  confident, wrong 1000, and caps the offered rates at 1000 so the real ones
  look unsupported. `connectionArg` picks the link; `hidraw.Node.Bus` and
  `.Link` decide which.
- `setReportRate` is the exception: index only, no connection byte. Passing one
  is not refused — the device takes the first byte as the index and sets a rate
  nobody asked for.

## One device, many nodes, and some of them lie

A device owns several hidraw nodes and can own them under more than one USB id
at once. Plug a wireless mouse in with a cable while its dongle is still in the
machine and both sets exist side by side — but only one of them carries the
mouse. The other keeps answering nothing, which surfaces as
`connect: device did not answer`.

`discovery.choose` sorts that out with two filters, cheapest first:

1. **The report descriptor.** `hidpp.Speaks` looks for the short and long HID++
   report ids in `report_descriptor`. Of the four nodes a wired PRO X2
   SUPERSTRIKE owns, exactly one declares both. Nothing is opened.
2. **A ping.** If more than one node survives, only a ping distinguishes the
   live one from the leftover. They are probed concurrently and the first
   answer wins, because a live device replies in milliseconds while a stale
   node costs the whole probe budget.

Nodes are grouped by serial, not by USB id, and the serial is normalised first:
the same mouse reports `5f-ba-c9-65` through its dongle and `5FBAC965` over
USB. Without that, plugging in a cable would split one mouse into two devices.

## Devices with no node of their own

A dongle the kernel recognises is expanded into one hidraw node per paired
device, and those are matched by USB id like anything else. A dongle it does
not recognise stays a single node, and the devices behind it are invisible to a
USB-id match — a Logi Bolt (`046d:c548`) is exactly this case, because
`hid-logitech-dj` has no entry for that product id and `hid-generic` binds
instead.

`discovery.behindReceivers` covers them:

- only receivers the kernel did **not** expand are scanned, since scanning an
  expanded one would find the same device twice under two identities;
- the slots are probed **concurrently** — an empty one costs the whole wake
  timeout, and waiting out five in series to find the sixth device would make
  every poll crawl;
- the receiver's own count (HID++ 1.0 register `0x02`) ends the scan early once
  that many have answered. It is an optimisation, not a gate: a receiver that
  answers strangely falls back to scanning every slot.

Such a device identifies itself by **name** rather than by USB id, so its
catalog entry carries `Names` instead of `USB`, and `model.Device.Index`
addresses it on the receiver's node.

## The same mouse is a different animal on each connection

An MX Master 3S is three devices as far as this code is concerned, and the
differences are not cosmetic:

| | dongle | Bluetooth |
|---|---|---|
| node | none of its own | its own, product `046d:b034` |
| identified by | the name it reports | USB id |
| HID++ reports | short **and** long | **long only** |
| shares the node with mouse input | no | yes |

Three things follow, each of which looked like "the device is not there":

- **A short report goes nowhere.** Over Bluetooth the descriptor declares only
  report id `0x11`; a short `0x10` request is not refused, it is simply never
  delivered. `hidpp.reports` reads the descriptor and the conversation falls
  back to long reports for the whole exchange.
- **A reply can be buried.** Where HID++ shares a node with the device's
  ordinary input, a mouse in use emits well over a hundred movement reports a
  second. Waiting for a reply is bounded by *time*, not by a count of reports
  to skip, or the answer is discarded while the device is answering perfectly.
- **`Speaks` requires only the long report.** Requiring both would rule out
  every Bluetooth device.

**Sleeping devices need a long first word.** A sleeping MX Master 3S took over
half a second to answer its first ping — the ordinary 600ms call timeout
reported it as absent, and the 150ms probe timeout never stood a chance. The
wake window is 2.5s, and only for that first ping; once awake it answers in
milliseconds. A too-short window does not only lose the device — it can return
*garbled* data, which is worse; a SUPERSTRIKE once reported its name as
`YYYYYYYY` under the short budget.

Paying that budget on every index would be its own bug: a mouse answering on
index 1 would spend the whole wake window discovering that `0xFF` is silent, on
every read. `Open` sweeps the indexes quickly first and only repeats the sweep
patiently when nothing at all answered. `discovery.choose` does the same when
picking between nodes.

## Onboard mode refuses writes

Feature 0x8100 says who owns a device's settings. In **onboard** mode it runs
the profile in its own memory and refuses software writes; in **host** mode
software owns them. The refusal arrives as error 0x05, "logitech internal",
which explains nothing — so `writeDPI` reads the mode first and fails with the
real reason.

The mode is not fixed. A PRO X2 SUPERSTRIKE is in host mode on its dongle and
onboard mode over USB, so the identical write succeeds or fails depending on
which cable is in.

The mode is settable — `omnigear set <device> profile-mode onboard|host`, and a
toggle in the panel — but only ever as an explicit choice. It is the one write
that changes how the device behaves when OmniGear is not running, so a failed
DPI write says why and stops, rather than quietly flipping the device into host
mode to get its way.

## You are not the only one talking to the device

A hidraw node delivers every reply to **every** open reader. A second HID++
client on the same mouse — Solaar, libratbag/Piper, a vendor plugin — is not
partitioned from OmniGear: its replies arrive interleaved with ours, and its
writes can revert ours moments after they land.

The signature is a device that reads perfectly but will not keep a setting.
It is unguessable from the outside, so both places that can see it say so:

```bash
omnigear probe    # every node lists `openedBy`
# → { "path": "/dev/hidraw8", "openedBy": [{"pid": 1234, "name": "solaar"}] }
```

and a write that does not stick names the other client in its error rather
than just reporting the stale value:

```
device accepted the change but kept 4000 — solaar (pid 1234) also has
/dev/hidraw8 open, which can revert writes
```

Only same-user processes are visible through `/proc`, so an empty list means
"nothing found", not "nothing there". The panel says nothing at all when the
list is empty, for exactly that reason: silence here is not a clean bill of
health, and a green "no conflicts" badge would be claiming more than is known.

Since neither `probe` nor a failed write is something a user reads unprompted,
every device in `list` and `get` also carries a `conflicts` array, and the
panel puts a red banner above the cards naming what it found:

```jsonc
"conflicts": [{ "pid": 1625, "process": "solaar", "label": "Solaar", "known": true }]
```

Three things about how it is gathered:

- **One `/proc` walk, not one per device.** `hidraw.Holders` takes every path
  at once. The bar polls this on a timer, and the walk — every pid, every open
  descriptor — is the whole cost of the answer.
- **Known programs are named properly.** `contenderLabels` maps a process to
  what a user would call it, so the banner says "Solaar" rather than "solaar".
  The kernel caps `comm` at 15 characters, so longer names are keyed by their
  truncation: `openrazer-daemon` arrives as `openrazer-daemo`.
- **An unrecognised holder is still reported**, with `known: false` and its
  process name. Whatever it is, it is still sharing every reply.

OmniGear's own processes are filtered out. The CLI is re-run on every poll, so
a hand-run command overlapping the bar's would otherwise have the panel
warning about OmniGear.

## Polling is not free: it wakes the mouse

A poll is a conversation. A wireless mouse has to power its radio and
transmit to answer one, so every poll wakes it — there is no passive read on
that path. Measured by counting round trips in the transport, one `list` of
two mice is **63 HID++ round trips**. On the default twenty-second timer that
is about 11,300 an hour, and a mouse that never gets to sleep.

It is easy to miss, because the symptom is absence: nothing errors, the
battery just goes down faster. It was noticed here only when someone observed
their mouse "only falling asleep shortly".

The fix is that the bar does not need any of it. `hid-logitech-hidpp` keeps a
power_supply per device, fed by reports the device sends of its own accord,
so `omnigear battery` reads charge from sysfs and sends the hardware nothing:
**0.003s against 1.149s**, and no wake at all. The expensive read — DPI,
buttons, hosts, wheel — is only worth doing while the panel is open and
someone is looking at it.

So the timer has two jobs. Panel open, or no full read in the last fifteen
minutes: a full read. Otherwise: charge from the kernel, folded into the
devices from the last full read.

Two details that matter:

- **The merge only takes what the kernel actually knows.** A device on the
  older 0x1000 battery feature gives `capacity_level` as a word and no
  percentage; overwriting a real figure with "unknown" would put `--` on the
  bar between full reads. A number fifteen minutes old is better than no
  number, because charge moves slowly and the reading it replaces was true.
- **It covers only devices the kernel drives directly.** One behind a
  receiver the kernel did not expand has no power_supply, and identifying it
  needs HID++ anyway, so it is left out of the cheap path and picked up by
  the occasional full read.

Confirmed end to end: with the panel shut, an MX Master 3 left alone for 150
seconds answered the next poll in 3.42s and reported `presence: asleep`.
Under the old timer the same mouse never once needed the wake sweep.

## Sleeping, and why it can only be reported in the past tense

A sleeping device cannot be observed while sleeping. The only way to ask it
anything is to send it a request, and that wakes it — so by the time there is
an answer, the answer comes from an awake device. There is no poll that
observes sleep.

What can be observed is the cost of the waking. `Open` sweeps the indexes
quickly first and only repeats the sweep patiently when nothing answered at
all; a device that ignores the quick sweep and answers the patient one was
asleep, and an awake one never needs the second pass. `Device.Woken` records
which sweep won, and the state reports `presence: "asleep"`. The label is
retrospective on purpose: it means "this was asleep when the poll reached
it", which is also the explanation for why that poll was slow.

The distinction that is *not* retrospective is off versus unreachable, and it
does not come from the device at all. `hid-logitech-hidpp` publishes a
power_supply per device it drives, whose `online` attribute tracks the
wireless link rather than the battery: 1 while connected, 0 once switched off
or out of range, and 1 throughout an ordinary idle — a sleeping mouse is
still a connected one. `hidraw.Node.LinkOnline` reads it, which costs a sysfs
read and wakes nothing.

That is worth more than a nicer label. Before it, a switched-off mouse cost
the whole wake budget on every single poll to rediscover that it was still
off: 2.7s, twenty seconds apart, forever. Now the driver asks the kernel
first and returns immediately. Measured on a switched-off MX Master 3, a read
went from 2.72s to 0.03s, and a full `list` alongside a live mouse from about
1.2s to 0.36s.

The kernel has no opinion about a device bound by `hid-generic`, or one
behind a receiver it did not expand. Then `LinkOnline` says so rather than
guessing, nothing is skipped, and a device that stays silent is reported as
`unreachable` rather than as off.

## A device that is off still has a node

Switch a wireless mouse off and its node stays: the dongle it is paired to is
still plugged in, so the kernel still lists it. Discovery finds it, the driver
tries to talk to it, and nothing answers.

Two things follow.

**The sweep has to be bounded.** Trying each device index in turn at the wake
budget is seven timeouts — eighteen seconds, longer than the poll interval,
which leaves the UI reading forever and never finishing. The indexes are asked
concurrently, so the whole sweep costs one timeout rather than seven.

**Silence is a state, not an error.** A timeout sets `connected: false` and
adds nothing to `errors`, because "Disconnected" says it better than an error
message would. Any other failure — a permission problem, a broken node — is
still reported. The panel shows such a device as a name and `Disconnected`,
with no controls to operate and no tabs, and the bar never picks it to speak
for the widget.

## Writes are verified, never assumed

A device can accept a write and quietly ignore it. `omnigear set` therefore
reads the device back and compares:

| Outcome                        | Result                                      |
|--------------------------------|---------------------------------------------|
| value matches what was asked   | `ok`                                        |
| value changed, but rounded     | `ok`, with `applied` and a `note`           |
| value did not change           | **error** — "accepted the change but kept …"|

The UI shows what the hardware says, not what it was told.

One setting is exempt, and only one. Switching Easy-Switch host (`0x1814`)
succeeds by making the device leave: the verifying read finds nothing, which
is indistinguishable from the write having failed. `SettingKey.Verifiable`
marks it, `cmdSet` stops at "the device accepted it", and the QML side treats
an `ok` reply carrying no device as a cue to re-read rather than as a failure.
Any future write with the same shape belongs there too; nothing else does.

## Permissions, and the failure with no symptom

hidraw nodes are `crw-------` root-only by default. Without a udev rule
granting access, every device is invisible: it enumerates, the kernel binds it
and drives it as a mouse, and this plugin cannot open it to ask it anything.
Nothing errors. The device just is not there.

`udev/60-omnigear.rules` is the fix, tagging the vendors this project drives
with `uaccess` so logind hands the node to whoever is logged in at the seat.
That is preferred over a world-writable mode or a group to join, because the
ACL follows the session rather than standing open. It is scoped by vendor: a
blanket rule over `SUBSYSTEM=="hidraw"` would hand every HID device on the
machine, firmware-update endpoints included, to anything in the session.

This was found the hard way. The project had been working on a machine that
happened to have Solaar installed, and it was **Solaar's** udev rule granting
the access. Uninstalling Solaar left already-plugged devices working — a node
keeps the ACL it was given when it was added — while every newly plugged
device silently vanished. Nothing here should depend on another project being
installed, and now nothing does.

Because the symptom is absence, `discovery.Unreadable` looks for the cause
directly: nodes that carry HID++ and return `EACCES` on open. `list` reports
them and the panel prints the fix. A node that is merely busy is not reported,
since no udev rule would help.

## Devices nobody catalogued

A device that is not in the catalog still shows up, described by what it says
about itself: the name it reports, how it is attached, and a capability list
derived from its own feature table rather than from anyone's testing. The
existing driver then reads and writes it unchanged, because the driver keys
off the capability list and does not care who wrote it. Battery and DPI on an
unknown Logitech mouse work for exactly that reason.

Such an entry is marked `Discovered` and `SupportUnsupported`, and the panel
tints its card and labels it, because "the device advertises this feature" is
a much weaker claim than "someone confirmed this works on this model".

**Only HID++ devices are found this way, and that is a deliberate limit.**
Identifying an arbitrary mouse from outside cannot be done without guessing:

- The HID report descriptor does not say. A Keychron Q3 keyboard declares a
  mouse collection, because it has mouse-keys, and would be offered as an
  unsupported mouse.
- The kernel's input capabilities do not say either, for the same reason: that
  keyboard publishes an input device named "… Mouse" carrying `REL_X`,
  `REL_Y` and `BTN_LEFT`.
- And both miss the opposite case. The node an MX Master 3S is reached through
  behind an unexpanded Bolt receiver declares no usages at all and publishes
  no input device.

A HID++ device will state its own name and list its own features, which is
evidence rather than inference. Everything else is left to `omnigear report`.

## Reporting a device

`omnigear report [device]` writes the issue somebody else would need in order
to add support for hardware they do not own.

With a device, it is that device's identity, its whole feature table, and what
the driver managed to read. With no device, it is every hidraw node on the
machine — which is the path for a mouse this project cannot identify at all, a
Razer say, that never appears as a device because it speaks no protocol here.

The reply carries both the text and a prefilled GitHub issue link. The link is
capped at 8000 characters because a full feature table plus a state dump
encodes to well over ten thousand and a query string that long gets rejected
between here and GitHub; past the cap the link is trimmed and says where the
rest is. The text is always returned whole.

The body contains device ids, what answered, kernel and version, and nothing
about the person sending it — no hostname, no username, no serial.

## Reassigning buttons

Feature `0x1B04`. The device holds a table of controls, each with a control id,
a group, and a mask of the groups it will accept a mapping from. Both the list
of reprogrammable buttons and the list of what each may become are read off
the device rather than hardcoded, so a mouse with a different button layout
needs no code.

An MX Master 3S reports eight controls, five of them reprogrammable: middle,
back, forward, gesture, wheel mode. Left and right carry no reprogrammable
bit, which is the structural reason nothing set here can leave a mouse unable
to click.

The group masks differ per model, which is why they are read rather than
assumed. On a 3S every reprogrammable button accepts all seven targets; on an
MX Master 3 back and forward accept only left, right, back and forward. So
"remap forward to middle" works on one and is correctly refused on the other,
with no special case for either.

Three things learned on hardware, all of them non-obvious:

- **A remap of zero is ignored.** Writing `remap = 0x0000` to put a button
  back is accepted and does nothing. The reset is to map the control *to
  itself*. A device that has never been touched still reads `0x0000`, so both
  spellings count as default on the way in. `ParseSetting` turns the word
  `default` into the button's own id, so what is asked for and what is written
  are the same number and a reset is not reported as a snapped value.
- **The flags byte is left at zero on write.** For a set, divert and persist
  are each paired with a "change this" bit, and with those clear the device
  leaves both alone — a read-modify-write for free. It also cannot divert a
  button by accident, which would stop the button working entirely, exactly as
  a diverted thumbwheel does.
- **The keys are per-device, not from a fixed list.** `button-<slug>` is the
  only setting whose valid keys and values come from the hardware, so `cmdSet`
  checks both against the read it already did rather than sending them blind.
  Asking for a button the device will not reassign names the ones it will.

It costs about 170ms of the poll: one `getCount`, one `getCidInfo` per control,
and one `getCidReporting` per reprogrammable one — fourteen round trips on this
mouse. That is the price of asking the device instead of assuming, and it is
paid on every read because the CLI is a fresh process each time with nowhere
to cache it.

## A diverted wheel is a dead wheel

Both wheel features can hand their movement to HID++ notifications instead of
ordinary scroll events. Nothing in OmniGear reads those notifications, so
unless another client is listening the wheel stops working entirely.

The hi-res wheel (`0x2121`) therefore never has its divert bit touched: a
read-modify-write carries it over as found. The thumbwheel (`0x2150`) is the
opposite case and gets a control, because the mouse this was written on
arrived *already* diverted and its thumbwheel did nothing at all. Writing
reporting mode 0 brought it straight back. Being able to see that state and
undo it is the whole value of the capability, and it is not diagnosable from
the desk: the wheel simply does nothing and the device reports no error.

The thumbwheel's invert flag is deliberately not exposed. Setting it sticks
(`fn 1` reads back `00 01`) and changes nothing about which direction the
wheel scrolls, so it evidently applies only to the diverted stream. A switch
that visibly does nothing is worse than no switch. Writes still carry the byte
over untouched, in case something else set it on purpose.

## Easy-Switch, and a warning about probing

An MX device pairs with three hosts and `0x1814` moves it between them.
`0x1814` owns the count, the current slot and the switch; `0x1815` adds
per-slot pairing status and the host's stored name.

**A device can have the first without the second.** That was written here as
"a device with one has both", on the evidence of a single mouse, and an MX
Master 3 disproved it: it switches hosts and has no `0x1815` at all. The first
cut reported every slot as unpaired, because "the call failed" and "the slot
is empty" had been allowed to look the same. `Hosts.PairingKnown` separates
them, and the panel offers every slot on such a device rather than claiming
they are all empty. Slots are 0-based on the wire and 1-based
everywhere above the driver, matching the buttons on the underside.

The warning is about `0x1815`. Its functions are not symmetrical the way most
are: fn 3 reads a host's friendly name and **fn 4 writes it**, taking whatever
bytes it is given. Calling fn 4 with no name bytes, as one would to see what a
read returns, stores an empty name and destroys what was there. It was found
that way, on a real mouse, and the names had to be written back:

```bash
omnigear call mx-master-3s 0x1815 4 00 00 6d 65 67 61 74 72 6f 6e  # "megatron"
```

`omnigear call` is a loaded gun by design, but a function index is not a hint
about whether it reads or writes. Probe an unknown one on hardware you can
afford to reconfigure, and read a feature's whole function list before
sweeping it.

## JSON contract

Every command prints exactly one JSON object. Failures are objects too, so the
QML side never has to parse stderr:

```jsonc
{ "ok": true, "schema": 1, "devices": [ … ] }
{ "ok": false, "error": "no connected device matches 'foo'" }
```

`schema` is bumped when the shape changes in a way that would break a reader.

## Why Go

The first cut of this was Rust, ported early. Nothing here needs what Rust
buys: a 20-second poll, a few hundred bytes of I/O, no concurrency. What the
project does need is to be readable by whoever maintains it and cheap to
contribute a device to — and the catalog is data, which reads better as plain
Go structs than as `&'static [Capability]`. Cross-compiling the shipped binary
for another architecture is also one environment variable rather than a
toolchain.

The one thing lost is exhaustive matching: adding a `Capability` no longer
makes the compiler point at every place that must handle it. If the capability
list grows much past a handful, that gap is worth covering with a test that
walks the catalog.
