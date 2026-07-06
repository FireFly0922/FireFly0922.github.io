// 每日「总结」页（TypeScript）。day.html?date=YYYY-MM-DD。
// 只作为当天新增项目的入口：展示 agent 日报 + 各条新增项目的链接；
// 点某条 → 进该项独立详情页 item.html（git 提交则直接跳 GitHub）。
// markdown 渲染 / 助手来自 effects.ts（renderMarkdown / must / escapeHtml / truncate / oneLine / startAmbient）。
// 编译：tsc -p web-src/tsconfig.day.json → 输出 site/{effects.js,day.js}。

startAmbient();

interface Item { id: string; source: string; title: string; detail: string; content?: string; url?: string; tags?: string[]; }
interface DayData { date: string; commits: Item[]; notes: Item[]; papers: Item[]; report: string; }

async function loadDay(date: string): Promise<DayData | null> {
  try {
    const r = await fetch(`./data/days/${date}.json`, { cache: "no-store" });
    if (!r.ok) return null;
    return (await r.json()) as DayData;
  } catch {
    return null;
  }
}

// 一条新增项目 → 入口链接。git 有远端则直接跳 GitHub；其余进独立详情页 item.html。
function entryLink(it: Item, date: string): string {
  const snippet = escapeHtml(truncate(oneLine(it.detail || it.content || ""), 64));
  const title = escapeHtml(it.title);
  if (it.url) {
    return `<a class="entry" href="${escapeHtml(it.url)}" target="_blank" rel="noopener">
      <span class="et">${title}<span class="ext">↗ GitHub</span></span>
      <span class="en">${snippet}</span></a>`;
  }
  return `<a class="entry" href="item.html?date=${encodeURIComponent(date)}&id=${encodeURIComponent(it.id)}">
    <span class="et">${title}<span class="arrow">›</span></span>
    <span class="en">${snippet}</span></a>`;
}

function section(title: string, en: string, items: Item[], date: string): string {
  if (items.length === 0) return "";
  return `<div class="glass">
    <div class="ph"><b>${title}</b><span>${en}</span><span class="note">${items.length} 条</span></div>
    <div class="entries">${items.map((it) => entryLink(it, date)).join("")}</div>
  </div>`;
}

(async () => {
  const title = must<HTMLElement>("#dayTitle");
  const content = must<HTMLElement>("#dayContent");

  const date = new URLSearchParams(location.search).get("date") ?? "";
  if (!/^\d{4}-\d{2}-\d{2}$/.test(date)) {
    title.textContent = "参数错误";
    content.innerHTML = `<div class="glass"><p class="empty-note">缺少或非法的 date 参数（应形如 day.html?date=2026-07-05）。</p></div>`;
    return;
  }
  title.innerHTML = `${date.replace(/-/g, ".")} <em>总结</em>`;

  const day = await loadDay(date);
  if (!day) {
    content.innerHTML = `<div class="glass"><p class="empty-note">找不到 ${escapeHtml(date)} 的记录数据。</p></div>`;
    return;
  }

  const parts: string[] = [];
  if (day.report && day.report.trim()) {
    parts.push(`<div class="glass day-report">
      <div class="ph"><b>今日对比日报</b><span>AGENT</span></div>
      <div class="md-body">${renderMarkdown(day.report)}</div>
    </div>`);
  }
  parts.push(section("git · 提交", "GIT", day.commits ?? [], date));
  parts.push(section("obsidian · 笔记", "OBSIDIAN", day.notes ?? [], date));
  parts.push(section("zotero · 文献", "ZOTERO", day.papers ?? [], date));

  const html = parts.filter(Boolean).join("");
  content.innerHTML = html || `<div class="glass"><p class="empty-note">这天没有内容。</p></div>`;
})();
