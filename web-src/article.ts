// 文章阅读页（TypeScript）。article.html?id=<笔记id>。
// 从聚合的 articles.json 里按 id 找到该篇，渲染全文（markdown）。返回文章列表（上一级）。
// markdown 渲染 / 助手来自 effects.ts。
// 编译：tsc -p web-src/tsconfig.article.json → 输出 site/{effects.js,article.js}。

startAmbient();

interface Article { id: string; title: string; tags?: string[]; date: string; snippet: string; content: string; }

(async () => {
  const titleEl = must<HTMLElement>("#articleTitle");
  const content = must<HTMLElement>("#articleContent");

  const id = new URLSearchParams(location.search).get("id") ?? "";
  if (id === "") {
    titleEl.textContent = "参数错误";
    content.innerHTML = `<div class="glass"><p class="empty-note">缺少 id 参数。</p></div>`;
    return;
  }

  let articles: Article[] = [];
  try {
    const r = await fetch("./data/articles.json", { cache: "no-store" });
    if (r.ok) articles = (await r.json()) as Article[];
  } catch {
    /* 忽略：下面按找不到处理 */
  }

  const a = articles.find((x) => x.id === id);
  if (!a) {
    titleEl.textContent = "未找到";
    content.innerHTML = `<div class="glass"><p class="empty-note">找不到这篇文章（可能尚未上传）。</p></div>`;
    return;
  }

  titleEl.textContent = a.title;
  const tags = (a.tags ?? []).map((t) => `<span class="chip">#${escapeHtml(t)}</span>`).join(" ");
  content.innerHTML = `<div class="glass">
    <div class="ph"><b>笔记</b><span>${escapeHtml(a.date.replace(/-/g, "."))}</span>${tags}</div>
    <div class="md-body">${renderMarkdown(a.content)}</div>
  </div>`;
})();
