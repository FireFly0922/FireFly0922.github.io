"use strict";
// 公网静态站交互（TypeScript）。
// 忠实复刻 muelsyse 的视图切换 + 涟漪，并把「学习记录」页的示例数据换成真实后端数据：
//   ./data/index.json   → 热力图 + 仪表盘统计
//   ./data/days/*.json  → 记录时间线（日报标题/摘要 + 三源计数）
// 氛围效果由 effects.ts 提供（startAmbient / must / escapeHtml / ripple）。
// 编译：tsc -p web-src/tsconfig.site.json → 输出 site/{effects.js,site.js}。
startAmbient();
// ---------- 视图切换（首页 / 学习 / 关于 / 联系）----------
const views = Array.from(document.querySelectorAll(".view"));
function go(name) {
    views.forEach((v) => v.classList.toggle("active", v.dataset.view === name));
    const active = document.querySelector(".view.active");
    if (active)
        active.scrollTop = 0;
}
document.querySelectorAll("[data-go]").forEach((a) => {
    a.addEventListener("click", (e) => {
        e.preventDefault();
        const me = e;
        ripple(me.clientX, me.clientY);
        const target = a.dataset.go;
        if (target)
            setTimeout(() => go(target), 90);
    });
});
// 支持从详情页「返回上一级」直接落到对应视图，如 index.html#study 打开即显示学习记录。
{
    const hash = location.hash.replace(/^#/, "");
    if (hash)
        go(hash);
}
function total(c) { return c.commits + c.notes + c.papers; }
function pad2(n) { return n < 10 ? "0" + n : n; }
function isoDate(d) { return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}`; }
// truncate 现由共享的 effects.ts 提供。
async function loadJSON(url) {
    try {
        const r = await fetch(url, { cache: "no-store" });
        if (!r.ok)
            return null;
        return (await r.json());
    }
    catch {
        return null;
    }
}
// ---------- 热力图 + 仪表盘 ----------
const MONTHS = ["1月", "2月", "3月", "4月", "5月", "6月", "7月", "8月", "9月", "10月", "11月", "12月"];
function setNum(id, v) {
    const e = document.getElementById(id);
    if (e)
        e.textContent = String(v);
}
function setStreak(id, v) {
    const e = document.getElementById(id);
    if (e)
        e.innerHTML = `${v}<small>天</small>`;
}
function buildHeatmap(index) {
    const byDate = new Map();
    for (const e of index)
        byDate.set(e.date, e);
    const grid = document.getElementById("heatmap");
    const months = document.getElementById("hmMonths");
    if (!grid || !months)
        return;
    grid.innerHTML = "";
    months.innerHTML = "";
    const weeks = 27;
    const today = new Date();
    today.setHours(0, 0, 0, 0);
    // 让网格结束于本周六（今天所在列），从而保证今天一定被渲染；
    // 起点即结束前 weeks*7-1 天，正好落在周日。
    const end = new Date(today);
    end.setDate(end.getDate() + (6 - end.getDay()));
    const cur = new Date(end);
    cur.setDate(cur.getDate() - (weeks * 7 - 1));
    let activeDays = 0, sum = 0, monthCount = 0, lastM = -1;
    const seq = [];
    for (let w = 0; w < weeks; w++) {
        if (cur.getMonth() !== lastM) {
            const m = document.createElement("span");
            m.className = "hm-m";
            m.style.gridColumn = String(w + 1);
            m.textContent = MONTHS[cur.getMonth()];
            months.appendChild(m);
            lastM = cur.getMonth();
        }
        for (let d = 0; d < 7; d++) {
            const cell = document.createElement("div");
            cell.className = "cell";
            const dt = new Date(cur);
            if (dt <= today) {
                const entry = byDate.get(isoDate(dt));
                const cnt = entry ? total(entry.counts) : 0;
                const lvl = entry ? entry.level : 0;
                cell.classList.add("l" + lvl);
                cell.title = `${isoDate(dt)} · ${cnt ? cnt + " 次" : "无"}`;
                if (cnt > 0) {
                    activeDays++;
                    sum += cnt;
                    if (dt.getMonth() === today.getMonth() && dt.getFullYear() === today.getFullYear())
                        monthCount++;
                }
                seq.push(cnt);
            }
            else {
                cell.classList.add("l0", "future");
            }
            grid.appendChild(cell);
            cur.setDate(cur.getDate() + 1);
        }
    }
    let streak = 0;
    for (let i = seq.length - 1; i >= 0; i--) {
        if (seq[i] > 0)
            streak++;
        else
            break;
    }
    setNum("stDays", activeDays);
    setStreak("stStreak", streak);
    setNum("stMonth", monthCount);
    setNum("stSum", sum);
    const note = document.getElementById("hmNote");
    if (note)
        note.textContent = index.length ? `真实数据 · 累计 ${activeDays} 天` : "暂无数据";
}
// ---------- 记录时间线 ----------
function reportSummary(md) {
    const lines = md.split(/\r?\n/).map((l) => l.trim()).filter((l) => l !== "");
    if (lines.length === 0)
        return { title: "", note: "" };
    const title = lines[0].replace(/^#+\s*/, "").replace(/\*\*/g, "");
    const note = lines.slice(1).join(" ").replace(/[#*>_-]/g, " ").replace(/\s+/g, " ").trim();
    return { title, note: truncate(note, 90) };
}
function logItem(entry, day) {
    const dateDots = entry.date.replace(/-/g, ".");
    const n = total(entry.counts);
    const { title, note } = reportSummary(day?.report ?? "");
    const chips = [];
    if (entry.counts.commits)
        chips.push(`git ${entry.counts.commits}`);
    if (entry.counts.notes)
        chips.push(`obsidian ${entry.counts.notes}`);
    if (entry.counts.papers)
        chips.push(`zotero ${entry.counts.papers}`);
    const chipHtml = chips.map((c) => `<span class="chip">${escapeHtml(c)}</span>`).join(" ");
    // 整条作为链接，点击跳详情页看日报全文 + 上传资料具体内容。
    return `<li><a href="day.html?date=${encodeURIComponent(entry.date)}">
    <div class="d">${escapeHtml(dateDots)}<br>${n} 次</div>
    <div>
      <div class="t">${escapeHtml(title || "学习记录")}<span class="arrow">查看全文 ›</span></div>
      <div class="n">${escapeHtml(note || "（当日无日报正文）")}</div>
      ${chipHtml}
    </div></a></li>`;
}
async function buildLog(index) {
    const log = document.getElementById("log");
    if (!log)
        return;
    const active = index
        .filter((e) => total(e.counts) > 0)
        .sort((a, b) => b.date.localeCompare(a.date))
        .slice(0, 12);
    if (active.length === 0) {
        log.innerHTML = `<li><div class="d">—</div><div><div class="n">还没有记录。去本地勾选页采集并上传第一天吧。</div></div></li>`;
        return;
    }
    const rows = await Promise.all(active.map(async (e) => ({ e, day: await loadJSON(`./data/days/${e.date}.json`) })));
    log.innerHTML = rows.map(({ e, day }) => logItem(e, day)).join("");
}
// ---------- 文章 / 笔记博客 ----------
function articleCard(a) {
    const tags = (a.tags ?? []).map((t) => `<span class="chip">#${escapeHtml(t)}</span>`).join(" ");
    return `<a class="entry" href="article.html?id=${encodeURIComponent(a.id)}">
    <span class="et">${escapeHtml(a.title)}<span class="arrow">›</span></span>
    <span class="meta"><span class="date">${escapeHtml(a.date.replace(/-/g, "."))}</span>${tags}</span>
    <span class="en">${escapeHtml(truncate(oneLine(a.snippet || a.content || ""), 90))}</span></a>`;
}
async function buildBlog() {
    const listEl = document.getElementById("blogList");
    const tagSel = document.getElementById("blogTag");
    const searchEl = document.getElementById("blogSearch");
    if (!listEl || !tagSel || !searchEl)
        return;
    const articles = (await loadJSON("./data/articles.json")) ?? [];
    if (articles.length === 0) {
        listEl.innerHTML = `<p class="empty-note">还没有文章。去本地勾选页上传笔记后，这里会自动汇总。</p>`;
        return;
    }
    // 用 TS 填充标签下拉：全部 + 每个标签
    const allTags = Array.from(new Set(articles.flatMap((a) => a.tags ?? []))).sort();
    const opt = (label, val) => `<option value="${escapeHtml(val)}">${escapeHtml(label)}</option>`;
    tagSel.innerHTML = opt("全部标签", "") + allTags.map((t) => opt("#" + t, t)).join("");
    const renderList = () => {
        const activeTag = tagSel.value;
        const kw = searchEl.value.trim().toLowerCase();
        const shown = articles.filter((a) => {
            if (activeTag && !(a.tags ?? []).includes(activeTag))
                return false;
            if (kw === "")
                return true;
            const hay = (a.title + " " + (a.tags ?? []).join(" ") + " " + a.content).toLowerCase();
            return hay.includes(kw);
        });
        listEl.innerHTML = shown.length
            ? shown.map(articleCard).join("")
            : `<p class="empty-note">没有匹配的文章。</p>`;
    };
    tagSel.addEventListener("change", renderList);
    searchEl.addEventListener("input", renderList);
    renderList();
}
// ---------- 关于我：随笔 / 想法 ----------
function musingCard(m) {
    return `<a class="entry" href="musing.html?id=${encodeURIComponent(m.id)}">
    <span class="et">${escapeHtml(m.title)}<span class="arrow">›</span></span>
    <span class="meta"><span class="date">${escapeHtml(m.date.replace(/-/g, "."))}</span><span class="chip">${escapeHtml(m.category)}</span></span>
    <span class="en">${escapeHtml(truncate(oneLine(m.content), 90))}</span></a>`;
}
async function buildMusings() {
    const listEl = document.getElementById("musingList");
    const catSel = document.getElementById("musingCat");
    if (!listEl || !catSel)
        return;
    const musings = (await loadJSON("./data/musings.json")) ?? [];
    if (musings.length === 0) {
        listEl.innerHTML = `<p class="empty-note">还没有随笔。去本地工具页「写随笔」发布第一篇吧。</p>`;
        catSel.style.display = "none";
        return;
    }
    const cats = Array.from(new Set(musings.map((m) => m.category))).sort();
    const opt = (label, val) => `<option value="${escapeHtml(val)}">${escapeHtml(label)}</option>`;
    catSel.innerHTML = opt("全部分类", "") + cats.map((c) => opt(c, c)).join("");
    const renderList = () => {
        const cat = catSel.value;
        const shown = cat ? musings.filter((m) => m.category === cat) : musings;
        listEl.innerHTML = shown.length ? shown.map(musingCard).join("") : `<p class="empty-note">这个分类还没有内容。</p>`;
    };
    catSel.addEventListener("change", renderList);
    renderList();
}
// ---------- 启动：读数据 → 渲染 ----------
(async () => {
    const index = (await loadJSON("./data/index.json")) ?? [];
    buildHeatmap(index);
    await buildLog(index);
    await buildBlog();
    await buildMusings();
})();
