package api

// dashboardHTML is the single-file live dashboard: event ticker, call
// states, tool/action audit log, transcripts. Vanilla JS, polls /api/sessions.
const dashboardHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>calle — event → call → outcome</title>
<style>
  :root { color-scheme: dark; }
  * { box-sizing: border-box; }
  body { margin: 0; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; background: #0b0e14; color: #d5dae2; }
  header { padding: 20px 24px; border-bottom: 1px solid #1e2530; display: flex; align-items: baseline; gap: 14px; }
  header h1 { font-size: 20px; margin: 0; color: #7dd3fc; }
  header span { color: #6b7688; font-size: 13px; }
  main { padding: 20px 24px; max-width: 1100px; margin: 0 auto; }
  .session { border: 1px solid #1e2530; border-radius: 10px; padding: 14px 16px; margin-bottom: 14px; background: #0f131b; }
  .row { display: flex; gap: 10px; flex-wrap: wrap; align-items: center; }
  .pill { font-size: 12px; padding: 2px 10px; border-radius: 99px; border: 1px solid #2a3342; }
  .pill.type { color: #fbbf24; border-color: #4d3d12; }
  .pill.status { color: #a5f3a5; border-color: #1f4022; }
  .pill.status.bad { color: #fca5a5; border-color: #4d1517; }
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
  .empty { color: #4d5a70; text-align: center; padding: 60px 0; }
  .fire { display: flex; gap: 8px; margin-bottom: 20px; flex-wrap: wrap; }
  button { font-family: inherit; background: #16202e; color: #7dd3fc; border: 1px solid #234; border-radius: 8px; padding: 8px 14px; cursor: pointer; font-size: 13px; }
  button:hover { background: #1b2738; }
</style>
</head>
<body>
<header><h1>calle</h1><span>event → intelligent call → structured outcome</span></header>
<main>
  <div class="fire">
    <button onclick="fire('invoice.due')">⚡ fire invoice.due (cus_1002)</button>
    <button onclick="fire('account.warning')">⚡ fire account.warning (cus_1003)</button>
    <button onclick="fire('promo.offer')">⚡ fire promo.offer (cus_1001)</button>
  </div>
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
  await fetch('/api/events', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      id: 'evt_' + Math.random().toString(36).slice(2, 10),
      type, customer_id: cust, payload: payloads[type],
    }),
  });
  setTimeout(render, 400);
}
async function render() {
  const res = await fetch('/api/sessions');
  const sessions = await res.json();
  const list = document.getElementById('list');
  if (!sessions.length) return;
  list.innerHTML = sessions.map(s => {
    const actions = (s.actions || []).map(a =>
      '<div class="action"><span class="t">' + new Date(a.at).toLocaleTimeString() + '</span><span class="kind">' + a.kind + '</span> ' + esc(a.detail) + '</div>').join('');
    const outcome = s.outcome ? '<div class="outcome">⇒ ' + esc(JSON.stringify(s.outcome)) + '</div>' : '';
    const bad = s.call_status === 'failed' || s.call_status === 'canceled';
    return '<div class="session"><div class="row">' +
      '<span class="name">' + esc(s.customer || s.customer_id) + '</span>' +
      '<span class="pill type">' + esc(s.event_type) + '</span>' +
      '<span class="phone">' + esc(s.phone || '') + '</span>' +
      '<span class="pill status' + (bad ? ' bad' : '') + '">' + esc(s.call_status || 'intake') + '</span>' +
      '</div>' + outcome + '<div class="actions">' + actions + '</div>' +
      '<details><summary>task prompt sent to CALL-E</summary><pre>' + esc(s.task || '') + '</pre></details></div>';
  }).join('');
}
function esc(s) { return String(s ?? '').replace(/[&<>"]/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;'}[c])); }
render();
setInterval(render, 2000);
</script>
</body>
</html>`
