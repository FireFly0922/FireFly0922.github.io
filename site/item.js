"use strict";
// 单个新增项目的独立详情页（TypeScript）。item.html?date=YYYY-MM-DD&id=<条目id>。
// 展示该条目的全文（markdown 渲染）；git 提交则给 GitHub 跳转按钮。
// 返回按钮回到当天「总结」页（上一级）。
// markdown 渲染 / 助手来自 effects.ts。
// 编译：tsc -p web-src/tsconfig.item.json → 输出 site/{effects.js,item.js}。
startAmbient();
const SRC_LABEL = {
    git: "git · 提交",
    obsidian: "obsidian · 笔记",
    zotero: "zotero · 文献",
};
async function loadDay(date) {
    try {
        const r = await fetch(`./data/days/${date}.json`, { cache: "no-store" });
        if (!r.ok)
            return null;
        return (await r.json());
    }
    catch {
        return null;
    }
}
function findItem(day, id) {
    for (const it of [...(day.commits ?? []), ...(day.notes ?? []), ...(day.papers ?? [])]) {
        if (it.id === id)
            return it;
    }
    return null;
}
(async () => {
    const title = must("#itemTitle");
    const content = must("#itemContent");
    const back = must("#backLink");
    const p = new URLSearchParams(location.search);
    const date = p.get("date") ?? "";
    const id = p.get("id") ?? "";
    // 返回按钮只回上一级：当天的总结页。
    back.href = `day.html?date=${encodeURIComponent(date)}`;
    if (!/^\d{4}-\d{2}-\d{2}$/.test(date) || id === "") {
        title.textContent = "参数错误";
        content.innerHTML = `<div class="glass"><p class="empty-note">缺少 date 或 id 参数。</p></div>`;
        return;
    }
    const day = await loadDay(date);
    const it = day ? findItem(day, id) : null;
    if (!it) {
        title.textContent = "未找到";
        content.innerHTML = `<div class="glass"><p class="empty-note">在 ${escapeHtml(date)} 的记录里找不到该条目。</p></div>`;
        return;
    }
    title.textContent = it.title;
    const src = SRC_LABEL[it.source] ?? it.source;
    const tags = (it.tags ?? []).map((t) => `<span class="chip">#${escapeHtml(t)}</span>`).join("");
    if (it.url) {
        content.innerHTML = `<div class="glass">
      <div class="ph"><b>${escapeHtml(src)}</b>${tags}</div>
      <a class="gh-link" href="${escapeHtml(it.url)}" target="_blank" rel="noopener">在 GitHub 查看提交 ›</a>
    </div>`;
        return;
    }
    const body = it.content && it.content.trim() ? it.content : it.detail;
    content.innerHTML = `<div class="glass">
    <div class="ph"><b>${escapeHtml(src)}</b>${tags}</div>
    <div class="md-body">${renderMarkdown(body)}</div>
  </div>`;
})();
