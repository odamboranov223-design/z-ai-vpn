/* ═══════════════════════════════════════════════════════════════
   Z AI VPN — UI logic
   Keeps the same backend API contract; renders into the new
   monochrome DOM structure with orb, ping bars, country codes.
   ═══════════════════════════════════════════════════════════════ */
'use strict';

const $  = (id) => document.getElementById(id);
const el = (tag, cls, text) => {
  const n = document.createElement(tag);
  if (cls) n.className = cls;
  if (text !== undefined) n.textContent = text;
  return n;
};

// One-off token handed out by the local server.
const TOKEN = new URLSearchParams(location.search).get('token') || '';

// Country code from country name (RU → RU, Germany → DE, etc.)
const CC_MAP = {
  'Россия':'RU','Russian Federation':'RU','Russia':'RU',
  'Германия':'DE','Germany':'DE',
  'Нидерланды':'NL','Netherlands':'NL',
  'США':'US','United States':'US','USA':'US',
  'Великобритания':'GB','United Kingdom':'GB','UK':'GB',
  'Франция':'FR','France':'FR',
  'Япония':'JP','Japan':'JP',
  'Сингапур':'SG','Singapore':'SG',
  'Гонконг':'HK','Hong Kong':'HK',
  'Турция':'TR','Turkey':'TR',
  'Финляндия':'FI','Finland':'FI',
  'Швеция':'SE','Sweden':'SE',
  'Швейцария':'CH','Switzerland':'CH',
  'Канада':'CA','Canada':'CA',
  'Австралия':'AU','Australia':'AU',
  'Италия':'IT','Italy':'IT',
  'Испания':'ES','Spain':'ES',
  'Польша':'PL','Poland':'PL',
  'Украина':'UA','Ukraine':'UA',
  'Казахстан':'KZ','Kazakhstan':'KZ',
  'Латвия':'LV','Latvia':'LV',
  'Эстония':'EE','Estonia':'EE',
  'Литва':'LT','Lithuania':'LT',
  'Чехия':'CZ','Czech Republic':'CZ','Czechia':'CZ',
  'Австрия':'AT','Austria':'AT',
  'Бельгия':'BE','Belgium':'BE',
  'Дания':'DK','Denmark':'DK',
  'Норвегия':'NO','Norway':'NO',
  'Румыния':'RO','Romania':'RO',
  'Болгария':'BG','Bulgaria':'BG',
  'Греция':'GR','Greece':'GR',
  'Португалия':'PT','Portugal':'PT',
  'Ирландия':'IE','Ireland':'IE',
  'Исландия':'IS','Iceland':'IS',
  'Израиль':'IL','Israel':'IL',
  'ОАЭ':'AE','UAE':'AE','United Arab Emirates':'AE',
  'Южная Корея':'KR','South Korea':'KR','Korea':'KR',
  'Индия':'IN','India':'IN',
  'Бразилия':'BR','Brazil':'BR',
  'Мексика':'MX','Mexico':'MX',
  'Аргентина':'AR','Argentina':'AR',
  'Чили':'CL','Chile':'CL',
  'ЮАР':'ZA','South Africa':'ZA',
};
function countryCode(name) {
  if (!name) return '—';
  if (name.length === 2 && name === name.toUpperCase()) return name;
  if (CC_MAP[name]) return CC_MAP[name];
  // try first word uppercase
  const words = name.trim().split(/\s+/);
  if (words.length >= 2) {
    const cc = (words[0][0] + words[1][0]).toUpperCase();
    if (cc.length === 2) return cc;
  }
  return name.slice(0, 2).toUpperCase();
}

// Ping → bar class
function pingClass(p) {
  if (p === null || p === undefined) return '';
  if (p < 150) return 'good';
  if (p < 400) return 'mid';
  return 'bad';
}

const state = {
  servers: [],
  selected: null,
  currentId: null,
  connected: false,
  busy: false,
  ip: '',
  settings: {},
  sortByPing: false,
  view: 'servers',
  sessionStart: null,
};

/* ─────────────────────────── api ─────────────────────────── */
async function api(path, opts = {}) {
  const res = await fetch(path, {
    headers: { 'Content-Type': 'application/json', 'X-Token': TOKEN },
    ...opts,
    body: opts.body ? JSON.stringify(opts.body) : undefined,
  });
  if (!res.ok) {
    let msg = res.statusText;
    try { msg = (await res.json()).error || msg; } catch (_) {}
    throw new Error(msg);
  }
  return res.status === 204 ? null : res.json();
}

async function refreshState() {
  const s = await api('/api/state');
  state.connected = !!s.connected;
  state.busy = !!s.busy;
  state.currentId = s.current ? s.current.id : null;
  state.ip = s.ip || '';
  if (state.connected && !state.sessionStart) state.sessionStart = Date.now();
  if (!state.connected) state.sessionStart = null;
  applyStatus(s);
}

async function refreshServers() {
  state.servers = await api('/api/servers');
  renderList();
  renderDetail();
}

async function refreshSettings() {
  state.settings = await api('/api/settings');
  renderSettings();
}

/* ─────────────────────────── status / orb / strip ─────────────────────────── */
function applyStatus(s) {
  document.body.classList.toggle('is-connected', !!s.connected);
  document.body.classList.toggle('is-busy', !!s.busy);

  // rail status text
  let text = 'Отключено';
  if (s.busy) text = 'Подключение…';
  else if (s.connected) {
    const cur = s.current || {};
    text = (cur.country || cur.name || 'Подключено').slice(0, 14);
  }
  $('statusText').textContent = text;
  $('powerLabel').textContent = s.busy ? '…' : (s.connected ? 'ON' : 'OFF');

  // make status text brighter when connected (was too faint)
  $('statusText').style.color = s.connected ? 'var(--fg-1)' : 'var(--fg-3)';

  // title bar status badge
  const tbStatus = $('tbStatus');
  if (tbStatus) {
    if (s.busy)      tbStatus.textContent = '● CONNECTING';
    else if (s.connected) tbStatus.textContent = '● ONLINE · ' + (s.ip || '');
    else             tbStatus.textContent = '● OFFLINE';
  }

  // orb value
  let orbText = 'OFFLINE';
  if (s.busy) orbText = 'CONNECTING';
  else if (s.connected) orbText = 'ONLINE';
  $('orbValue').textContent = orbText;

  // connect button
  const btn = $('connectBtn');
  const btnLabel = btn.querySelector('span:last-child');
  btnLabel.textContent = s.connected ? 'Отключиться' : 'Подключиться';

  // strip
  $('stripIp').textContent = s.ip || '—';
  if (s.current) {
    $('stripServer').textContent = s.current.name || s.current.country || '—';
  } else {
    $('stripServer').textContent = '—';
  }

  // view sub
  $('viewSub').textContent = s.connected
    ? 'Подключено' + (s.current ? ' · ' + (s.current.name || '').slice(0, 22) : '')
    : 'Ничего не подключено';
}

function tickSession() {
  const el = $('stripSession');
  if (!state.sessionStart) { el.textContent = '00:00:00'; return; }
  const sec = Math.floor((Date.now() - state.sessionStart) / 1000);
  const h = String(Math.floor(sec / 3600)).padStart(2, '0');
  const m = String(Math.floor((sec % 3600) / 60)).padStart(2, '0');
  const s = String(sec % 60).padStart(2, '0');
  el.textContent = `${h}:${m}:${s}`;
}

/* ─────────────────────────── list ─────────────────────────── */
function filteredServers() {
  const q = ($('search').value || '').trim().toLowerCase();
  const sub = $('subFilter').value;
  let list = state.servers.filter((s) => {
    if (sub && s.subId !== sub) return false;
    if (!q) return true;
    return (s.name + ' ' + (s.country || '') + ' ' + (s.address || '')).toLowerCase().includes(q);
  });
  if (state.sortByPing) {
    list = list.slice().sort((a, b) => {
      const av = a.ping === null || a.ping === undefined ? 1e9 : a.ping;
      const bv = b.ping === null || b.ping === undefined ? 1e9 : b.ping;
      return av - bv;
    });
  }
  return list;
}

function renderList() {
  const list = filteredServers();
  const box = $('serverList');
  box.textContent = '';

  $('emptyState').hidden = list.length > 0;
  $('serverList').hidden = list.length === 0;

  for (const s of list) {
    const cls = 'server-item'
      + (state.selected === s.id ? ' is-selected' : '')
      + (state.currentId === s.id && state.connected ? ' is-connected' : '');
    const item = el('div', cls);
    item.onclick = () => { state.selected = s.id; renderList(); renderDetail(); };

    // country code badge
    const cc = countryCode(s.country);
    item.appendChild(el('div', 'flag-cc', cc));

    const meta = el('div', 'meta');
    meta.appendChild(el('div', 'name', s.name));
    const sub = el('div', 'sub');
    const parts = [s.country, s.protocol].filter(Boolean);
    sub.textContent = parts.join(' · ');
    meta.appendChild(sub);
    item.appendChild(meta);

    // ping bars
    const pb = el('div', 'ping-bars ' + pingClass(s.ping));
    for (let i = 0; i < 5; i++) pb.appendChild(el('div', 'bar'));
    item.appendChild(pb);

    // ping number
    const pn = el('div', 'ping-num',
      (s.ping === null || s.ping === undefined) ? '—' : (s.ping + 'ms'));
    item.appendChild(pn);

    box.appendChild(item);
  }
}

/* ─────────────────────────── detail ─────────────────────────── */
function renderDetail() {
  const s = state.servers.find((x) => x.id === state.selected);
  $('detailEmpty').hidden = !!s;
  $('detail').hidden = !s;
  if (!s) return;

  $('dName').textContent = s.name || s.address || '—';
  $('dCountry').textContent = [s.country, s.subName].filter(Boolean).join(' · ') || '—';
  $('heroCountry').textContent = countryCode(s.country);

  $('dProto').textContent = s.protocol || '—';
  $('dTransport').textContent = s.transport || '—';
  $('dSecurity').textContent = s.security || '—';
  $('dAddr').textContent = s.address ? `${s.address}:${s.port}` : '—';
  $('dSub').textContent = s.subName || '—';
  $('dTraffic').textContent = s.traffic || '—';
  $('dExpiry').textContent = s.expiry || '—';

  $('dPing').textContent = (s.ping === null || s.ping === undefined) ? 'не измерен' : (s.ping + ' ms');

  $('apiPanel').hidden = false;

  const btn = $('connectBtn');
  const lbl = btn.querySelector('span:last-child');
  if (state.connected) {
    lbl.textContent = state.currentId === s.id ? 'Отключиться' : 'Переключиться';
  } else {
    lbl.textContent = 'Подключиться';
  }
}

/* ─────────────────────────── settings ─────────────────────────── */
function renderSettings() {
  const st = state.settings || {};
  $('bypassRU').checked = !!st.bypassRU;
  $('sysProxy').checked = !!st.sysProxy;
  $('checkIP').checked = !!st.checkIP;
  $('trayMode').checked = !!st.trayMode;

  const box = $('subList');
  box.textContent = '';
  for (const sub of st.subscriptions || []) {
    const item = el('div', 'sub-item');
    const meta = el('div', 'meta');
    meta.appendChild(el('div', 'name', sub.name || 'Подписка'));
    meta.appendChild(el('div', 'url', sub.url));
    item.appendChild(meta);
    item.appendChild(el('span', 'badge', (sub.serverCount || 0) + ' серв.'));

    const del = el('button', 'ghost-btn danger', 'Удалить');
    del.onclick = async () => {
      if (!confirm(`Удалить подписку «${sub.name || sub.url}»?`)) return;
      await api('/api/subscriptions/delete', { method: 'POST', body: { id: sub.id } });
      toast('Подписка удалена');
      await refreshSettings();
      await refreshServers();
    };
    item.appendChild(del);
    box.appendChild(item);
  }

  const filter = $('subFilter');
  const current = filter.value;
  filter.textContent = '';
  filter.appendChild(new Option('Все', ''));
  for (const sub of st.subscriptions || []) {
    filter.appendChild(new Option(sub.name || sub.url, sub.id));
  }
  filter.value = current;

  $('aboutVer').textContent = st.version || '—';
  $('aboutEngine').textContent = st.engine || '—';
  $('aboutData').textContent = st.dataDir || '—';
  $('aboutPorts').textContent = st.ports || '—';
  $('stripVersion').textContent = st.version || '—';
}

/* ─────────────────────────── actions ─────────────────────────── */
function toast(msg) {
  const t = $('toast');
  t.textContent = msg;
  t.hidden = false;
  clearTimeout(toast._t);
  toast._t = setTimeout(() => { t.hidden = true; }, 2800);
}

async function guard(fn, busyMsg) {
  if (state.busy) return;
  state.busy = true;
  document.body.classList.add('is-busy');
  if (busyMsg) toast(busyMsg);
  try {
    await fn();
  } catch (e) {
    toast('Ошибка: ' + e.message);
  } finally {
    state.busy = false;
    document.body.classList.remove('is-busy');
    await refreshState();
  }
}

async function togglePower() {
  if (!state.selected && !state.connected) {
    toast('Сначала выбери сервер');
    return;
  }
  await guard(async () => {
    if (state.connected) {
      await api('/api/disconnect', { method: 'POST' });
      toast('Отключено');
    } else {
      const r = await api('/api/connect', { method: 'POST', body: { id: state.selected } });
      toast(r.message || 'Подключено');
    }
    await refreshServers();
  }, state.connected ? null : 'Подключение…');
}

async function pingAll() {
  await guard(async () => {
    const r = await api('/api/ping', { method: 'POST', body: { id: state.selected } });
    await refreshServers();
    toast(r.message || 'Пинг завершён');
  }, 'Пингуем…');
}

/* ─────────────────────────── modal ─────────────────────────── */
function openModal(title, hint, placeholder, onOk) {
  $('modalTitle').textContent = title;
  $('modalHint').textContent = hint;
  $('modalInput').value = '';
  $('modalInput').placeholder = placeholder || '';
  $('modalError').hidden = true;
  $('modal').hidden = false;
  $('modalInput').focus();

  $('modalOk').onclick = async () => {
    const text = $('modalInput').value.trim();
    if (!text) return;
    try {
      await onOk(text);
      $('modal').hidden = true;
    } catch (e) {
      $('modalError').textContent = e.message;
      $('modalError').hidden = false;
    }
  };
}
const closeModal = () => { $('modal').hidden = true; };

/* ─────────────────────────── wiring ─────────────────────────── */
function switchView(view) {
  state.view = view;
  document.getElementById('app').classList.toggle('settings-mode', view === 'settings');
  $('settingsView').hidden = view !== 'settings';
  $('viewTitle').textContent = view === 'settings' ? 'Настройки' : 'Серверы';
  for (const t of document.querySelectorAll('.tab')) {
    t.classList.toggle('is-active', t.dataset.view === view);
  }
  if (view === 'settings') refreshSettings();
}

async function boot() {
  await Promise.all([refreshState(), refreshServers(), refreshSettings()]);

  $('power').onclick = togglePower;
  $('connectBtn').onclick = togglePower;
  $('pingBtn').onclick = pingAll;
  $('refreshBtn').onclick = () => guard(async () => {
    await api('/api/subscriptions/refresh', { method: 'POST' });
    await refreshServers();
    toast('Подписки обновлены');
  }, 'Обновляем…');

  $('deleteBtn').onclick = async () => {
    const s = state.servers.find((x) => x.id === state.selected);
    if (!s || !confirm(`Удалить «${s.name}»?`)) return;
    await api('/api/servers/delete', { method: 'POST', body: { id: s.id } });
    state.selected = null;
    await refreshServers();
    toast('Сервер удалён');
  };

  $('search').oninput = renderList;
  $('subFilter').onchange = renderList;
  $('sortBtn').onclick = () => { state.sortByPing = !state.sortByPing; renderList(); };

  for (const t of document.querySelectorAll('.tab')) {
    t.onclick = () => switchView(t.dataset.view);
  }

  $('addBtn').onclick = () => openModal(
    'Добавить сервер',
    'Вставь одну ссылку или несколько, каждую с новой строки.',
    'vless://…',
    async (text) => {
      const r = await api('/api/servers/add', { method: 'POST', body: { text } });
      await refreshServers();
      toast(r.message || 'Добавлено');
    });

  $('emptyAdd').onclick = $('addBtn').onclick;

  $('addSubBtn').onclick = async () => {
    const url = $('newSubUrl').value.trim();
    if (!url) { toast('Укажи ссылку на подписку'); return; }
    const name = $('newSubName').value.trim();
    await api('/api/subscriptions/add', { method: 'POST', body: { url, name } });
    $('newSubUrl').value = ''; $('newSubName').value = '';
    await refreshSettings();
    await refreshServers();
    toast('Подписка добавлена');
  };

  $('importBtn').onclick = async () => {
    const text = $('importBox').value.trim();
    if (!text) { toast('Пусто'); return; }
    const r = await api('/api/servers/add', { method: 'POST', body: { text } });
    $('importBox').value = '';
    await refreshServers();
    toast(r.message || 'Импортировано');
  };

  for (const key of ['bypassRU', 'sysProxy', 'checkIP', 'trayMode']) {
    $(key).onchange = async () => {
      const body = {};
      for (const k of ['bypassRU', 'sysProxy', 'checkIP', 'trayMode']) body[k] = $(k).checked;
      await api('/api/settings', { method: 'POST', body });
      toast('Сохранено');
    };
  }

  $('themeBtn').onclick = () => {
    const cur = document.documentElement.dataset.theme;
    const next = cur === 'dark' ? 'light' : 'dark';
    document.documentElement.dataset.theme = next;
    localStorage.setItem('theme', next);
  };
  const savedTheme = localStorage.getItem('theme');
  if (savedTheme) document.documentElement.dataset.theme = savedTheme;

  // ── custom title bar (frameless window) ──
  wireTitleBar();

  // ── animated background system ──
  wireBackgrounds();

  $('modalCancel').onclick = closeModal;
  $('modal').onclick = (e) => { if (e.target === $('modal')) closeModal(); };

  // Enter submits modal
  $('modalInput').addEventListener('keydown', (e) => {
    if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); $('modalOk').click(); }
    if (e.key === 'Escape') closeModal();
  });

  // Poll state every 2s; tick session timer every 1s
  setInterval(refreshState, 2000);
  setInterval(tickSession, 1000);
  tickSession();
}

/* ═══════════════════════════════════════════════════════════════
   TITLE BAR — frameless window controls
   The native Go side exposes windowMinimize / windowToggleMax /
   windowClose / windowDrag to JS via WebView2 bindings.
   In a regular browser these are undefined; we fall back to no-op.
   ═══════════════════════════════════════════════════════════════ */
function wireTitleBar() {
  const call = (name) => {
    if (typeof window[name] === 'function') {
      try { window[name](); } catch (e) { /* ignore */ }
    }
  };

  const tbMin = $('tbMin'); if (tbMin)   tbMin.onclick  = () => call('windowMinimize');
  const tbMax = $('tbMax'); if (tbMax)   tbMax.onclick  = () => call('windowToggleMax');
  const tbClose = $('tbClose'); if (tbClose) tbClose.onclick = () => call('windowClose');

  // Drag: only when mousedown on the drag area itself (not on the buttons)
  const drag = $('titlebarDrag');
  if (drag) {
    drag.addEventListener('mousedown', (e) => {
      if (e.target.closest('.titlebar-tabs') || e.target.closest('.titlebar-brand')) {
        // still allow drag from brand area but not from the status pill
        if (e.target.closest('.titlebar-tab')) return;
      }
      if (e.button !== 0) return;
      call('windowDrag');
    });
    // double-click toggles maximize
    drag.addEventListener('dblclick', () => call('windowToggleMax'));
  }
}

/* ═══════════════════════════════════════════════════════════════
   ANIMATED BACKGROUNDS
   Aurora  — pure CSS radial gradients + drift animation
   Grid    — pure CSS linear-gradient grid + mask + shift animation
   Particles / Starfield — canvas-based, drawn every frame
   ═══════════════════════════════════════════════════════════════ */
const BG_KEY = 'z-ai-vpn-bg';
const canvas = document.getElementById('bgCanvas');
const ctx = canvas ? canvas.getContext('2d') : null;
let bgMode = 'aurora';
let bgRAF = null;
let particles = [];
let stars = [];

function resizeCanvas() {
  if (!canvas || !ctx) return;
  const dpr = Math.min(window.devicePixelRatio || 1, 2);
  canvas.width  = Math.floor(window.innerWidth * dpr);
  canvas.height = Math.floor(window.innerHeight * dpr);
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
}

function initParticles() {
  particles = [];
  const count = Math.min(120, Math.floor(window.innerWidth / 14));
  for (let i = 0; i < count; i++) {
    particles.push({
      x: Math.random() * window.innerWidth,
      y: Math.random() * window.innerHeight,
      vx: (Math.random() - 0.5) * 0.4,
      vy: (Math.random() - 0.5) * 0.4,
      r: Math.random() * 2.2 + 0.6,
      a: Math.random() * 0.5 + 0.3,
    });
  }
}

function initStars() {
  stars = [];
  const count = Math.min(260, Math.floor(window.innerWidth * window.innerHeight / 6000));
  for (let i = 0; i < count; i++) {
    stars.push({
      x: Math.random() * window.innerWidth,
      y: Math.random() * window.innerHeight,
      r: Math.random() * 1.6 + 0.3,
      tw: Math.random() * Math.PI * 2,
      sp: Math.random() * 0.03 + 0.005,
      base: Math.random() * 0.5 + 0.3,
    });
  }
}

function drawParticles() {
  if (!ctx) return;
  const W = window.innerWidth, H = window.innerHeight;
  ctx.clearRect(0, 0, W, H);
  // dots with glow
  for (const p of particles) {
    p.x += p.vx; p.y += p.vy;
    if (p.x < 0) p.x = W; else if (p.x > W) p.x = 0;
    if (p.y < 0) p.y = H; else if (p.y > H) p.y = 0;
    // outer glow
    ctx.beginPath();
    ctx.arc(p.x, p.y, p.r * 2.5, 0, Math.PI * 2);
    ctx.fillStyle = `rgba(255,255,255,${p.a * 0.15})`;
    ctx.fill();
    // core
    ctx.beginPath();
    ctx.arc(p.x, p.y, p.r, 0, Math.PI * 2);
    ctx.fillStyle = `rgba(255,255,255,${p.a})`;
    ctx.fill();
  }
  // connecting lines — closer pairs only
  for (let i = 0; i < particles.length; i++) {
    for (let j = i + 1; j < particles.length; j++) {
      const a = particles[i], b = particles[j];
      const dx = a.x - b.x, dy = a.y - b.y;
      const d2 = dx * dx + dy * dy;
      if (d2 < 130 * 130) {
        const alpha = (1 - Math.sqrt(d2) / 130) * 0.18;
        ctx.beginPath();
        ctx.moveTo(a.x, a.y);
        ctx.lineTo(b.x, b.y);
        ctx.strokeStyle = `rgba(255,255,255,${alpha})`;
        ctx.lineWidth = 0.6;
        ctx.stroke();
      }
    }
  }
}

function drawStars() {
  if (!ctx) return;
  const W = window.innerWidth, H = window.innerHeight;
  ctx.clearRect(0, 0, W, H);
  for (const s of stars) {
    s.tw += s.sp;
    const alpha = s.base + Math.sin(s.tw) * 0.45;
    const a = Math.max(0.05, Math.min(1, alpha));
    // soft halo
    ctx.beginPath();
    ctx.arc(s.x, s.y, s.r * 3, 0, Math.PI * 2);
    ctx.fillStyle = `rgba(255,255,255,${a * 0.08})`;
    ctx.fill();
    // core
    ctx.beginPath();
    ctx.arc(s.x, s.y, s.r, 0, Math.PI * 2);
    ctx.fillStyle = `rgba(255,255,255,${a})`;
    ctx.fill();
    // diffraction cross on brightest stars
    if (s.r > 1.0 && a > 0.6) {
      ctx.strokeStyle = `rgba(255,255,255,${a * 0.5})`;
      ctx.lineWidth = 0.6;
      ctx.beginPath();
      ctx.moveTo(s.x - 4, s.y); ctx.lineTo(s.x + 4, s.y);
      ctx.moveTo(s.x, s.y - 4); ctx.lineTo(s.x, s.y + 4);
      ctx.stroke();
    }
  }
}

function bgLoop() {
  if (bgMode === 'particles')      drawParticles();
  else if (bgMode === 'starfield') drawStars();
  else { bgRAF = null; return; }   // nothing to animate, stop loop
  bgRAF = requestAnimationFrame(bgLoop);
}

function setBackground(mode) {
  bgMode = mode;
  document.body.dataset.bg = mode;
  localStorage.setItem(BG_KEY, mode);

  // mark active button
  for (const btn of document.querySelectorAll('.bg-opt')) {
    btn.classList.toggle('is-active', btn.dataset.bg === mode);
  }

  // (re)init canvas particles / stars
  if (canvas) {
    resizeCanvas();
    if (mode === 'particles') { initParticles(); }
    else if (mode === 'starfield') { initStars(); }
  }

  // start or stop the loop
  if (bgRAF) { cancelAnimationFrame(bgRAF); bgRAF = null; }
  if (mode === 'particles' || mode === 'starfield') {
    bgRAF = requestAnimationFrame(bgLoop);
  }
}

function wireBackgrounds() {
  // expose setBackground globally so the devtools / Playwright can call it
  window.setBackground = setBackground;

  // restore saved choice (default: aurora)
  const saved = localStorage.getItem(BG_KEY) || 'aurora';
  setBackground(saved);

  // option buttons
  for (const btn of document.querySelectorAll('.bg-opt')) {
    btn.onclick = () => setBackground(btn.dataset.bg);
  }

  // resize handler — debounced
  let resizeT;
  window.addEventListener('resize', () => {
    clearTimeout(resizeT);
    resizeT = setTimeout(() => {
      if (canvas) {
        resizeCanvas();
        if (bgMode === 'particles') initParticles();
        else if (bgMode === 'starfield') initStars();
      }
    }, 150);
  });
}

boot().catch((e) => {
  document.body.innerHTML =
    '<div style="padding:40px;color:#f5f5f5;font:14px system-ui;background:#050505;height:100vh">' +
    'Не удалось загрузить интерфейс: ' + e.message + '</div>';
});
