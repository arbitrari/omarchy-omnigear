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

An MX device pairs with three hosts and `0x1814` moves it between them. Two
features describe the same thing and a device with one has both: `0x1814` owns
the count, the current slot and the switch, and `0x1815` adds per-slot pairing
status and the host's stored name. Slots are 0-based on the wire and 1-based
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
