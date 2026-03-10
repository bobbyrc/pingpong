/* ============================================================
   PingPong — Dashboard & Config Client
   Vanilla JS: SSE updates, sparklines, config form
   ============================================================ */

(function () {
    'use strict';

    // ── Client-side state ───────────────────────────────────────

    var outageStartedAt = null; // Date when UI first saw connection_up == 0

    // ── Formatting Helpers ──────────────────────────────────────

    function formatLatency(ms) {
        if (ms == null || isNaN(ms)) return '--';
        return ms.toFixed(1);
    }

    function formatSpeed(mbps) {
        if (mbps == null || isNaN(mbps)) return '--';
        return mbps.toFixed(1);
    }

    function formatLoss(pct) {
        if (pct == null || isNaN(pct)) return '--';
        return pct.toFixed(1);
    }

    function computeStats(arr) {
        if (!arr || arr.length === 0) return { min: null, avg: null, max: null };
        var min = Infinity, max = -Infinity, sum = 0;
        for (var i = 0; i < arr.length; i++) {
            if (arr[i] < min) min = arr[i];
            if (arr[i] > max) max = arr[i];
            sum += arr[i];
        }
        return { min: min, avg: sum / arr.length, max: max };
    }

    function formatDuration(seconds) {
        if (seconds == null || isNaN(seconds) || seconds < 0) return '--';
        seconds = Math.floor(seconds);
        if (seconds === 0) return '0s';

        var h = Math.floor(seconds / 3600);
        var m = Math.floor((seconds % 3600) / 60);
        var s = seconds % 60;
        var parts = [];
        if (h > 0) parts.push(h + 'h');
        if (m > 0) parts.push(m + 'm');
        if (s > 0 || parts.length === 0) parts.push(s + 's');
        return parts.join(' ');
    }

    function latencyColor(ms) {
        if (ms == null || isNaN(ms)) return '';
        if (ms < 50) return 'text-green';
        if (ms <= 100) return 'text-yellow';
        return 'text-red';
    }

    function lossColor(pct) {
        if (pct == null || isNaN(pct)) return '';
        if (pct <= 0) return 'text-green';
        if (pct < 5) return 'text-yellow';
        return 'text-red';
    }

    // ── Utility ─────────────────────────────────────────────────

    /** Get the first metric entry from a metric family (or null). */
    function first(metrics, name) {
        var arr = metrics[name];
        if (arr && arr.length > 0) return arr[0];
        return null;
    }

    /** Get all entries for a metric family (or empty array). */
    function all(metrics, name) {
        return metrics[name] || [];
    }

    /** Set text content and flash the .updated class briefly. */
    function setText(el, text) {
        if (!el) return;
        var prev = el.textContent;
        el.textContent = text;
        if (prev !== text && prev !== '') {
            el.classList.add('updated');
            setTimeout(function () {
                el.classList.remove('updated');
            }, 300);
        }
    }

    /** Set color class on element (removes previous color classes first). */
    function setColor(el, colorClass) {
        if (!el) return;
        el.classList.remove('text-green', 'text-yellow', 'text-red');
        if (colorClass) el.classList.add(colorClass);
    }

    function removeLoading(el) {
        if (el) el.classList.remove('loading');
    }

    // ── Sparkline Renderer ──────────────────────────────────────

    function drawSparkline(canvas, values, color) {
        if (!canvas || !values || values.length === 0) return;

        var ctx = canvas.getContext('2d');
        var dpr = window.devicePixelRatio || 1;
        var w = canvas.clientWidth;
        var h = canvas.clientHeight;

        // Set canvas resolution for crisp drawing
        canvas.width = w * dpr;
        canvas.height = h * dpr;
        ctx.scale(dpr, dpr);

        ctx.clearRect(0, 0, w, h);

        var len = values.length;
        if (len < 2) {
            if (len === 1) {
                // Draw a single dot so the user sees data immediately
                var dotX = w / 2;
                var dotY = h / 2;
                ctx.beginPath();
                ctx.arc(dotX, dotY, 3, 0, Math.PI * 2);
                ctx.fillStyle = color;
                ctx.fill();
            }
            return;
        }

        var min = Infinity;
        var max = -Infinity;
        for (var i = 0; i < len; i++) {
            if (values[i] < min) min = values[i];
            if (values[i] > max) max = values[i];
        }

        // Add 10% padding on Y axis
        var range = max - min;
        if (range === 0) range = 1;
        var pad = range * 0.1;
        min -= pad;
        max += pad;

        var stepX = w / (len - 1);

        // Build the points
        function toX(idx) { return idx * stepX; }
        function toY(val) { return h - ((val - min) / (max - min)) * h; }

        // Draw gradient fill
        ctx.beginPath();
        ctx.moveTo(toX(0), toY(values[0]));
        for (var j = 1; j < len; j++) {
            ctx.lineTo(toX(j), toY(values[j]));
        }
        ctx.lineTo(toX(len - 1), h);
        ctx.lineTo(0, h);
        ctx.closePath();

        var gradient = ctx.createLinearGradient(0, 0, 0, h);
        gradient.addColorStop(0, color + '33'); // ~20% opacity
        gradient.addColorStop(1, color + '00'); // transparent
        ctx.fillStyle = gradient;
        ctx.fill();

        // Draw line
        ctx.beginPath();
        ctx.moveTo(toX(0), toY(values[0]));
        for (var k = 1; k < len; k++) {
            ctx.lineTo(toX(k), toY(values[k]));
        }
        ctx.strokeStyle = color;
        ctx.lineWidth = 1.5;
        ctx.lineJoin = 'round';
        ctx.lineCap = 'round';
        ctx.stroke();
    }

    // ── Sparkline History (ring buffers) ────────────────────────

    var SPARK_MAX = 60;
    var pingHistory = {};       // keyed by target
    var downloadHistory = [];
    var uploadHistory = [];

    function pushHistory(arr, value) {
        arr.push(value);
        if (arr.length > SPARK_MAX) arr.shift();
        return arr;
    }

    // ── CSS color values for sparklines ─────────────────────────
    var COLOR_GREEN = '#22c55e';

    // ── History Seeding ─────────────────────────────────────────

    function loadHistory() {
        return fetch('/api/history')
            .then(function (res) {
                if (!res.ok) throw new Error('history fetch failed');
                return res.json();
            })
            .then(function (data) {
                // Seed ping latency history
                var pingData = data.ping_latency;
                if (pingData) {
                    for (var target in pingData) {
                        if (!pingData.hasOwnProperty(target)) continue;
                        pingHistory[target] = pingData[target].map(function (p) { return p.v; });
                    }
                }

                // Seed download history
                var dlData = data.download_speed;
                if (dlData && dlData['']) {
                    downloadHistory.length = 0;
                    dlData[''].forEach(function (p) { downloadHistory.push(p.v); });
                }

                // Seed upload history
                var ulData = data.upload_speed;
                if (ulData && ulData['']) {
                    uploadHistory.length = 0;
                    ulData[''].forEach(function (p) { uploadHistory.push(p.v); });
                }
            })
            .catch(function () {
                // Silent fallback — sparklines start empty as before
            });
    }

    // ── Dashboard Update ────────────────────────────────────────

    function updateDashboard(snapshot) {
        var metrics = snapshot.metrics;
        if (!metrics) return;

        updateConnectionStatus(metrics);
        updatePingCards(metrics);
        updateSpeedTest(metrics);
        updateDNS(metrics);
        updateTraceroute(metrics);
    }

    // ── Connection Status ───────────────────────────────────────

    function updateConnectionStatus(metrics) {
        var connEntry = first(metrics, 'pingpong_connection_up');
        var isUp = connEntry ? connEntry.value === 1 : null;

        var dot = document.getElementById('header-status-dot');
        var label = document.getElementById('header-status-label');
        var banner = document.getElementById('connection-banner');
        var bannerIcon = document.getElementById('connection-banner-icon');
        var bannerText = document.getElementById('connection-banner-text');
        var bannerDetail = document.getElementById('connection-banner-detail');

        if (isUp === null) return;

        // Header status
        if (dot) {
            dot.classList.remove('up', 'down');
            dot.classList.add(isUp ? 'up' : 'down');
        }
        if (label) {
            label.textContent = isUp ? 'Connected' : 'Disconnected';
        }

        // Banner
        if (banner) {
            if (isUp) {
                banner.classList.remove('down');
            } else {
                banner.classList.add('down');
            }
        }

        if (bannerIcon) {
            // Unicode bullet for dot indicator
            bannerIcon.innerHTML = '&#9679;';
        }

        if (bannerText) {
            if (isUp) {
                outageStartedAt = null;
                bannerText.textContent = 'Connection Up';
            } else {
                if (!outageStartedAt) {
                    outageStartedAt = new Date();
                }
                var currentOutageSec = (Date.now() - outageStartedAt.getTime()) / 1000;
                bannerText.textContent = 'Down for ' + formatDuration(currentOutageSec);
            }
        }

        if (bannerDetail) {
            var parts = [];
            var downtimeEntry = first(metrics, 'pingpong_downtime_seconds_total');
            if (downtimeEntry && downtimeEntry.value > 0) {
                parts.push('Total downtime: ' + formatDuration(downtimeEntry.value));
            }
            var flapEntry = first(metrics, 'pingpong_connection_flaps_total');
            if (flapEntry) {
                parts.push('Flaps: ' + Math.floor(flapEntry.value));
            }
            bannerDetail.textContent = parts.join(' | ');
        }
    }

    // ── Ping Cards ──────────────────────────────────────────────

    function updatePingCards(metrics) {
        var container = document.getElementById('ping-cards');
        if (!container) return;

        var latencyEntries = all(metrics, 'pingpong_ping_latency_ms');
        if (latencyEntries.length === 0) return;

        // Build lookup maps for auxiliary metrics by target
        var minMap = {};
        var maxMap = {};
        var jitterMap = {};
        var lossMap = {};

        all(metrics, 'pingpong_ping_min_ms').forEach(function (e) {
            if (e.labels && e.labels.target) minMap[e.labels.target] = e.value;
        });
        all(metrics, 'pingpong_ping_max_ms').forEach(function (e) {
            if (e.labels && e.labels.target) maxMap[e.labels.target] = e.value;
        });
        all(metrics, 'pingpong_jitter_ms').forEach(function (e) {
            if (e.labels && e.labels.target) jitterMap[e.labels.target] = e.value;
        });
        all(metrics, 'pingpong_packet_loss_percent').forEach(function (e) {
            if (e.labels && e.labels.target) lossMap[e.labels.target] = e.value;
        });

        // Track which targets are present so we can remove stale cards
        var seenTargets = {};

        latencyEntries.forEach(function (entry) {
            var target = entry.labels ? entry.labels.target : 'unknown';
            seenTargets[target] = true;

            var cardId = 'ping-card-' + target.replace(/[^a-zA-Z0-9]/g, '-');
            var card = document.getElementById(cardId);

            if (!card) {
                var hostname = (entry.labels && entry.labels.hostname) ? entry.labels.hostname : '';
                card = createPingCard(cardId, target, hostname);
                container.appendChild(card);
                // Remove the loading placeholder on first card creation
                var placeholder = document.getElementById('ping-loading');
                if (placeholder) placeholder.remove();
            }

            // Update latency
            var latVal = card.querySelector('.ping-latency-value');
            if (latVal) {
                setText(latVal, formatLatency(entry.value));
                setColor(latVal, latencyColor(entry.value));
            }

            // Update min
            var minVal = card.querySelector('.ping-min-value');
            if (minVal) setText(minVal, formatLatency(minMap[target]));

            // Update max
            var maxVal = card.querySelector('.ping-max-value');
            if (maxVal) setText(maxVal, formatLatency(maxMap[target]));

            // Update jitter
            var jitVal = card.querySelector('.ping-jitter-value');
            if (jitVal) setText(jitVal, formatLatency(jitterMap[target]));

            // Update packet loss
            var lossVal = card.querySelector('.ping-loss-value');
            if (lossVal) {
                setText(lossVal, formatLoss(lossMap[target]));
                setColor(lossVal, lossColor(lossMap[target]));
            }

            // Sparkline history — only push when value actually changes
            if (!pingHistory[target]) pingHistory[target] = [];
            var hist = pingHistory[target];
            if (hist.length === 0 || hist[hist.length - 1] !== entry.value) {
                pushHistory(hist, entry.value);
            }
            var sparkCanvas = card.querySelector('.sparkline');
            if (sparkCanvas) {
                drawSparkline(sparkCanvas, hist, COLOR_GREEN);
            }
        });

        // Remove cards for targets no longer reported
        var existingCards = container.querySelectorAll('.card');
        for (var i = 0; i < existingCards.length; i++) {
            var cId = existingCards[i].id;
            var found = false;
            for (var t in seenTargets) {
                if (cId === 'ping-card-' + t.replace(/[^a-zA-Z0-9]/g, '-')) {
                    found = true;
                    break;
                }
            }
            if (!found) {
                container.removeChild(existingCards[i]);
            }
        }
    }

    function createPingCard(id, target, hostname) {
        var card = document.createElement('div');
        card.className = 'card';
        card.id = id;

        var headerHtml;
        if (hostname && hostname !== target) {
            headerHtml =
                '<div class="card-header">' +
                    '<div class="ping-target-header">' +
                        '<h3 class="card-title ping-hostname">' + escapeHtml(hostname) + '</h3>' +
                        '<span class="ping-target-ip font-mono">' + escapeHtml(target) + '</span>' +
                    '</div>' +
                '</div>';
        } else {
            headerHtml =
                '<div class="card-header">' +
                    '<h3 class="card-title ping-target-name font-mono">' + escapeHtml(target) + '</h3>' +
                '</div>';
        }

        card.innerHTML =
            headerHtml +
            '<div class="card-body">' +
                '<div class="metric" style="margin-bottom:12px">' +
                    '<span class="metric-value ping-latency-value">--</span>' +
                    '<span class="metric-label">Latency <span class="metric-unit">ms</span></span>' +
                '</div>' +
                '<canvas class="sparkline" width="200" height="60"></canvas>' +
                '<div class="ping-metrics" style="margin-top:12px">' +
                    '<div class="metric metric-small">' +
                        '<span class="metric-value-sm ping-min-value">--</span>' +
                        '<span class="metric-label">Min <span class="metric-unit">ms</span></span>' +
                    '</div>' +
                    '<div class="metric metric-small">' +
                        '<span class="metric-value-sm ping-max-value">--</span>' +
                        '<span class="metric-label">Max <span class="metric-unit">ms</span></span>' +
                    '</div>' +
                    '<div class="metric metric-small">' +
                        '<span class="metric-value-sm ping-jitter-value">--</span>' +
                        '<span class="metric-label">Jitter <span class="metric-unit">ms</span></span>' +
                    '</div>' +
                    '<div class="metric metric-small">' +
                        '<span class="metric-value-sm ping-loss-value">--</span>' +
                        '<span class="metric-label">Loss <span class="metric-unit">%</span></span>' +
                    '</div>' +
                '</div>' +
            '</div>';
        return card;
    }

    function escapeHtml(str) {
        var div = document.createElement('div');
        div.appendChild(document.createTextNode(str));
        return div.innerHTML;
    }

    // ── Speed Test ──────────────────────────────────────────────

    function updateSpeedTest(metrics) {
        var dlEntry = first(metrics, 'pingpong_download_speed_mbps');
        var ulEntry = first(metrics, 'pingpong_upload_speed_mbps');
        var latEntry = first(metrics, 'pingpong_speedtest_latency_ms');
        var infoEntry = first(metrics, 'pingpong_speedtest_info');

        // Download
        var dlEl = document.getElementById('speedtest-download');
        if (dlEl && dlEntry) {
            setText(dlEl, formatSpeed(dlEntry.value));
            removeLoading(dlEl);
            if (downloadHistory.length === 0 || downloadHistory[downloadHistory.length - 1] !== dlEntry.value) {
                pushHistory(downloadHistory, dlEntry.value);
            }
            var dlStats = computeStats(downloadHistory);
            setText(document.getElementById('dl-min'), formatSpeed(dlStats.min));
            setText(document.getElementById('dl-avg'), formatSpeed(dlStats.avg));
            setText(document.getElementById('dl-max'), formatSpeed(dlStats.max));
        }

        // Upload
        var ulEl = document.getElementById('speedtest-upload');
        if (ulEl && ulEntry) {
            setText(ulEl, formatSpeed(ulEntry.value));
            removeLoading(ulEl);
            if (uploadHistory.length === 0 || uploadHistory[uploadHistory.length - 1] !== ulEntry.value) {
                pushHistory(uploadHistory, ulEntry.value);
            }
            var ulStats = computeStats(uploadHistory);
            setText(document.getElementById('ul-min'), formatSpeed(ulStats.min));
            setText(document.getElementById('ul-avg'), formatSpeed(ulStats.avg));
            setText(document.getElementById('ul-max'), formatSpeed(ulStats.max));
        }

        // Latency
        var latEl = document.getElementById('speedtest-latency');
        if (latEl && latEntry) {
            setText(latEl, formatLatency(latEntry.value));
            removeLoading(latEl);
        }

        // Jitter
        var jitEl = document.getElementById('speedtest-jitter');
        if (jitEl) {
            var jitEntry = first(metrics, 'pingpong_speedtest_jitter_ms');
            setText(jitEl, jitEntry ? formatLatency(jitEntry.value) : '--');
            if (jitEntry) removeLoading(jitEl);
        }

        // Server info
        var serverNameEl = document.getElementById('speedtest-server-name');
        if (serverNameEl && infoEntry && infoEntry.labels) {
            var name = infoEntry.labels.server_name || '';
            var location = infoEntry.labels.server_location || '';
            var display = name;
            if (location) display += ' (' + location + ')';
            setText(serverNameEl, display || '--');
        }
    }

    // ── DNS ─────────────────────────────────────────────────────

    function updateDNS(metrics) {
        var tbody = document.getElementById('dns-table-body');
        if (!tbody) return;

        var entries = all(metrics, 'pingpong_dns_resolution_ms');
        if (entries.length === 0) return;

        // Sort by target then server
        entries.sort(function (a, b) {
            var at = (a.labels && a.labels.target) || '';
            var bt = (b.labels && b.labels.target) || '';
            if (at !== bt) return at.localeCompare(bt);
            var as = (a.labels && a.labels.server) || '';
            var bs = (b.labels && b.labels.server) || '';
            return as.localeCompare(bs);
        });

        var html = '';
        entries.forEach(function (entry) {
            var target = (entry.labels && entry.labels.target) || '--';
            var server = (entry.labels && entry.labels.server) || 'system';
            var ms = entry.value;
            var color = latencyColor(ms);
            html +=
                '<tr>' +
                    '<td class="cell-nowrap">' + escapeHtml(target) + '</td>' +
                    '<td class="cell-nowrap">' + escapeHtml(server) + '</td>' +
                    '<td class="cell-nowrap ' + color + '">' + formatLatency(ms) + ' ms</td>' +
                '</tr>';
        });
        tbody.innerHTML = html;

        // Failures total
        var failEl = document.getElementById('dns-failures');
        if (failEl) {
            var failEntries = all(metrics, 'pingpong_dns_failures_total');
            var total = 0;
            failEntries.forEach(function (e) { total += e.value; });
            setText(failEl, String(Math.floor(total)));
        }
    }

    // ── Traceroute ──────────────────────────────────────────────

    function updateTraceroute(metrics) {
        var tbody = document.getElementById('traceroute-table-body');
        if (!tbody) return;

        var hopEntries = all(metrics, 'pingpong_traceroute_hop_latency_ms');

        // If no hop data yet, leave table empty
        if (hopEntries.length === 0) return;

        // Sort by hop number (numeric)
        hopEntries.sort(function (a, b) {
            var ah = parseInt((a.labels && a.labels.hop) || '0', 10);
            var bh = parseInt((b.labels && b.labels.hop) || '0', 10);
            return ah - bh;
        });

        // Find max latency for bar scaling
        var maxLatency = 0;
        hopEntries.forEach(function (e) {
            if (e.value > maxLatency) maxLatency = e.value;
        });
        if (maxLatency === 0) maxLatency = 1;

        // Determine traceroute target for display
        var targetEl = document.getElementById('traceroute-target');
        if (targetEl && hopEntries.length > 0 && hopEntries[0].labels) {
            var trTarget = hopEntries[0].labels.target || '';
            if (trTarget) setText(targetEl, trTarget);
        }

        var html = '';
        hopEntries.forEach(function (entry) {
            var hop = (entry.labels && entry.labels.hop) || '--';
            var address = (entry.labels && entry.labels.address) || '*';
            var ms = entry.value;
            var color = latencyColor(ms);
            var barPct = Math.max(2, (ms / maxLatency) * 100);
            var barColor = ms < 50 ? COLOR_GREEN : (ms <= 100 ? '#eab308' : '#ef4444');

            html +=
                '<tr>' +
                    '<td class="cell-center">' + escapeHtml(String(hop)) + '</td>' +
                    '<td class="cell-nowrap">' + escapeHtml(address) + '</td>' +
                    '<td class="cell-nowrap ' + color + '" style="text-align:right">' + formatLatency(ms) + ' ms</td>' +
                    '<td><div class="hop-bar" style="width:' + barPct.toFixed(1) + '%;background:' + barColor + '"></div></td>' +
                '</tr>';
        });
        tbody.innerHTML = html;

        // Total hops
        var hopsEl = document.getElementById('traceroute-hops');
        if (hopsEl) {
            var hopsEntry = first(metrics, 'pingpong_traceroute_hops');
            if (hopsEntry) setText(hopsEl, String(Math.floor(hopsEntry.value)));
        }

        // Failures
        var failEl = document.getElementById('traceroute-failures');
        if (failEl) {
            var failEntry = first(metrics, 'pingpong_traceroute_failures_total');
            if (failEntry) setText(failEl, String(Math.floor(failEntry.value)));
        }
    }

    // ── SSE Connection (all pages) ───────────────────────────

    var isDashboard = !!document.getElementById('ping-cards');

    function connectSSE() {
        var source = new EventSource('/api/events');

        source.onmessage = function (e) {
            try {
                var data = JSON.parse(e.data);
                if (isDashboard) {
                    updateDashboard(data);
                } else if (data.metrics) {
                    updateConnectionStatus(data.metrics);
                }
            } catch (err) {
                // Silently ignore malformed JSON
            }
        };

        source.onerror = function () {
            var dot = document.getElementById('header-status-dot');
            var label = document.getElementById('header-status-label');
            if (dot) {
                dot.classList.remove('up', 'down');
                dot.classList.add('down');
            }
            if (label) {
                label.textContent = 'SSE Disconnected';
            }
        };

        source.onopen = function () {
            var dot = document.getElementById('header-status-dot');
            var label = document.getElementById('header-status-label');
            if (dot) {
                dot.classList.remove('down');
            }
            if (label && label.textContent === 'SSE Disconnected') {
                label.textContent = 'Reconnecting\u2026';
            }
        };
    }

    if (isDashboard) {
        loadHistory().then(connectSSE).catch(connectSSE);
    } else {
        connectSSE();
    }

    // ── Config Page ─────────────────────────────────────────────

    function loadConfig() {
        fetch('/api/config')
            .then(function (res) {
                if (!res.ok) throw new Error('Failed to load config');
                return res.json();
            })
            .then(function (data) {
                // data is expected to be an object with PINGPONG_* keys
                for (var key in data) {
                    if (!data.hasOwnProperty(key)) continue;
                    var input = document.getElementById(key);
                    if (input) {
                        input.value = data[key];
                    }
                }
            })
            .catch(function (err) {
                showConfigAlert('error', 'Failed to load configuration: ' + err.message);
            });
    }

    function setupConfigForm() {
        var form = document.getElementById('config-form');
        if (!form) return;

        form.addEventListener('submit', function (e) {
            e.preventDefault();

            var saveBtn = document.getElementById('config-save');
            if (saveBtn) {
                saveBtn.disabled = true;
                saveBtn.textContent = 'Saving\u2026';
            }

            var inputs = form.querySelectorAll('input[name]');
            var payload = {};
            for (var i = 0; i < inputs.length; i++) {
                var input = inputs[i];
                if (input.name) {
                    payload[input.name] = input.value;
                }
            }

            fetch('/api/config', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(payload)
            })
            .then(function (res) {
                if (!res.ok) {
                    return res.text().then(function (t) {
                        throw new Error(t || 'Save failed');
                    });
                }
                showConfigAlert('success', 'Configuration saved. Restart the container for changes to take effect.');
            })
            .catch(function (err) {
                showConfigAlert('error', 'Failed to save: ' + err.message);
            })
            .finally(function () {
                if (saveBtn) {
                    saveBtn.disabled = false;
                    saveBtn.textContent = 'Save Configuration';
                }
            });
        });
    }

    function showConfigAlert(type, message) {
        var alert = document.getElementById('config-alert');
        var icon = document.getElementById('config-alert-icon');
        var text = document.getElementById('config-alert-text');
        if (!alert) return;

        alert.classList.remove('alert-hidden', 'alert-success', 'alert-error');
        alert.classList.add(type === 'success' ? 'alert-success' : 'alert-error');

        if (icon) {
            icon.innerHTML = type === 'success' ? '&#10003;' : '&#10007;';
        }
        if (text) {
            text.textContent = message;
        }

        // Auto-hide success after 5 seconds
        if (type === 'success') {
            setTimeout(function () {
                alert.classList.add('alert-hidden');
            }, 5000);
        }
    }

    function setupCollapsible() {
        var headers = document.querySelectorAll('.config-card-header[data-collapse]');
        for (var i = 0; i < headers.length; i++) {
            headers[i].addEventListener('click', function () {
                var card = this.closest('.config-card');
                if (card) {
                    card.classList.toggle('collapsed');
                }
            });
        }
    }

    if (document.getElementById('config-form')) {
        loadConfig();
        setupConfigForm();
        setupCollapsible();
    }

    // ── Alerts Per-Page Dropdown ─────────────────────────────

    var perPageSelect = document.getElementById('per-page-select');
    if (perPageSelect) {
        perPageSelect.addEventListener('change', function () {
            var val = this.value;
            document.cookie = 'pingpong_alerts_per_page=' + val + ';path=/;max-age=31536000;SameSite=Lax';
            window.location.href = '/alerts?page=1&perPage=' + val;
        });
    }

    // ── Alert Deletion ───────────────────────────────────────

    var clearAllBtn = document.getElementById('clear-all-alerts');
    if (clearAllBtn) {
        clearAllBtn.addEventListener('click', function () {
            if (!confirm('Delete all alerts? This cannot be undone.')) return;
            fetch('/api/alerts', { method: 'DELETE' })
                .then(function (res) {
                    if (!res.ok) throw new Error('Delete failed');
                    window.location.href = '/alerts';
                })
                .catch(function (err) {
                    alert('Failed to delete alerts: ' + err.message);
                });
        });
    }

    var deleteButtons = document.querySelectorAll('.btn-delete-alert');
    for (var i = 0; i < deleteButtons.length; i++) {
        deleteButtons[i].addEventListener('click', function () {
            var id = this.getAttribute('data-alert-id');
            if (!confirm('Delete this alert?')) return;
            fetch('/api/alerts/' + id, { method: 'DELETE' })
                .then(function (res) {
                    if (!res.ok) throw new Error('Delete failed');
                    window.location.href = '/alerts';
                })
                .catch(function (err) {
                    alert('Failed to delete alert: ' + err.message);
                });
        });
    }

})();
