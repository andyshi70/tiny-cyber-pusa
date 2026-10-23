import re

p = r'cmd\zenstats\main.go'
src = open(p, encoding='utf-8').read()

start = src.index('const dashHTML = `')
m = re.search(r'const dashHTML = `.*?`\n', src, re.S)
assert m, 'dashHTML block not found'

new_block = """const dashHTML = `<!doctype html>
<html lang="zh"><head><meta charset="utf-8">
<title>Zen Gate · 用户总览</title>
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="description" content="Zen Gate 本地免费模型网关 · 部署实例匿名心跳统计">
<link rel="icon" type="image/png" href="/logo.png">
<style>
:root { --bg:#141414; --panel:#1c1c1c; --panel-2:#262626; --ink:#ececec; --ink-dim:#a8a8a8; --ink-mute:#787878;
  --hairline:rgba(255,255,255,.08); --accent:#38bdf8; --ok:#34d399; --warn:#fbbf24; }
* { box-sizing:border-box; }
body { margin:0; background:radial-gradient(1200px 500px at 50% -200px, #1a2332 0%, var(--bg) 60%); color:var(--ink);
  font:14px/1.65 "Segoe UI",system-ui,-apple-system,sans-serif; min-height:100vh; }
.wrap { max-width:1100px; margin:0 auto; padding:26px 22px 70px; }
header { display:flex; align-items:center; gap:14px; margin-bottom:6px; }
header img { width:44px; height:44px; border-radius:10px; }
.tt h1 { font-size:19px; letter-spacing:.16em; margin:0; font-weight:600; }
.tt h1 b { color:var(--accent); font-weight:600; }
.tt .tag { font-size:12px; color:var(--ink-mute); }
.live { margin-left:auto; display:flex; align-items:center; gap:7px; font-size:12px; color:var(--ink-dim);
  background:var(--panel); border:1px solid var(--hairline); border-radius:20px; padding:5px 14px; }
.live .dot { width:8px; height:8px; border-radius:50%; background:var(--ok); animation:pulse 2s infinite; }
@keyframes pulse { 0%,100%{opacity:1} 50%{opacity:.35} }
.sub { color:var(--ink-mute); font-size:12px; margin:0 0 26px; padding-left:58px; }
.cards { display:grid; grid-template-columns:repeat(auto-fit,minmax(200px,1fr)); gap:12px; margin-bottom:18px; }
.card { background:var(--panel); border:1px solid var(--hairline); border-radius:12px; padding:16px 18px;
  display:flex; gap:14px; align-items:center; }
.card .ic { width:38px; height:38px; border-radius:9px; background:rgba(56,189,248,.12); display:flex;
  align-items:center; justify-content:center; flex:none; }
.card .ic svg { width:19px; height:19px; stroke:var(--accent); fill:none; stroke-width:2; stroke-linecap:round; stroke-linejoin:round; }
.card .v { font-size:26px; font-weight:700; line-height:1.1; font-variant-numeric:tabular-nums; }
.card .l { font-size:12px; color:var(--ink-mute); }
.panel { background:var(--panel); border:1px solid var(--hairline); border-radius:12px; padding:18px 20px; margin-bottom:14px; }
.panel h3 { margin:0 0 14px; font-size:12px; color:var(--ink-mute); letter-spacing:.1em; font-weight:600; }
.bar-row { display:flex; align-items:center; gap:12px; margin:8px 0; }
.bar-row .nm { width:130px; font-size:12.5px; color:var(--ink-dim); text-align:right; flex:none; }
.bar-row .tr { flex:1; height:16px; background:var(--panel-2); border-radius:5px; overflow:hidden; }
.bar-row .tr i { display:block; height:100%; background:linear-gradient(90deg,#0ea5e9,var(--accent)); border-radius:5px; transition:width .6s ease; }
.bar-row .ct { width:60px; font-size:12px; color:var(--ink-mute); font-variant-numeric:tabular-nums; }
table { width:100%; border-collapse:collapse; font-size:12.5px; }
th { text-align:left; color:var(--ink-mute); font-weight:500; padding:6px 10px; border-bottom:1px solid var(--hairline); font-size:11.5px; letter-spacing:.05em; }
td { padding:7px 10px; border-bottom:1px solid rgba(255,255,255,.045); color:var(--ink-dim); font-variant-numeric:tabular-nums; }
tr:hover td { background:rgba(255,255,255,.02); }
.dot { width:7px; height:7px; border-radius:50%; background:var(--ok); display:inline-block; margin-right:8px; box-shadow:0 0 6px rgba(52,211,153,.6); }
.muted { color:var(--ink-mute); font-size:11.5px; }
.empty { color:var(--ink-mute); font-size:12.5px; padding:8px 0; }
footer { margin-top:34px; display:flex; justify-content:space-between; color:var(--ink-mute); font-size:11.5px; }
</style></head><body><div class="wrap">
<header>
  <img src="/logo.png" alt="Zen Gate">
  <div class="tt"><h1>ZEN—GATE <b>用户总览</b></h1><div class="tag">本地免费模型网关 · 部署实例统计</div></div>
  <div class="live"><span class="dot"></span><span id="live">实时</span></div>
</header>
<div class="sub">数据来自各部署实例的匿名心跳（仅 installId / 版本 / 国家），每 6 小时上报一次，仅存于本服务器。</div>
<div class="cards">
  <div class="card"><div class="ic"><svg viewBox="0 0 24 24"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg></div><div><div class="v" id="c-total">—</div><div class="l">累计安装（去重实例）</div></div></div>
  <div class="card"><div class="ic"><svg viewBox="0 0 24 24"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg></div><div><div class="v" id="c-7d">—</div><div class="l">7 日活跃实例</div></div></div>
  <div class="card"><div class="ic"><svg viewBox="0 0 24 24"><rect x="3" y="4" width="18" height="18" rx="2"/><line x1="16" y1="2" x2="16" y2="6"/><line x1="8" y1="2" x2="8" y2="6"/><line x1="3" y1="10" x2="21" y2="10"/></svg></div><div><div class="v" id="c-30d">—</div><div class="l">30 日活跃实例</div></div></div>
  <div class="card"><div class="ic"><svg viewBox="0 0 24 24"><path d="M20.59 13.41 12 22l-9-9V3h10l7.59 7.59a2 2 0 0 1 0 2.82z"/><circle cx="7.5" cy="7.5" r="1.5"/></svg></div><div><div class="v" id="c-ver">—</div><div class="l">在用版本数</div></div></div>
</div>
<div class="panel"><h3>版本分布</h3><div id="p-versions"><div class="empty">加载中…</div></div></div>
<div class="panel"><h3>国家 / 地区分布</h3><div id="p-countries"><div class="empty">加载中…</div></div></div>
<div class="panel"><h3>最近心跳</h3><table><thead><tr><th>实例</th><th>版本</th><th>国家 / 地区</th><th>首次上线</th><th>最近心跳</th><th>上报次数</th></tr></thead><tbody id="tb"><tr><td colspan="6" class="empty">加载中…</td></tr></tbody></table></div>
<footer><span>Zen Gate · 心跳间隔 6 小时 · 数据仅存于本服务器</span><span>每 30 秒自动刷新 · 更新于 <span id="gen">—</span></span></footer>
</div>
<script>
const f = n => new Date(n).toLocaleString("zh-CN", {hour12:false});
const ago = n => { const s = Math.max(0,(Date.now()-n)/1000);
  if (s < 90) return "刚刚"; if (s < 3600) return Math.round(s/60)+" 分钟前";
  if (s < 86400) return Math.round(s/3600)+" 小时前"; return Math.round(s/86400)+" 天前"; };
const flag = cc => { cc = (cc||"").toUpperCase(); if (!/^[A-Z]{2}$/.test(cc)) return "";
  return String.fromCodePoint(...[...cc].map(c=>127397+c.charCodeAt(0))); };
const shortId = s => (s||"").slice(0,8);
const bars = (obj, el) => {
  const items = Object.entries(obj||{}).sort((a,b)=>b[1]-a[1]);
  const max = Math.max(1, ...items.map(x=>x[1]));
  document.getElementById(el).innerHTML = items.length
    ? items.map(([k,v])=>'<div class="bar-row"><span class="nm">'+k+'</span><span class="tr"><i style="width:'+Math.round(v/max*100)+'%"></i></span><span class="ct">'+v+'</span></div>').join("")
    : '<div class="empty">暂无数据</div>';
};
const refresh = () => {
  fetch("/api/stats").then(r=>r.json()).then(d=>{
    document.getElementById("c-total").textContent = d.totalInstalls;
    document.getElementById("c-7d").textContent = d.active7d;
    document.getElementById("c-30d").textContent = d.active30d;
    document.getElementById("c-ver").textContent = Object.keys(d.versions||{}).length;
    bars(d.versions, "p-versions");
    bars(d.countries, "p-countries");
    document.getElementById("gen").textContent = f(d.generatedAt);
  });
  fetch("/api/installs").then(r=>r.json()).then(list=>{
    const rows = [...list].sort((a,b)=>b.lastSeen-a.lastSeen).slice(0,50);
    document.getElementById("tb").innerHTML = rows.length ? rows.map(x=>
      '<tr><td><span class="dot"></span>'+shortId(x.installId)+"…"+'</td><td>'+ (x.version||"—") +'</td><td>'
      + flag(x.country) +' '+ (x.country||"—") +'</td><td>'+f(x.firstSeen)+'</td><td>'+f(x.lastSeen)+'（'+ago(x.lastSeen)+'）</td><td>'+x.pings+'</td></tr>').join("")
      : '<tr><td colspan="6" class="empty">还没有实例上线</td></tr>';
  });
};
refresh();
setInterval(refresh, 30000);
</script></body></html>
"""

src = src[:m.start()] + new_block + src[m.end():]
open(p, 'w', encoding='utf-8', newline='\n').write(src)
print('dashboard rewritten')
