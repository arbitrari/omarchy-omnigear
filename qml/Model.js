// Model.js — turning `omnigear` JSON into things the UI can draw.
//
// Pure functions only: no QML types, no side effects. Everything the CLI says
// is treated as untrusted text until it has been through here.

.pragma library

var UNKNOWN = -1

/// Cap on a single CLI reply, so a runaway process cannot grow the shell.
var MAX_REPLY = 65536

function emptyState() {
  return { ok: false, devices: [], error: "", note: "", unreadable: [] }
}

/// Strip control characters and anything that could be read as markup, then
/// bound the length. Device names come off the wire and land in tooltips.
function safeText(value, fallback, maxLength) {
  if (value === undefined || value === null) return fallback
  return String(value)
    .replace(/[<>&\u0000-\u001f\u007f]/g, "")
    .slice(0, maxLength) || fallback
}

function parse(raw) {
  var state = emptyState()
  if (typeof raw !== "string" || raw === "" || raw.length > MAX_REPLY) {
    state.error = "No reply from omnigear"
    return state
  }

  var data
  try {
    data = JSON.parse(raw)
  } catch (e) {
    state.error = "Unreadable reply from omnigear"
    return state
  }

  if (!data || data.ok !== true) {
    state.error = safeText(data && data.error, "omnigear reported a failure", 256)
    return state
  }

  // `list` replies with an array; `set` replies with the one device it just
  // wrote and read back. Both are devices, so both parse the same way.
  var devices = Array.isArray(data.devices) ? data.devices
              : (data.device ? [data.device] : [])
  state.devices = devices.map(parseDevice)
  state.unreadable = (Array.isArray(data.unreadable) ? data.unreadable : [])
    .map(function (path) { return safeText(path, "", 64) })
  state.note = safeText(data.note, "", 128)
  state.ok = true
  return state
}

/// Branches whose name means "this is the mainline". Naming both spellings
/// rather than one means a repo that renames master to main keeps working
/// without the panel needing to be told.
var MAINLINE = ["master", "main"]

/// What the panel should say about the build, from a `version` reply.
///
/// Off the mainline, that is the checkout: which branch is running, and at
/// which commit, is the thing worth knowing about a work in progress. On the
/// mainline — or on a binary carrying no stamp at all, which is what a release
/// built outside a checkout looks like — the branch would say nothing the
/// version does not, so the version is what shows.
///
/// `label` is empty only when the CLI offered neither, which means the reply
/// was unusable.
function parseBuild(raw) {
  var none = { branch: "", commit: "", version: "", label: "" }
  if (typeof raw !== "string" || raw === "" || raw.length > MAX_REPLY) return none

  var data
  try {
    data = JSON.parse(raw)
  } catch (e) {
    return none
  }
  if (!data || data.ok !== true) return none

  var branch = safeText(data.branch, "", 64)
  var commit = safeText(data.commit, "", 40)
  var version = safeText(data.version, "", 32)

  var build = { branch: branch, commit: commit, version: version, label: "" }
  if (branch !== "" && MAINLINE.indexOf(branch) === -1) {
    // The same separator the device cards use for their own trail of facts.
    build.label = commit ? branch + " \u00b7 " + commit : branch
  } else if (version !== "") {
    build.label = "v" + version
  }
  return build
}

function parseDevice(raw) {
  var d = raw || {}
  var s = d.state || {}
  return {
    id: safeText(d.id, "", 128),
    name: safeText(d.name, "Unknown device", 64),
    brand: safeText(d.brandLabel, "", 32),
    category: safeText(d.category, "", 16),
    support: safeText(d.support, "planned", 16),
    usbLabel: safeText(d.usbLabel, "", 16),
    connected: s.connected !== false,
    presence: safeText(s.presence, "awake", 16),
    connection: parseConnection(d.connection),
    icon: safeText(d.icon, "", 32),
    onboardProfile: safeText(s.onboardProfile, "", 16),
    capabilities: Array.isArray(d.capabilities) ? d.capabilities : [],
    conflicts: (Array.isArray(d.conflicts) ? d.conflicts : []).map(parseConflict),
    battery: parseBattery(s.battery),
    dpi: s.dpi ? {
      current: Number(s.dpi.current) || 0,
      min: Number(s.dpi.min) || 0,
      max: Number(s.dpi.max) || 0,
      step: Number(s.dpi.step) || 0,
      presets: Array.isArray(s.dpi.presets) ? s.dpi.presets : []
    } : null,
    pollingRate: s.pollingRate ? {
      current: Number(s.pollingRate.current) || 0,
      supported: Array.isArray(s.pollingRate.supported) ? s.pollingRate.supported : []
    } : null,
    hiResWheel: s.hiResWheel ? {
      hiRes: s.hiResWheel.hiRes === true,
      inverted: s.hiResWheel.inverted === true
    } : null,
    smartShift: s.smartShift ? {
      mode: safeText(s.smartShift.mode, "", 16),
      threshold: Number(s.smartShift.threshold) || 0,
      max: Number(s.smartShift.max) || 1
    } : null,
    buttons: (Array.isArray(s.buttons) ? s.buttons : []).map(parseButton),
    thumbwheel: s.thumbwheel ? {
      mode: safeText(s.thumbwheel.mode, "", 16)
    } : null,
    hosts: s.hosts ? {
      current: Number(s.hosts.current) || 0,
      pairingKnown: s.hosts.pairingKnown === true,
      slots: (Array.isArray(s.hosts.slots) ? s.hosts.slots : []).map(parseHost)
    } : null,
    hits: s.hits ? {
      left: parseHitsButton(s.hits.left),
      right: parseHitsButton(s.hits.right),
      maxActuation: Number(s.hits.maxActuation) || 0,
      maxRapidTrigger: Number(s.hits.maxRapidTrigger) || 0,
      maxHaptics: Number(s.hits.maxHaptics) || 0,
      step: Number(s.hits.step) || 1
    } : null,
    errors: Array.isArray(s.errors) ? s.errors.map(function (e) {
      return safeText(e, "", 256)
    }) : []
  }
}

function parseHitsButton(raw) {
  var b = raw || {}
  return {
    actuation: Number(b.actuation) || 0,
    rapidTrigger: Number(b.rapidTrigger) || 0,
    haptics: Number(b.haptics) || 0
  }
}

function parseButton(raw) {
  var b = raw || {}
  return {
    slug: safeText(b.slug, "", 32),
    label: safeText(b.label, "Button", 32),
    mappedTo: safeText(b.mappedTo, "", 32),
    isDefault: b.default === true,
    targets: (Array.isArray(b.targets) ? b.targets : []).map(function (t) {
      return {
        slug: safeText(t.slug, "", 32),
        label: safeText(t.label, "Button", 32)
      }
    })
  }
}

function parseConflict(raw) {
  var c = raw || {}
  return {
    pid: Number(c.pid) || 0,
    label: safeText(c.label, "another program", 48),
    known: c.known === true
  }
}

function parseHost(raw) {
  var h = raw || {}
  return {
    slot: Number(h.slot) || 0,
    paired: h.paired === true,
    name: safeText(h.name, "", 32),
    active: h.active === true
  }
}

function parseConnection(raw) {
  if (!raw) return { kind: "", label: "" }
  return {
    kind: safeText(raw.kind, "", 24),
    label: safeText(raw.label, "", 48)
  }
}

function parseBattery(raw) {
  if (!raw) return null
  var percent = typeof raw.percent === "number" ? raw.percent : UNKNOWN
  return {
    percent: percent,
    level: safeText(raw.level, "unknown", 16),
    status: safeText(raw.status, "unknown", 16)
  }
}

function has(device, capability) {
  return !!device && device.capabilities.indexOf(capability) !== -1
}

// --- presentation ----------------------------------------------------------

/// Nerd Font glyphs, matching the icons Omarchy's own plugins use.
function categoryIcon(category) {
  switch (category) {
  case "mouse": return "󰍽"
  case "keyboard": return "󰌌"
  case "headset": return "󰋋"
  default: return "󰄜"
  }
}

/// Not every device measures its charge. Some report which of a handful of
/// steps they are on, and the number attached to a step is a boundary rather
/// than a reading: an MX Master 3 says 20 when it means the third of four
/// levels. The driver leaves the percentage unset for those, and everything
/// here falls back to the word.
///
/// A rough percentage for a step, used only for ranking and for choosing an
/// icon. Never shown — showing it is the fiction being avoided.
function levelRank(level) {
  switch (level) {
  case "critical": return 5
  case "low": return 20
  case "good": return 55
  case "full": return 95
  default: return UNKNOWN
  }
}

/// What to compare by, so "lowest battery speaks for the bar" still works
/// across devices that report differently.
function batteryScore(battery) {
  if (!battery) return UNKNOWN
  if (battery.percent !== UNKNOWN && battery.percent >= 0) return battery.percent
  return levelRank(battery.level)
}

function batteryIcon(battery) {
  var charging = ["󰢜", "󰂆", "󰂇", "󰂈", "󰢝", "󰂉", "󰢞", "󰂊", "󰂋", "󰂅"]
  var idle = ["󰁺", "󰁻", "󰁼", "󰁽", "󰁾", "󰁿", "󰂀", "󰂁", "󰂂", "󰁹"]

  var score = batteryScore(battery)
  if (score === UNKNOWN || score < 0) return "󰁽"
  var status = battery ? battery.status : "unknown"
  var slot = Math.max(0, Math.min(9, Math.floor(score / 10)))
  if (status === "charging") return charging[slot]
  if (status === "full") return "󰂅"
  return idle[slot]
}

/// The charge as a number when the device measures one, as a word when it
/// only counts steps, and "--" when it says nothing at all.
function batteryReading(battery) {
  if (!battery) return "--"
  if (battery.percent !== UNKNOWN && battery.percent >= 0)
    return String(battery.percent) + "%"
  return titleCase(battery.level) || "--"
}

/// The device the bar should speak for.
///
/// A chosen device wins, as long as it is connected — a preference for a mouse
/// that is switched off should not leave the bar blank, so it falls back to
/// the default: the lowest battery among the devices that are answering, so
/// the thing about to die is the thing you see.
function primary(devices, preferredId) {
  if (!devices || devices.length === 0) return null
  var live = devices.filter(function (d) {
    return d.connected
  })
  if (live.length === 0) return null
  devices = live

  if (preferredId) {
    for (var i = 0; i < devices.length; i++) {
      if (devices[i].id === preferredId) return devices[i]
    }
  }

  var withBattery = devices.filter(function (d) {
    return batteryScore(d.battery) !== UNKNOWN
  })
  if (withBattery.length === 0) return devices[0]
  return withBattery.reduce(function (lowest, d) {
    return batteryScore(d.battery) < batteryScore(lowest.battery) ? d : lowest
  })
}

/// Whether a device is taking on charge right now.
function isCharging(device) {
  return !!device && !!device.battery && device.battery.status === "charging"
}

/// A bolt tucked against the device icon, for a device taking on charge.
var CHARGING_BOLT = "\uF0E7"

/// The charge reading for the bar, or "" when it should not be shown.
function barCharge(device, showPercentage) {
  if (!device || !device.battery || !showPercentage) return ""
  return batteryReading(device.battery)
}

/// How many icon slots the bar label needs, so the widget reserves the right
/// width for the charge reading and the device icon beside it.
function barSlots(device, showPercentage) {
  // The OmniGear mark stands in until the first read lands, and a wordmark is
  // wider than the square-ish glyph a slot is cut for. It fits a single slot
  // at the default bar font; the margin is for a theme that sets a larger one.
  if (!device) return 1.2

  var slots = 1.0
  if (showPercentage && device && device.battery) {
    // Sized from the reading itself rather than assumed to be three
    // characters: a device that reports steps instead of a percentage shows
    // a word, and "Critical" is not the width of "91%".
    slots += 0.4 * barCharge(device, showPercentage).length
  }
  if (isCharging(device)) slots += 0.5
  return slots
}

function tooltip(state) {
  if (!state.ok) return "OmniGear: " + (state.error.split("\n")[0] || "unavailable")
  if (state.devices.length === 0) return "OmniGear: no supported devices"

  return state.devices.map(function (d) {
    if (!d.connected) return d.name + " · off"
    var parts = [d.name]
    if (batteryScore(d.battery) !== UNKNOWN) {
      var charge = batteryReading(d.battery)
      var state = batteryStatusLabel(d.battery)
      parts.push(state ? charge + " " + state : charge)
    }
    if (d.dpi && d.dpi.current > 0) parts.push(d.dpi.current + " DPI")
    if (d.pollingRate && d.pollingRate.current > 0) parts.push(d.pollingRate.current + " Hz")
    return parts.join(" · ")
  }).join("\n")
}

// --- grouping --------------------------------------------------------------

/// Categories in the order the README lists them, so the panel reads the same
/// way the support table does.
var CATEGORY_ORDER = ["mouse", "keyboard", "headset"]

/// Section headings are main titles, so they are set in caps. Everything
/// below one — capability names, group headings, status words — is a subtitle
/// and is set in Title Case.
function categoryLabel(category) {
  switch (category) {
  case "mouse": return "MICE"
  case "keyboard": return "KEYBOARDS"
  case "headset": return "HEADSETS"
  default: return "OTHER"
  }
}

/// Group devices into [{ category, label, devices }], skipping empty
/// categories. One section per "###" heading in the README.
function groups(devices) {
  var out = []
  var all = devices || []

  // By what kind of thing it is, supported or not. An uncatalogued mouse asks
  // the device for its type, so it lands under MICE with everything else
  // rather than in a ghetto of its own; the card says what it is.
  for (var i = 0; i < CATEGORY_ORDER.length; i++) {
    var category = CATEGORY_ORDER[i]
    var members = all.filter(function (d) {
      return d.category === category
    })
    if (members.length > 0) {
      out.push({ category: category, label: categoryLabel(category), devices: members })
    }
  }

  // Anything with an unrecognised category still gets shown rather than
  // silently dropped — a device the UI cannot name is still a device. This is
  // where a discovered device lands when it will not say what kind it is.
  var known = CATEGORY_ORDER.join(",")
  var rest = all.filter(function (d) {
    return known.indexOf(d.category) === -1
  })
  if (rest.length > 0) {
    out.push({ category: "other", label: categoryLabel("other"), devices: rest })
  }
  return out
}

/// How a device's controls are grouped into tabs. A group appears only if the
/// device has something in it, so a keyboard that only reports battery never
/// grows a tab strip it does not need.
///
/// Battery is deliberately absent: it lives in the card header, visible
/// whichever tab is open.
var TAB_GROUPS = [
  { id: "sensor", label: "Sensor", capabilities: ["dpi", "polling-rate", "onboard-profile"] },
  { id: "wheel", label: "Wheel", capabilities: ["smart-shift", "hi-res-wheel", "thumbwheel"] },
  { id: "triggers", label: "Triggers", capabilities: ["hits"] },
  { id: "buttons", label: "Buttons", capabilities: ["buttons"] },
  { id: "hosts", label: "Hosts", capabilities: ["host"] }
]

/// The devices of one kind. Each kind gets its own primary, because "the
/// mouse" and "the keyboard" are separate questions.
function devicesOfCategory(devices, category) {
  return (devices || []).filter(function (device) {
    return device.category === category
  })
}

function deviceTabs(device) {
  if (!device) return []
  return TAB_GROUPS.filter(function (group) {
    return group.capabilities.some(function (capability) {
      return device.capabilities.indexOf(capability) !== -1
    })
  }).map(function (group) {
    return { id: group.id, label: group.label }
  })
}

function capabilityLabel(capability) {
  switch (capability) {
  case "battery": return "Battery"
  case "dpi": return "DPI"
  case "polling-rate": return "Polling Rate"
  case "hits": return "Haptic Triggers"
  case "smart-shift": return "Smart Shift"
  case "hi-res-wheel": return "Scrolling"
  case "lod": return "Lift-Off Distance"
  case "onboard-profile": return "Profile Storage"
  case "host": return "Easy-Switch"
  case "thumbwheel": return "Thumbwheel"
  case "buttons": return "Buttons"
  default: return capability
  }
}

/// What to call a host slot. The stored name is whatever the machine called
/// itself when it paired, and a slot can be paired with no name at all, so
/// the number is always there to fall back on.
function hostLabel(host, pairingKnown) {
  if (!host) return ""
  // A device that cannot describe its slots says nothing about them, so the
  // slot is named by its number rather than called empty.
  if (pairingKnown === false) return "Slot " + host.slot
  if (!host.paired) return "Slot " + host.slot + " · empty"
  return host.name !== "" ? host.name : "Slot " + host.slot
}

/// Capabilities the device claims but this build cannot show a control for.
/// Listing them is deliberate: "declared, not yet driven" is information, and
/// hiding it would make the mouse look less capable than it is.
function unsupportedCapabilities(device) {
  if (!device) return []
  var handled = ["battery", "dpi", "polling-rate", "onboard-profile", "hits",
                 "smart-shift", "hi-res-wheel", "host", "thumbwheel", "buttons"]
  return device.capabilities.filter(function (c) {
    return handled.indexOf(c) === -1
  })
}

/// Which side owns the device's settings. "Onboard" means the device runs its
/// own stored profile and refuses software writes.
/// A threshold past the end of the scale means the wheel never breaks into a
/// free spin, which reads better as a word than as 255.
var THRESHOLD_NEVER = 255

function thresholdLabel(threshold) {
  return threshold >= THRESHOLD_NEVER ? "Never" : String(threshold)
}

function wheelModeLabel(mode) {
  switch (mode) {
  case "ratchet": return "Ratchet"
  case "freespin": return "Free Spin"
  default: return ""
  }
}

/// The thumbwheel either scrolls or it has been handed to other software, in
/// which case it does nothing unless that software is listening.
function thumbwheelModeLabel(mode) {
  switch (mode) {
  case "scroll": return "Scrolling"
  case "diverted": return "Diverted"
  default: return ""
  }
}

function profileModeLabel(mode) {
  switch (mode) {
  case "onboard": return "Onboard"
  case "host": return "Software"
  case "": return ""
  default: return titleCase(mode)
  }
}

/// The word beside the percentage, and only when it adds something.
///
/// A discharging device gets no word at all. Where the device measures a
/// percentage, its own coarse level is not worth printing beside it: an MX
/// Master 3S reports "full" alongside 65%, so the two contradict each other
/// on screen, and the percentage is the honest one. Where there is no
/// percentage the level *is* the reading, and batteryReading shows it.
function batteryStatusLabel(battery) {
  if (!battery) return ""
  switch (battery.status) {
  case "charging": return "Charging"
  // Charge complete while still on power — worth saying, because the
  // percentage alone does not distinguish it from simply being near full.
  case "full": return "Charged"
  default: return ""
  }
}

function titleCase(text) {
  if (!text) return ""
  return text.charAt(0).toUpperCase() + text.slice(1)
}

/// The line under a device's name: who made it, how it is attached, and how
/// complete support for it is. Empty parts are dropped rather than leaving
/// stray separators.
function deviceSubtitle(device) {
  if (!device) return ""
  var parts = [device.brand]
  if (!device.connected) {
    // Nothing else is known while it is away, and the connection it would use
    // is not the same as the one it has. Which kind of away it is, though, is
    // worth saying: "Off" is a thing the user did, "Disconnected" is a thing
    // that happened to them.
    return parts.concat([presenceLabel(device.presence) || "Disconnected"]).join(" · ")
  }
  if (device.connection && device.connection.label) parts.push(device.connection.label)
  var presence = presenceLabel(device.presence)
  if (presence) parts.push(presence)
  var support = supportLabel(device.support)
  if (support) parts.push(support)
  return parts.filter(function (p) { return !!p }).join(" · ")
}

/// How present the device is, in a word. Empty for the ordinary case, which
/// needs no word at all.
///
/// "Asleep" is retrospective and deliberately so: a sleeping device cannot be
/// observed while sleeping, because asking it anything wakes it. It means the
/// device was asleep when the poll reached it, and will doze off again — which
/// is worth showing, because it is also why that poll was slow.
function presenceLabel(presence) {
  switch (presence) {
  // Nerd Font nf-md-sleep (U+F04B2), the "zzz" mark. Checked against the
  // font rather than guessed: the neighbouring code points in this block are
  // bluetooth-off and a crescent moon, and the first attempt here shipped
  // the bluetooth one.
  case "asleep": return "\uDB81\uDCB2 Asleep"
  case "off": return "Off"
  case "unreachable": return "Disconnected"
  default: return ""
  }
}


function supportLabel(support) {
  switch (support) {
  case "full": return ""
  case "partial": return "Partial Support"
  case "planned": return "Not Supported Yet"
  // "unsupported" is deliberately absent: the card prints that word itself,
  // in the urgent colour, and a plain-text copy beside it would say it twice.
  default: return ""
  }
}

/// Quick DPI buttons offered for every mouse.
///
/// A device only reports the stages its own DPI button cycles through, and
/// some report none at all — an MX Master 3S gives a bare range, so it would
/// get a slider and nothing else. This is the familiar ladder, offered
/// wherever the device can actually reach the value.
var DPI_QUICK_VALUES = [800, 1200, 2000, 4000, 8000]

/// How many chips the row will hold before it starts wrapping awkwardly.
var DPI_MAX_PRESETS = 6

function dpiPresets(dpi) {
  if (!dpi || dpi.max <= 0) return []

  var inRange = function (value) {
    return value >= dpi.min && value <= dpi.max
  }

  var values = DPI_QUICK_VALUES.filter(inRange)

  // A stage the device cycles through is worth keeping even when it is not on
  // the ladder — it is a value this mouse is known to use.
  var stages = dpi.presets || []
  for (var i = 0; i < stages.length; i++) {
    if (inRange(stages[i]) && values.indexOf(stages[i]) === -1)
      values.push(stages[i])
  }

  values.sort(function (a, b) {
    return a - b
  })
  return values.slice(0, DPI_MAX_PRESETS)
}

/// A slider needs a step it can actually land on. Devices report the finest
/// step of a range that gets coarser as it climbs, which would make a
/// 100–44000 slider crawl, so widen it enough to cross the range in a sane
/// number of moves while still hitting round numbers.
function sliderStep(dpi) {
  if (!dpi) return 1
  var span = Math.max(1, dpi.max - dpi.min)
  var target = span / 200
  var steps = [1, 5, 10, 25, 50, 100, 200, 500, 1000]
  for (var i = 0; i < steps.length; i++) {
    if (steps[i] >= target) return Math.max(steps[i], dpi.step || 1)
  }
  return steps[steps.length - 1]
}


/// Programs holding any device's node, gathered across the whole reply.
///
/// Grouped by process rather than by device, because the usual case is one
/// daemon sitting on every mouse in the machine — three cards each repeating
/// "Solaar has this open" says it worse than one line naming all three.
function conflictSummary(devices) {
  var byPid = {}
  var order = []

  ;(devices || []).forEach(function (device) {
    (device.conflicts || []).forEach(function (conflict) {
      var key = String(conflict.pid)
      if (!byPid[key]) {
        byPid[key] = { pid: conflict.pid, label: conflict.label,
                       known: conflict.known, devices: [] }
        order.push(key)
      }
      if (byPid[key].devices.indexOf(device.name) === -1)
        byPid[key].devices.push(device.name)
    })
  })

  return order.map(function (key) { return byPid[key] })
}

/// One line per contending program: who it is, and what it has hold of.
function conflictLine(conflict) {
  return conflict.label + " (pid " + conflict.pid + ") \u2014 "
    + conflict.devices.join(", ")
}


/// What a button is currently set to do, for the line above its picker.
/// A button doing its own job says so rather than repeating its own name.
function buttonAssignment(button) {
  if (!button) return ""
  return button.isDefault ? "Default" : buttonTargetLabel(button, button.mappedTo)
}

function buttonTargetLabel(button, slug) {
  var targets = (button && button.targets) || []
  for (var i = 0; i < targets.length; i++) {
    if (targets[i].slug === slug) return targets[i].label
  }
  return slug
}


/// A `report` reply: the prefilled issue link, and the text behind it.
///
/// The URL is checked rather than trusted. It is built by our own CLI, but it
/// ends up at Qt.openUrlExternally, and that is not a place to hand anything
/// that has not been looked at.
function parseReport(raw) {
  var none = { url: "", title: "", error: "" }
  if (typeof raw !== "string" || raw === "" || raw.length > MAX_REPLY)
    return { url: "", title: "", error: "No reply from omnigear" }

  var data
  try {
    data = JSON.parse(raw)
  } catch (e) {
    return { url: "", title: "", error: "Unreadable reply from omnigear" }
  }
  if (!data || data.ok !== true)
    return { url: "", title: "", error: safeText(data && data.error, "omnigear reported a failure", 256) }

  var url = String(data.url || "")
  if (url.indexOf("https://github.com/") !== 0)
    return { url: "", title: "", error: "Report link was not a GitHub URL" }

  return { url: url, title: safeText(data.title, "", 128), error: "" }
}


/// Whether this device is one the plugin knows, or one it merely found.
///
/// An unsupported device is driven entirely on the strength of the features
/// it advertises: nobody has confirmed any of it against this model. That is
/// worth saying on the card rather than letting it pass for a tested device.
function isUnsupported(device) {
  return !!device && device.support === "unsupported"
}

/// The line under an unsupported device's name, in place of the usual
/// brand-and-connection subtitle facts it has no catalog entry to supply.
function unsupportedNote(device) {
  if (!device) return ""
  var caps = device.capabilities.length
  if (caps === 0)
    return "Not supported yet. Nothing here could be read from it."
  return "Not supported yet. The " + caps + " control"
    + (caps === 1 ? "" : "s") + " below were detected, not tested."
}


/// The line for the permissions card: how many devices are locked away.
function unreadableSummary(paths) {
  var n = (paths || []).length
  if (n === 0) return ""
  return n === 1
    ? "1 device is connected but cannot be opened."
    : n + " devices are connected but cannot be opened."
}


/// A `battery` reply: charge for each device the kernel can speak for.
function parseBatteries(raw) {
  if (typeof raw !== "string" || raw === "" || raw.length > MAX_REPLY) return []
  var data
  try { data = JSON.parse(raw) } catch (e) { return [] }
  if (!data || data.ok !== true || !Array.isArray(data.batteries)) return []
  return data.batteries.map(function (b) {
    return {
      id: safeText(b.id, "", 128),
      percent: typeof b.percent === "number" ? b.percent : UNKNOWN,
      level: safeText(b.level, "", 16),
      status: safeText(b.status, "unknown", 16)
    }
  })
}

/// Fold a cheap battery reading into a device from the last full read.
///
/// Only what the kernel actually knows is taken. A device on the older
/// battery feature gives a coarse word and no percentage, and overwriting a
/// real figure with "unknown" would make the bar show "--" between full
/// reads — worse than a number that is a few minutes old, because charge
/// moves slowly and the reading it replaces was true.
function mergeBattery(device, reading) {
  if (!device || !reading) return device
  var existing = device.battery || { percent: UNKNOWN, level: "unknown", status: "unknown" }
  var merged = {
    percent: reading.percent >= 0 ? reading.percent : existing.percent,
    level: reading.level !== "" ? reading.level : existing.level,
    status: reading.status
  }
  var next = {}
  for (var key in device) next[key] = device[key]
  next.battery = merged
  return next
}


/// What to show when the CLI has not been built.
///
/// The plugin ships its source, so a clone has the panel and no program
/// behind it. That failure otherwise looks identical to a device that stopped
/// answering, which is the one thing this panel exists to report, so it says
/// which it is and how to fix it.
function missingBinaryState(pluginDir) {
  var state = emptyState()
  state.error = "omnigear has not been built.\n\n"
    + "This plugin ships its source and the binary is compiled from it, so a "
    + "fresh clone has no program to run. Build it with:\n\n"
    + "cd " + pluginDir + "\n"
    + "mise exec -- go build -ldflags=\"-s -w\" -o bin/omnigear ."
  return state
}
