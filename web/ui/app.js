// backmon web UI: a summary built from /metrics and a browser for the HAL documents of /api.
// All URLs are relative to the <base> element, which contains the path prefix of a reverse proxy.
"use strict";

(function () {
  const REFRESH_INTERVAL_MS = 30000;
  const API = "api";

  const main = document.getElementById("main");
  const updated = document.getElementById("updated");
  // path of the UI including a reverse proxy's prefix, e.g. "/" or "/backmon/"
  const basePath = new URL(document.baseURI).pathname;

  let refreshTimer = null;
  // increases with each navigation, so that a late response of a previous view is dropped
  let generation = 0;

  // ---------------------------------------------------------------------------------------------------------------
  // DOM helpers; text is always inserted as text node, never as HTML

  function h(tag, attrs, ...children) {
    const el = document.createElement(tag);

    for (const [key, value] of Object.entries(attrs || {})) {
      if (value === null || value === undefined || value === false) continue;
      if (key === "class") el.className = value;
      else if (key === "text") el.textContent = value;
      else if (key.startsWith("on")) el.addEventListener(key.slice(2), value);
      else el.setAttribute(key, value === true ? "" : value);
    }

    for (const child of children.flat()) {
      if (child === null || child === undefined || child === false) continue;
      el.append(child instanceof Node ? child : document.createTextNode(String(child)));
    }

    return el;
  }

  function render(...nodes) {
    main.replaceChildren(...nodes);
  }

  function errorNotice(message, retry) {
    return h("div", { class: "notice error", role: "alert" },
      h("div", { text: message }),
      retry ? h("div", { class: "actions" }, h("button", { type: "button", onclick: retry }, "Try again")) : null);
  }

  // ---------------------------------------------------------------------------------------------------------------
  // formatting

  function formatBytes(bytes) {
    if (!Number.isFinite(bytes)) return "–";
    const units = ["B", "KiB", "MiB", "GiB", "TiB", "PiB"];
    let i = 0;
    while (Math.abs(bytes) >= 1024 && i < units.length - 1) {
      bytes /= 1024;
      i++;
    }
    return (i === 0 ? bytes.toFixed(0) : bytes.toFixed(bytes < 10 ? 2 : 1)) + " " + units[i];
  }

  function formatDuration(seconds) {
    seconds = Math.abs(seconds);
    if (seconds < 60) return Math.round(seconds) + " s";
    if (seconds < 3600) return Math.round(seconds / 60) + " min";
    if (seconds < 172800) return Math.round(seconds / 3600) + " h";
    if (seconds < 63072000) return Math.round(seconds / 86400) + " d";
    return Math.round(seconds / 31536000) + " y";
  }

  function plural(count, noun) {
    return count + " " + noun + (count === 1 ? "" : "s");
  }

  function relativeTime(timestamp) {
    if (!timestamp) return "–";
    const diff = timestamp - Date.now() / 1000;
    return diff < 0 ? formatDuration(diff) + " ago" : "in " + formatDuration(diff);
  }

  function absoluteTime(timestamp) {
    return timestamp ? new Date(timestamp * 1000).toLocaleString() : "";
  }

  function timeCell(timestamp) {
    return h("td", { title: absoluteTime(timestamp) }, relativeTime(timestamp),
      timestamp ? h("span", { class: "sub" }, absoluteTime(timestamp)) : null);
  }

  // ---------------------------------------------------------------------------------------------------------------
  // routing: #/ shows the summary, #/api/... the API browser for the resource at that path

  function route() {
    generation++;
    clearInterval(refreshTimer);
    refreshTimer = null;

    const hash = decodeHashPath(location.hash);
    const browsing = hash === API || hash.startsWith(API + "/");

    for (const link of document.querySelectorAll("nav a[data-view]")) {
      link.classList.toggle("active", link.dataset.view === (browsing ? "browser" : "summary"));
    }

    if (browsing) {
      showResource(hash);
    } else {
      showSummary();
      refreshTimer = setInterval(() => {
        if (document.visibilityState === "visible") showSummary(true);
      }, REFRESH_INTERVAL_MS);
    }
  }

  // returns the path of the hash relative to the base, e.g. "api/%2Fbackup" for "#/api/%2Fbackup"
  function decodeHashPath(hash) {
    return hash.replace(/^#\/?/, "");
  }

  // returns the hash of an absolute href of the API, or null if it isn't one
  function hashOf(href) {
    const url = new URL(href, document.baseURI);
    if (url.origin !== location.origin || !url.pathname.startsWith(basePath)) return null;
    return "#/" + url.pathname.slice(basePath.length);
  }

  function apiPath(...segments) {
    return [API, ...segments.map(encodeURIComponent)].join("/");
  }

  // ---------------------------------------------------------------------------------------------------------------
  // summary

  // parses the Prometheus text format into {name: [{labels, value}]}
  function parseMetrics(text) {
    const metrics = {};
    const line = /^([a-zA-Z_:][a-zA-Z0-9_:]*)(?:\{(.*)\})?\s+(\S+)/;
    const label = /([a-zA-Z_][a-zA-Z0-9_]*)="((?:[^"\\]|\\.)*)"/g;

    for (const row of text.split("\n")) {
      if (row === "" || row.startsWith("#")) continue;
      const match = line.exec(row);
      if (!match) continue;

      const labels = {};
      for (const [, key, value] of (match[2] || "").matchAll(label)) {
        labels[key] = value.replace(/\\(.)/g, (_, c) => (c === "n" ? "\n" : c));
      }

      const raw = match[3];
      const value = raw === "+Inf" ? Infinity : raw === "-Inf" ? -Infinity : Number(raw);
      (metrics[match[1]] = metrics[match[1]] || []).push({ labels, value });
    }

    return metrics;
  }

  // builds the disks with their backup groups from the metrics of backmon
  function buildModel(metrics) {
    const disks = new Map();
    const disk = (name) => {
      if (!disks.has(name)) disks.set(name, { name, status: 0, usage: NaN, quota: 0, files: NaN, groups: new Map() });
      return disks.get(name);
    };
    const each = (name, fn) => (metrics[name] || []).forEach(fn);
    const fileKey = (l) => JSON.stringify([l.dir, l.file]);
    const groupKey = (l) => JSON.stringify([l.dir, l.file, l.group]);

    each("backmon_definition_status", (m) => { disk(m.labels.disk).status = m.value; });
    each("backmon_disk_usage_bytes", (m) => { disk(m.labels.disk).usage = m.value; });
    each("backmon_disk_quota_bytes", (m) => { disk(m.labels.disk).quota = m.value; });
    each("backmon_file_count_total", (m) => { disk(m.labels.disk).files = m.value; });

    // values per file definition; they apply to all groups of the file
    const perFile = new Map();
    const fileValues = (l) => {
      const key = l.disk + "\n" + fileKey(l);
      if (!perFile.has(key)) perFile.set(key, {});
      return perFile.get(key);
    };
    each("backmon_backup_file_count_max", (m) => { fileValues(m.labels).max = m.value; });
    each("backmon_backup_latest_file_creation_expected_at_timestamp_seconds", (m) => { fileValues(m.labels).expected = m.value; });

    const group = (l) => {
      const groups = disk(l.disk).groups;
      const key = groupKey(l);
      if (!groups.has(key)) groups.set(key, { dir: l.dir, file: l.file, group: l.group, count: 0 });
      return groups.get(key);
    };
    each("backmon_backup_file_count", (m) => { group(m.labels).count = m.value; });
    each("backmon_backup_latest_file_created_at_timestamp_seconds", (m) => { group(m.labels).created = m.value; });
    each("backmon_backup_latest_size_bytes", (m) => { group(m.labels).size = m.value; });

    for (const d of disks.values()) {
      for (const g of d.groups.values()) {
        Object.assign(g, fileValues({ disk: d.name, dir: g.dir, file: g.file }));
        g.state = groupState(g);
      }
    }

    const sorted = (list, key) => list.sort((a, b) => key(a).localeCompare(key(b)));

    return sorted([...disks.values()], (d) => d.name).map((d) => ({
      ...d,
      groups: sorted([...d.groups.values()], (g) => [g.dir, g.file, g.group].join("\n")),
    }));
  }

  // the state of a backup group, matching the usual alerting rules of backmon
  function groupState(g) {
    if (!g.count) return { level: "bad", label: "No backups" };
    if (g.expected && g.created && g.created < g.expected) {
      return { level: "bad", label: "Overdue", detail: "expected " + relativeTime(g.expected) };
    }
    if (g.max && g.count > g.max) return { level: "warn", label: "Retention exceeded", detail: "keeps at most " + g.max };
    return { level: "ok", label: "OK" };
  }

  function quotaLevel(d) {
    if (!d.quota || !Number.isFinite(d.usage)) return "";
    const ratio = d.usage / d.quota;
    return ratio >= 1 ? "bad" : ratio >= 0.9 ? "warn" : "";
  }

  async function showSummary(silent) {
    const current = generation;
    if (!silent && !main.querySelector(".tiles")) render(h("p", { class: "muted" }, "Loading metrics…"));

    let disks;
    try {
      const response = await fetch("metrics", { headers: { Accept: "text/plain" }, cache: "no-store" });
      if (!response.ok) throw new Error("GET metrics: " + response.status + " " + response.statusText);
      disks = buildModel(parseMetrics(await response.text()));
    } catch (e) {
      if (current === generation) render(errorNotice("Failed to load the metrics: " + e.message, () => showSummary()));
      return;
    }

    if (current !== generation) return;

    updated.textContent = "Updated " + new Date().toLocaleTimeString();
    render(summaryView(disks));
  }

  function summaryView(disks) {
    if (disks.length === 0) {
      return h("div", { class: "notice" }, "No disks are monitored yet. Check the environments in config.yaml and the log of backmon.");
    }

    const groups = disks.flatMap((d) => d.groups);
    const failedDisks = disks.filter((d) => d.status > 0).length;
    const bad = groups.filter((g) => g.state.level === "bad").length;
    const warn = groups.filter((g) => g.state.level === "warn").length;
    const usage = disks.reduce((sum, d) => sum + (Number.isFinite(d.usage) ? d.usage : 0), 0);
    const quota = disks.reduce((sum, d) => sum + d.quota, 0);

    const tiles = h("div", { class: "tiles" },
      tile("Disks", disks.length, failedDisks ? failedDisks + " with errors" : "all checked successfully", failedDisks ? "bad" : ""),
      tile("Backup groups", groups.length, groups.length - bad - warn + " OK"),
      tile("Problems", bad + warn, bad + " failing, " + plural(warn, "warning"), bad ? "bad" : warn ? "warn" : ""),
      tile("Storage used", formatBytes(usage), quota ? "of " + formatBytes(quota) + " quota" : "no quota configured"));

    return h("div", null, tiles, disks.map(diskView));
  }

  function tile(label, value, detail, level) {
    return h("div", { class: "tile " + (level || "") },
      h("div", { class: "label" }, label),
      h("div", { class: "value" }, String(value)),
      h("div", { class: "detail" }, detail));
  }

  function diskView(d) {
    const level = quotaLevel(d);
    const problems = d.groups.filter((g) => g.state.level !== "ok").length;
    const status = d.status > 0
      ? h("span", { class: "pill bad" }, "Check failed")
      : problems
        ? h("span", { class: "pill warn" }, plural(problems, "problem"))
        : h("span", { class: "pill ok" }, "OK");

    const usage = d.quota
      ? h("span", { class: "usage" },
        h("span", { class: "bar " + level, role: "img", "aria-label": Math.round((d.usage / d.quota) * 100) + "% of quota" },
          h("span", { "data-width": Math.min(100, (d.usage / d.quota) * 100) })),
        formatBytes(d.usage) + " of " + formatBytes(d.quota))
      : h("span", null, formatBytes(d.usage) + " used");

    const head = h("div", { class: "disk-head" },
      h("h2", null, d.name),
      status,
      h("span", { class: "grow" }),
      h("span", { class: "disk-meta" },
        usage,
        Number.isFinite(d.files) ? h("span", null, plural(d.files, "file")) : null,
        h("a", { class: "button", href: "#/" + apiPath(d.name) }, "Browse")));

    let body;
    if (d.status > 0 && d.groups.length === 0) {
      body = h("div", { class: "empty" },
        "The backup definitions of this disk are missing or invalid, or its files could not be listed. See the log of backmon for details.");
    } else if (d.groups.length === 0) {
      body = h("div", { class: "empty" }, "No backup files are defined for this disk.");
    } else {
      body = h("div", { class: "table-wrap" }, h("table", null,
        h("thead", null, h("tr", null,
          h("th", null, "Directory / file"),
          h("th", null, "Group"),
          h("th", null, "Latest backup"),
          h("th", { class: "num" }, "Size"),
          h("th", { class: "num" }, "Backups"),
          h("th", null, "Status"),
          h("th", null))),
        h("tbody", null, d.groups.map((g) => groupRow(d, g)))));
    }

    const section = h("section", { class: "disk" }, head, body);
    // widths are set through the CSSOM, as the Content-Security-Policy forbids inline style attributes
    for (const bar of section.querySelectorAll("[data-width]")) bar.style.width = bar.dataset.width + "%";
    return section;
  }

  function groupRow(d, g) {
    return h("tr", null,
      h("td", null, h("span", { class: "mono" }, g.dir + " / " + g.file)),
      h("td", null, h("span", { class: "mono" }, g.group)),
      timeCell(g.created),
      h("td", { class: "num" }, g.size === undefined ? "–" : formatBytes(g.size)),
      h("td", { class: "num" }, g.max ? g.count + " / " + g.max : String(g.count)),
      h("td", null, h("span", { class: "pill " + g.state.level }, g.state.label),
        g.state.detail ? h("span", { class: "sub" }, g.state.detail) : null),
      h("td", { class: "actions" }, h("a", { href: "#/" + apiPath(d.name, g.dir, g.file) }, "Browse")));
  }

  // ---------------------------------------------------------------------------------------------------------------
  // API browser

  async function showResource(path) {
    const current = generation;
    const url = new URL(path, document.baseURI);

    // only resources of this API are browsed, whatever the hash contains
    if (url.origin !== location.origin || !(url.pathname + "/").startsWith(basePath + API + "/")) {
      render(errorNotice("Only resources below " + basePath + API + " can be browsed."));
      return;
    }

    render(crumbs(url.pathname), h("p", { class: "muted" }, "Loading…"));

    let response, body;
    try {
      // a 401 response makes the browser ask for the credentials of the API's Basic Auth
      response = await fetch(url, { headers: { Accept: "application/hal+json, application/json" }, cache: "no-store" });
      body = await response.text();
    } catch (e) {
      if (current === generation) render(crumbs(url.pathname), errorNotice("Request failed: " + e.message, () => showResource(path)));
      return;
    }

    if (current !== generation) return;
    updated.textContent = "Updated " + new Date().toLocaleTimeString();

    if (response.status === 401) {
      render(crumbs(url.pathname), errorNotice(
        "The API requires authentication. Reload to enter the credentials configured in http.basic_auth.",
        () => showResource(path)));
      return;
    }

    if (!response.ok) {
      render(crumbs(url.pathname), errorNotice(response.status + " " + response.statusText + ": " + body.trim()));
      return;
    }

    let resource;
    try {
      resource = JSON.parse(body);
    } catch (e) {
      render(crumbs(url.pathname), errorNotice("The response is no JSON document: " + e.message));
      return;
    }

    render(crumbs(url.pathname), resourceView(resource, url, response));
  }

  // breadcrumbs for each segment of the path below the base, e.g. api › /backup › db
  function crumbs(pathname) {
    const segments = pathname.slice(basePath.length).split("/").filter(Boolean);
    const nav = h("nav", { class: "crumbs", "aria-label": "Path" });

    segments.forEach((segment, i) => {
      if (i > 0) nav.append(h("span", { class: "sep" }, "›"));
      const label = safeDecode(segment);
      nav.append(i === segments.length - 1
        ? h("span", { class: "current" }, label)
        : h("a", { href: "#/" + segments.slice(0, i + 1).join("/") }, label));
    });

    return nav;
  }

  function safeDecode(segment) {
    try {
      return decodeURIComponent(segment);
    } catch (e) {
      return segment;
    }
  }

  function resourceView(resource, url, response) {
    const links = resource._links || {};
    const embedded = resource._embedded || {};
    const props = Object.entries(resource).filter(([key]) => !key.startsWith("_"));
    const title = resource.name || (url.pathname === basePath + API ? "API" : safeDecode(url.pathname.split("/").pop()));

    const up = linkOf(links.up);
    const download = linkOf(links.download);

    return h("article", { class: "resource" },
      h("header", null,
        h("h1", null, title),
        h("span", { class: "request" }, "GET " + url.pathname + " · " + response.status + " · " + (response.headers.get("Content-Type") || "")),
        up && hashOf(up.href) ? h("a", { class: "button", href: hashOf(up.href) }, "↑ Up") : null,
        download ? downloadButton(download.href) : null),

      props.length ? h("section", null, h("h3", null, "Properties"),
        h("dl", { class: "props" }, props.flatMap(([key, value]) => [h("dt", null, key), h("dd", null, propertyValue(key, value))]))) : null,

      Object.keys(links).length ? h("section", null, h("h3", null, "Links"),
        h("ul", { class: "links" }, Object.entries(links).flatMap(([rel, value]) =>
          (Array.isArray(value) ? value : [value]).map((link) => h("li", null, h("span", { class: "rel" }, rel), linkTarget(rel, link)))))) : null,

      Object.entries(embedded).map(([rel, children]) => embeddedView(rel, Array.isArray(children) ? children : [children])),

      h("section", null, h("details", { class: "raw" }, h("summary", null, "Raw response"),
        h("pre", null, JSON.stringify(resource, null, 2)))));
  }

  function linkOf(value) {
    const link = Array.isArray(value) ? value[0] : value;
    return link && typeof link.href === "string" ? link : null;
  }

  function propertyValue(key, value) {
    if (key === "quota" && typeof value === "number") return value ? formatBytes(value) + " (" + value + " bytes)" : "none";
    return typeof value === "string" ? value : JSON.stringify(value);
  }

  function linkTarget(rel, link) {
    if (!link || typeof link.href !== "string") return h("span", { class: "muted" }, "(invalid link)");
    if (rel === "download") return downloadButton(link.href, link.href);

    const hash = hashOf(link.href);
    return hash ? h("a", { class: "href", href: hash }, link.href) : h("span", { class: "href" }, link.href);
  }

  function downloadButton(href, label) {
    // the browser sends the credentials it has cached for the API; the Content-Disposition header names the file
    return h("a", { class: "button primary", href: new URL(href, document.baseURI).pathname, download: true }, "⤓ ", label || "Download");
  }

  function embeddedView(rel, children) {
    const rows = children.map((child) => {
      const links = (child && child._links) || {};
      const self = linkOf(links.self);
      const download = linkOf(links.download);
      const selfHash = self && hashOf(self.href);
      const name = (child && child.name) || (self ? self.href : "(unnamed)");

      return h("tr", null,
        h("td", null, selfHash ? h("a", { class: "mono", href: selfHash }, name) : h("span", { class: "mono" }, name)),
        h("td", { class: "actions" },
          selfHash ? h("a", { class: "button", href: selfHash }, "Open") : null,
          download ? downloadButton(download.href) : null,
          !selfHash && !download && rel === "variants" ? h("span", { class: "muted" }, "downloads disabled") : null));
    });

    return h("section", { class: "flush" },
      h("h3", null, rel + " (" + children.length + ")"),
      children.length
        ? h("div", { class: "table-wrap" }, h("table", null, h("tbody", null, rows)))
        : h("div", { class: "empty muted" }, "none"));
  }

  // ---------------------------------------------------------------------------------------------------------------

  document.getElementById("reload").addEventListener("click", route);
  window.addEventListener("hashchange", route);
  route();
})();
