package api

// dashboardHTML is the single-file live dashboard: metrics header, event
// ticker, call states, transcripts, action audit log. Vanilla JS, polls
// /api/sessions and /api/metrics.
const dashboardHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>callhook — event → call → outcome</title>
<style>
  :root { color-scheme: dark; }
  * { box-sizing: border-box; }
  body { margin: 0; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; background: #0b0e14; color: #d5dae2; }
  header { padding: 16px 24px; border-bottom: 1px solid #1e2530; display: flex; align-items: baseline; gap: 14px; flex-wrap: wrap; }
  header h1 { font-size: 20px; margin: 0; color: #7dd3fc; }
  header span { color: #6b7688; font-size: 13px; }
  #metrics { padding: 10px 24px; border-bottom: 1px solid #1e2530; font-size: 13px; color: #9aa5b5; display: flex; gap: 18px; flex-wrap: wrap; }
  #metrics b { color: #e7ecf3; }
  main { padding: 20px 24px; max-width: 1100px; margin: 0 auto; }
  .session { border: 1px solid #1e2530; border-radius: 10px; padding: 14px 16px; margin-bottom: 14px; background: #0f131b; }
  .row { display: flex; gap: 10px; flex-wrap: wrap; align-items: center; }
  .pill { font-size: 12px; padding: 2px 10px; border-radius: 99px; border: 1px solid #2a3342; }
  .pill.type { color: #fbbf24; border-color: #4d3d12; }
  .pill.status { color: #a5f3a5; border-color: #1f4022; }
  .pill.status.bad { color: #fca5a5; border-color: #4d1517; }
  .pill.wait { color: #c4b5fd; border-color: #3b2f5e; }
  .name { font-weight: bold; color: #e7ecf3; }
  .phone { color: #6b7688; }
  .actions { margin-top: 10px; font-size: 12.5px; }
  .action { padding: 3px 0 3px 12px; border-left: 2px solid #223042; color: #9aa5b5; margin-left: 4px; }
  .action .kind { color: #7dd3fc; }
  .action .t { color: #4d5a70; margin-right: 8px; }
  .outcome { margin-top: 8px; font-size: 13px; color: #fbbf24; }
  details { margin-top: 8px; }
  summary { cursor: pointer; color: #6b7688; font-size: 12px; }
  pre { background: #0b0e14; border: 1px solid #1e2530; padding: 10px; border-radius: 8px; font-size: 12px; overflow-x: auto; white-space: pre-wrap; color: #b7c2d0; }
  .turn { padding: 2px 0; }
  .turn .bot { color: #7dd3fc; }
  .turn .user { color: #a5f3a5; }
  .empty { color: #4d5a70; text-align: center; padding: 60px 0; }
  .fire { display: flex; gap: 8px; margin-bottom: 20px; flex-wrap: wrap; }
  button { font-family: inherit; background: #16202e; color: #7dd3fc; border: 1px solid #234; border-radius: 8px; padding: 8px 14px; cursor: pointer; font-size: 13px; }
  button:hover { background: #1b2738; }
  .campaign { border: 1px solid #2a3342; border-radius: 10px; padding: 14px 16px; margin-bottom: 16px; background: #0d1420; }
  .campaign .head { display: flex; gap: 10px; align-items: center; flex-wrap: wrap; }
  .campaign .title { font-weight: bold; color: #e7ecf3; }
  .bar { margin-top: 10px; height: 14px; border-radius: 7px; background: #131926; border: 1px solid #1e2530; overflow: hidden; }
  .bar .fill { height: 100%; background: linear-gradient(90deg,#38bdf8,#a5f3a5); transition: width .4s; }
  .campaign .stats { display: flex; gap: 16px; margin-top: 8px; font-size: 12.5px; color: #9aa5b5; flex-wrap: wrap; }
  .campaign .stats b { color: #e7ecf3; }
  .clog { margin-top: 8px; font-size: 12px; color: #6b7688; }
  .clog div { padding: 1px 0 1px 10px; border-left: 2px solid #223042; margin-left: 3px; }
  .pill.cmp { color: #c4b5fd; border-color: #3b2f5e; }
  .pill.cmp.done { color: #a5f3a5; border-color: #1f4022; }
</style>
</head>
<body>
<header><h1>callhook</h1><span>event → intelligent call → structured outcome</span><span id="mode"></span></header>
<div id="metrics"></div>
<main>
  <div class="fire">
    <button onclick="fire('invoice.due')">fire invoice.due (cus_1002)</button>
    <button onclick="fire('account.warning')">fire account.warning (cus_1003)</button>
    <button onclick="fire('promo.offer')">fire promo.offer (cus_1001)</button>
    <button onclick="launchCampaign()">launch demo campaign</button>
  </div>
  <div id="campaigns"></div>
  <div id="list"><div class="empty">no sessions yet — fire an event above (or POST /api/events)</div></div>
</main>
<script>
async function fire(type) {
  const cust = { 'invoice.due': 'cus_1002', 'account.warning': 'cus_1003', 'promo.offer': 'cus_1001' }[type];
  const payloads = {
    'invoice.due': {},
    'account.warning': { reason: 'login from a new country', detail: 'A sign-in from Singapore was detected.' },
    'promo.offer': { offer: '20% off your next invoice', expires: 'end of this week' },
  };
  const res = await fetch('/api/events', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      id: 'evt_' + Math.random().toString(36).slice(2, 10),
      type, customer_id: cust, payload: payloads[type],
    }),
  });
  const body = await res.json();
  if (body.status) console.log('callhook:', body.status, body.call_id || body.reason || '');
  setTimeout(render, 400);
}
async function launchCampaign() {
  const res = await fetch('/api/campaigns', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      name: 'September collections',
      event_type: 'invoice.due',
      goal: { type: 'count', target: 5, success_outcomes: ['payment_promised'] },
      audience_source: 'all_overdue',
      waves: { size: 3, delay: '15s', max_waves: 4 },
      budget: { max_calls: 12 },
    }),
  });
  const c = await res.json();
  if (c.id) console.log('campaign launched:', c.id);
  setTimeout(render, 500);
}
function renderCampaignsHTML(campaigns) {
  const el = document.getElementById('campaigns');
  if (!campaigns.length) { el.innerHTML = ''; return; }
  el.innerHTML = campaigns.map(c => {
    const target = c.goal.type === 'count' ? c.goal.target : c.audience.length;
    const pct = Math.min(100, Math.round(100 * c.progress.successes / target));
    const done = c.status !== 'running';
    const logs = (c.log || []).slice(-5).map(l =>
      '<div>' + new Date(l.at).toLocaleTimeString() + ' — ' + esc(l.event) + (l.note ? ': ' + esc(l.note) : '') + '</div>').join('');
    const nextWave = c.next_wave_at && !done ? ' · next wave ' + new Date(c.next_wave_at).toLocaleTimeString() : '';
    return '<div class="campaign"><div class="head">' +
      '<span class="title">' + esc(c.name) + '</span>' +
      '<span class="pill cmp' + (done ? ' done' : '') + '">' + esc(c.status) + '</span>' +
      '<span class="pill">' + c.wave_number + ' wave(s)' + nextWave + '</span>' +
      (c.campaign ? '' : '') +
      '</div>' +
      '<div class="bar"><div class="fill" style="width:' + pct + '%"></div></div>' +
      '<div class="stats">' +
      '<span>successes <b>' + c.progress.successes + '/' + target + '</b></span>' +
      '<span>placed <b>' + c.progress.calls_placed + '</b>/budget ' + c.budget.max_calls + '</span>' +
      '<span>pending <b>' + c.progress.pending + '</b></span>' +
      '<span>no-answer <b>' + c.progress.no_answer + '</b></span>' +
      '<span>failed <b>' + c.progress.failures + '</b></span>' +
      '</div><div class="clog">' + logs + '</div></div>';
  }).join('');
}
async function render() {
  const [sessRes, metRes, health, campRes] = await Promise.all([
    fetch('/api/sessions'), fetch('/api/metrics'), fetch('/api/health'), fetch('/api/campaigns').catch(() => null),
  ]);
  const sessions = await sessRes.json();
  const metrics = await metRes.json();
  const h = await health.json();
  if (campRes) renderCampaignsHTML(await campRes.json());
  document.getElementById('mode').textContent = h.dry_run ? '· DRY-RUN' : '· LIVE';
  document.getElementById('metrics').innerHTML =
    '<span>sessions <b>' + metrics.sessions + '</b></span>' +
    '<span>in flight <b>' + ((metrics.by_status||{}).in_progress || 0) + '</b></span>' +
    '<span>completed <b>' + ((metrics.by_status||{}).completed || 0) + '</b></span>' +
    '<span>failed <b>' + ((metrics.by_status||{}).failed || 0) + '</b></span>' +
    '<span>deferred/scheduled <b>' + (metrics.retries_armed || 0) + '</b></span>' +
    '<span>outcomes <b>' + Object.values(metrics.by_outcome || {}).reduce((a,b)=>a+b,0) + '</b></span>';
  const list = document.getElementById('list');
  if (!sessions.length) return;
  list.innerHTML = sessions.map(s => {
    const actions = (s.actions || []).map(a =>
      '<div class="action"><span class="t">' + new Date(a.at).toLocaleTimeString() + '</span><span class="kind">' + a.kind + '</span> ' + esc(a.detail) + '</div>').join('');
    const outcome = s.outcome ? '<div class="outcome">⇒ ' + esc(JSON.stringify(s.outcome)) + '</div>' : '';
    const bad = s.call_status === 'failed' || s.call_status === 'canceled';
    let wait = '';
    if (s.next_retry_at) {
      const label = { retry: 'redial', window: 'window', scheduled: 'scheduled' }[s.next_retry_kind] || 'pending';
      wait = '<span class="pill wait">' + label + ' @ ' + new Date(s.next_retry_at).toLocaleTimeString() + '</span>';
    }
    const transcript = (s.transcript || []).map(t =>
      '<div class="turn"><span class="' + t.speaker + '">' + t.speaker + ':</span> ' + esc(t.text) + '</div>').join('');
    const transcriptBlock = transcript ? '<details><summary>transcript</summary><pre>' + transcript + '</pre></details>' : '';
    return '<div class="session"><div class="row">' +
      '<span class="name">' + esc(s.customer || s.customer_id) + '</span>' +
      '<span class="pill type">' + esc(s.event_type) + '</span>' +
      '<span class="phone">' + esc(s.phone || '') + '</span>' +
      '<span class="pill status' + (bad ? ' bad' : '') + '">' + esc(s.call_status || 'intake') + '</span>' +
      wait +
      (s.retry_count ? '<span class="pill">attempt ' + (s.retry_count + 1) + '</span>' : '') +
      '</div>' + outcome + '<div class="actions">' + actions + '</div>' +
      transcriptBlock +
      '<details><summary>task prompt sent to CALL-E</summary><pre>' + esc(s.task || '') + '</pre></details></div>';
  }).join('');
}
function esc(s) { return String(s ?? '').replace(/[&<>"]/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;'}[c])); }
render();
setInterval(render, 2000);
</script>
</body>
</html>`
