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
"nothing found", not "nothing there".

## Writes are verified, never assumed

A device can accept a write and quietly ignore it. `omnigear set` therefore
reads the device back and compares:

| Outcome                        | Result                                      |
|--------------------------------|---------------------------------------------|
| value matches what was asked   | `ok`                                        |
| value changed, but rounded     | `ok`, with `applied` and a `note`           |
| value did not change           | **error** — "accepted the change but kept …"|

The UI shows what the hardware says, not what it was told.

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
