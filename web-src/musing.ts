// 随笔阅读页（TypeScript）。musing.html?id=<随笔id>。
// 从 musings.json 按 id 找到该篇，渲染全文（markdown）。返回关于我页（上一级）。
// markdown 渲染 / 助手来自 effects.ts。
// 编译：tsc -p web-src/tsconfig.musing.json → 输出 site/{effects.js,musing.js}。

startAmbient();

interface Musing { id: string; title: string; category: string; date: string; content: string; }

(async () => {
  const titleEl = must<HTMLElement>("#musingTitle");
  const content = must<HTMLElement>("#musingContent");

  const id = new URLSearchParams(location.search).get("id") ?? "";
  if (id === "") {
    titleEl.textContent = "参数错误";
    content.innerHTML = `<div class="glass"><p class="empty-note">缺少 id 参数。</p></div>`;
    return;
  }

  let musings: Musing[] = [];
  try {
    const r = await fetch("./data/musings.json", { cache: "no-store" });
    if (r.ok) musings = (await r.json()) as Musing[];
  } catch {
    /* 忽略：下面按找不到处理 */
  }

  const m = musings.find((x) => x.id === id);
  if (!m) {
    titleEl.textContent = "未找到";
    content.innerHTML = `<div class="glass"><p class="empty-note">找不到这篇随笔（可能尚未发布）。</p></div>`;
    return;
  }

  titleEl.textContent = m.title;
  content.innerHTML = `<div class="glass">
    <div class="ph"><b>${escapeHtml(m.category)}</b><span>${escapeHtml(m.date.replace(/-/g, "."))}</span></div>
    <div class="md-body">${renderMarkdown(m.content)}</div>
  </div>`;
})();
