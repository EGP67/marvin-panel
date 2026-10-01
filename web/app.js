// HEART OF GOLD panel bindings (T10). Polls /snapshot.json once a second and writes
// values into the mockup SVG elements tagged id="b-...". Null always renders "--"
// (never 0). Bands and severities come from the wire (D-050); the only formula here
// is CToF. No browser storage, no dynamic code, no markup injection, no external URLs.
(function () {
  'use strict';

  var POLL_MS = 1000;
  var FAILS_BEFORE_DARK = 3;
  var NET_SCALE_MIB = 40;   // S, D-037: "<n>% OF 40 MiB/s"
  var IO_SCALE_MIB = 200;   // D-021: "SCALE 200 MiB/s"
  var MIB = 1048576;
  var GIB = 1073741824;
  var COLORS = { ok: '#5fd8ef', warn: '#f0b429', danger: '#ff6a3d' };
  var NULL_COLOR = '#6b9dad';
  var THERMAL_OK_TEXT = '#cdeef7';
  var DASH = '--';

  function $(id) { return document.getElementById(id); }

  function isNum(v) { return typeof v === 'number' && isFinite(v); }

  function bandColor(b) { return COLORS[b] || NULL_COLOR; }

  function CToF(c) { return Math.round(c * 9 / 5 + 32); }

  // Sets the element's first text node, leaving child tspans (the "%" suffixes) intact.
  function setText(id, s) {
    var el = $(id);
    if (!el) { return; }
    for (var n = el.firstChild; n; n = n.nextSibling) {
      if (n.nodeType === 3) { n.nodeValue = s; return; }
    }
    el.insertBefore(document.createTextNode(s), el.firstChild);
  }

  function setAttr(id, name, v) {
    var el = $(id);
    if (el) { el.setAttribute(name, String(v)); }
  }

  // Inline style beats the mockup's text{fill} rule; presentation attributes do not.
  function setFill(id, color) {
    var el = $(id);
    if (el) { el.style.fill = color; }
  }

  function setWidth(id, w) { setAttr(id, 'width', isNum(w) ? Math.max(0, w) : 0); }

  function round(v) { return isNum(v) ? String(Math.round(v)) : DASH; }
  function fixed1(v) { return isNum(v) ? v.toFixed(1) : DASH; }
  function fahr(c) { return isNum(c) ? String(CToF(c)) : DASH; }
  function mibps(bps) { return isNum(bps) ? (bps / MIB).toFixed(1) : DASH; }
  function kilo(v) { return isNum(v) ? (v >= 1000 ? (v / 1000).toFixed(1) + 'k' : String(v)) : DASH; }

  function sum(vals) {
    var t = 0;
    for (var i = 0; i < vals.length; i++) {
      if (!isNum(vals[i])) { return null; }
      t += vals[i];
    }
    return t;
  }

  function get(o, k) { return o && o[k] !== undefined ? o[k] : null; }
  function at(a, i) { return Array.isArray(a) && i < a.length ? a[i] : null; }

  // Most recent contiguous run of non-null entries: [start, end] inclusive, or null.
  function lastRun(a) {
    if (!Array.isArray(a)) { return null; }
    var end = a.length - 1;
    while (end >= 0 && !isNum(a[end])) { end--; }
    if (end < 0) { return null; }
    var start = end;
    while (start > 0 && isNum(a[start - 1])) { start--; }
    return [start, end];
  }

  function points(a, run, x0, step, y0, scale) {
    var p = [];
    for (var i = run[0]; i <= run[1]; i++) {
      p.push((x0 + i * step).toFixed(1) + ',' + (y0 - a[i] * scale).toFixed(1));
    }
    return p;
  }

  function renderPhrase(s) {
    var lines = get(get(s, 'phrase'), 'lines') || [];
    setText('b-phrase-1', at(lines, 0) || '');
    setText('b-phrase-2', at(lines, 1) || '');
  }

  function renderCPU(cpu) {
    setText('b-cpu-model', get(cpu, 'model_display') || DASH);
    var total = get(cpu, 'total_pct');
    setText('b-cpu-pct', round(total));
    setFill('b-cpu-pct', bandColor(get(cpu, 'band')));
    // D-051 (2): three digits at 132px reach the axis labels; drop "%" and the axis.
    var wide = isNum(total) && Math.round(total) >= 100;
    setAttr('b-cpu-pct-sign', 'visibility', wide ? 'hidden' : 'visible');
    setAttr('b-cpu-axis', 'visibility', wide ? 'hidden' : 'visible');
    var t = get(cpu, 'temp_c');
    setText('b-cpu-info', fixed1(get(cpu, 'freq_ghz')) + ' GHZ | TEMP: ' + fahr(t) + '°F / ' + round(t) + '°C');

    var hist = get(cpu, 'hist_pct');
    var run = lastRun(hist);
    var graph = ['b-cpu-area', 'b-cpu-line', 'b-cpu-peak', 'b-cpu-peak-text', 'b-cpu-now-halo', 'b-cpu-now'];
    for (var g = 0; g < graph.length; g++) { setAttr(graph[g], 'visibility', run ? 'visible' : 'hidden'); }
    if (run) {
      var step = 630 / 119;
      var pts = points(hist, run, 356, step, 494, 0.96);
      setAttr('b-cpu-line', 'points', pts.join(' '));
      var x0 = (356 + run[0] * step).toFixed(1);
      var x1 = (356 + run[1] * step).toFixed(1);
      setAttr('b-cpu-area', 'points', pts.join(' ') + ' ' + x1 + ',494 ' + x0 + ',494');
      var nowY = (494 - hist[run[1]] * 0.96).toFixed(1);
      ['b-cpu-now-halo', 'b-cpu-now'].forEach(function (id) {
        setAttr(id, 'cx', x1);
        setAttr(id, 'cy', nowY);
      });
      var pk = run[0];
      for (var i = run[0]; i <= run[1]; i++) { if (hist[i] > hist[pk]) { pk = i; } }
      var cx = 356 + pk * step;
      var cy = 494 - hist[pk] * 0.96;
      setAttr('b-cpu-peak', 'cx', cx.toFixed(1));
      setAttr('b-cpu-peak', 'cy', cy.toFixed(1));
      setAttr('b-cpu-peak-text', 'x', (cx + 14).toFixed(1));
      // D-051 (3): below the point when above it would crowd "LAST 120 s".
      setAttr('b-cpu-peak-text', 'y', (cy - 7 < 410 ? cy + 21 : cy - 7).toFixed(1));
      setText('b-cpu-peak-text', 'PEAK ' + Math.round(hist[pk]) + '%');
    }

    var pct = get(cpu, 'per_thread_pct');
    var sev = get(cpu, 'per_thread_sev');
    for (var c = 0; c < 12; c++) {
      var v = at(pct, c);
      var h = isNum(v) ? 11 * Math.round(v / 20) : 0;
      var bottom = c < 6 ? 644 : 732;
      setAttr('b-core-' + c, 'height', h);
      setAttr('b-core-' + c, 'y', bottom - h);
      setFill('b-core-' + c, bandColor(at(sev, c)));
      setText('b-core-label-' + c, 'C' + c + ' ' + (isNum(v) ? Math.round(v) + '%' : DASH));
    }
  }

  function renderGPUs(gpus, line) {
    var g0 = at(gpus, 0);
    var g1 = at(gpus, 1);
    var u = [get(g0, 'util_pct'), get(g1, 'util_pct')];
    var mean = isNum(u[0]) && isNum(u[1]) ? (u[0] + u[1]) / 2 : null;
    setText('b-gpu-pct', round(mean));
    setWidth('b-gpu-bar', isNum(mean) ? 896 * mean / 100 : 0);
    var used = sum([get(g0, 'mem_used_mib'), get(g1, 'mem_used_mib')]);
    var total = sum([get(g0, 'mem_total_mib'), get(g1, 'mem_total_mib')]);
    setText('b-gpu-vram', 'VRAM  ' + (isNum(used) ? (used / 1024).toFixed(1) : DASH) + ' / ' +
      (isNum(total) ? (total / 1024).toFixed(1) : DASH) + ' GiB');
    var pw = sum([get(g0, 'power_w'), get(g1, 'power_w')]);
    var lim = sum([get(g0, 'power_limit_w'), get(g1, 'power_limit_w')]);
    setText('b-gpu-power', 'POWER ' + round(pw) + ' / ' + round(lim) + ' W');
    setText('b-gpu-line', line || '');

    for (var g = 0; g < 2; g++) {
      var gpu = at(gpus, g);
      var p = 'b-gpu' + g;
      setText(p + '-head', 'GPU' + g + ' · ' + (get(gpu, 'display_name') || DASH));
      setText(p + '-pct', round(get(gpu, 'util_pct')) + '%');
      setFill(p + '-pct', bandColor(get(gpu, 'util_sev')));
      var mt = get(gpu, 'mem_total_mib');
      var t = get(gpu, 'temp_c');
      setText(p + '-info', (isNum(mt) ? String(Math.round(mt / 1024)) : DASH) + ' GiB · ' +
        fahr(t) + '°F / ' + round(t) + '°C');
      var hist = get(gpu, 'hist_util_pct');
      var run = lastRun(hist);
      setAttr(p + '-spark', 'points', run ? points(hist, run, g === 0 ? 104 : 568, 408 / 29, 1160, 0.28).join(' ') : '');
    }
  }

  function renderMemory(m) {
    setText('b-mem-pct', round(get(m, 'used_pct')) + '%');
    var used = get(m, 'used_bytes');
    var cache = get(m, 'cache_bytes');
    var total = get(m, 'total_bytes');
    var ok = isNum(total) && total > 0;
    setWidth('b-mem-cache', ok && isNum(used) && isNum(cache) ? 408 * (used + cache) / total : 0);
    setWidth('b-mem-used', ok && isNum(used) ? 408 * used / total : 0);
    setText('b-mem-text', (isNum(used) ? (used / GIB).toFixed(1) : DASH) + ' / ' +
      (isNum(total) ? String(Math.round(total / GIB)) : DASH) + ' GiB · CACHE ' +
      (isNum(cache) ? (cache / GIB).toFixed(1) : DASH));
    var st = get(m, 'swap_total_bytes');
    var su = get(m, 'swap_used_bytes');
    if (!isNum(st) || st === 0 || !isNum(su)) {
      setText('b-mem-swap', 'SWAP ' + DASH);
    } else {
      var sp = Math.round(su / st * 100);
      setText('b-mem-swap', 'SWAP ' + sp + '%' + (sp === 0 ? ' — SPARE, UNLIKE ME' : ''));
    }
  }

  function renderNetwork(net, conn) {
    var n = at(net, 0);
    var errs = sum([get(n, 'rx_err'), get(n, 'tx_err')]);
    setText('b-net-con', 'CON ' + round(get(conn, 'established')) + ' · ERR ' + round(errs));
    [['rx', 'RX'], ['tx', 'TX']].forEach(function (d) {
      var bps = get(n, d[0] + '_bps');
      var frac = isNum(bps) ? bps / (NET_SCALE_MIB * MIB) : null;
      setText('b-net-' + d[0], d[1] + ' ' + mibps(bps) + ' MiB/s');
      setText('b-net-' + d[0] + '-pct', round(isNum(frac) ? frac * 100 : null) + '% OF ' + NET_SCALE_MIB + ' MiB/s');
      setWidth('b-net-' + d[0] + '-bar', isNum(frac) ? 416 * Math.min(1, frac) : 0);
    });
  }

  function renderIO(d) {
    [['read', 'READ'], ['write', 'WRITE']].forEach(function (k) {
      var bps = get(d, k[0] + '_bps');
      setText('b-io-' + k[0], k[1] + ' ' + mibps(bps) + ' MiB/s');
      setWidth('b-io-' + k[0] + '-bar', isNum(bps) ? 408 * Math.min(1, bps / (IO_SCALE_MIB * MIB)) : 0);
    });
    setText('b-io-iops', 'IOPS ' + kilo(get(d, 'read_iops')) + '/' + kilo(get(d, 'write_iops')) +
      ' · QUEUE ' + fixed1(get(d, 'queue_avg')));
  }

  function renderSpace(storage, smart, cpu) {
    var st = { ok: 'OK', failing: 'FAIL' }[get(smart, 'state')] || DASH;
    setText('b-space-head', 'IOWAIT ' + fixed1(get(cpu, 'iowait_pct')) + '% · SMART ' + st);
    for (var i = 0; i < 3; i++) {
      var m = at(storage, i);
      var up = get(m, 'used_pct');
      setWidth('b-space-' + i + '-bar', isNum(up) ? 180 * up / 100 : 0);
      setFill('b-space-' + i + '-bar', bandColor(get(m, 'state')));
      setText('b-space-' + i + '-pct', round(up) + '%');
    }
  }

  function renderThermal(name, c, band) {
    var p = 'b-th-' + name;
    setText(p + '-f', fahr(c) + '°F');
    setFill(p + '-f', band === 'ok' ? THERMAL_OK_TEXT : bandColor(band));
    setText(p + '-c', round(c) + '°C');
    setWidth(p + '-bar', isNum(c) ? 206 * Math.min(1, c / 100) : 0);
    setFill(p + '-bar', bandColor(band));
  }

  function renderThermals(s) {
    var cpu = get(s, 'cpu');
    var gpus = get(s, 'gpus');
    var temps = get(s, 'temps');
    renderThermal('cpu', get(cpu, 'temp_c'), get(cpu, 'thermal_band'));
    renderThermal('gpu0', get(at(gpus, 0), 'temp_c'), get(at(gpus, 0), 'thermal_band'));
    renderThermal('gpu1', get(at(gpus, 1), 'temp_c'), get(at(gpus, 1), 'thermal_band'));
    renderThermal('nvme', get(temps, 'nvme_c'), get(temps, 'nvme_thermal_band'));

    var fans = get(s, 'fans');
    for (var b = 0; b < 2; b++) {
      var f = at(fans, b);
      var rpm = get(f, 'rpm');
      var max = get(f, 'max_rpm');
      var p = 'b-fan-' + b;
      if (isNum(rpm)) {
        setText(p + '-label', 'FAN BANK ' + (b + 1));
        setWidth(p + '-bar', isNum(max) && max > 0 ? 250 * Math.min(1, rpm / max) : 0);
        setText(p + '-rpm', rpm + ' RPM');
      } else {
        setText(p + '-label', 'FAN BANK ' + (b + 1) + ': NO TELEMETRY');
        setWidth(p + '-bar', 0);
        setText(p + '-rpm', DASH);
      }
    }
  }

  function pad2(n) { return (n < 10 ? '0' : '') + n; }

  function renderFooter(s) {
    var up = get(get(s, 'host'), 'uptime_seconds');
    var u = DASH;
    if (isNum(up)) {
      var d = Math.floor(up / 86400);
      var r = up % 86400;
      u = d + 'd ' + pad2(Math.floor(r / 3600)) + ':' + pad2(Math.floor(r % 3600 / 60)) + ':' + pad2(r % 60);
    }
    var pc = get(s, 'panic_count');
    var p = isNum(pc) ? (pc > 99 ? '99+' : String(pc)) : DASH;
    setText('b-footer', 'UPTIME ' + u + ' · PANIC COUNT ' + p);
  }

  function render(s) {
    renderPhrase(s);
    renderCPU(get(s, 'cpu'));
    renderGPUs(get(s, 'gpus'), get(s, 'gpu_line'));
    renderMemory(get(s, 'memory'));
    renderNetwork(get(s, 'network'), get(s, 'connections'));
    renderIO(get(s, 'disk_io'));
    renderSpace(get(s, 'storage'), get(s, 'smart'), get(s, 'cpu'));
    renderThermals(s);
    renderFooter(s);
  }

  // After FAILS_BEFORE_DARK misses every value goes "--" and Marvin admits it (MARVIN.md rule 7 note).
  function renderDark() {
    render({});
    setText('b-phrase-1', 'THE SHIP IS NOT ANSWERING. I KNOW HOW IT FEELS.');
    setText('b-phrase-2', '');
  }

  var fails = 0;

  function tick() {
    fetch('/snapshot.json', { cache: 'no-store' })
      .then(function (r) {
        if (!r.ok) { throw new Error('HTTP ' + r.status); }
        return r.json();
      })
      .then(function (s) {
        fails = 0;
        render(s);
      })
      .catch(function () {
        fails++;
        if (fails >= FAILS_BEFORE_DARK) { renderDark(); }
      })
      .then(function () { setTimeout(tick, POLL_MS); });
  }

  tick();
}());
