// muelsyse 氛围效果（共享模块，勾选页与公网站共用）。
// 忠实移植 muelsyse-bg-v1.html 的 <script>：纸雨 + 玻璃水珠/水痕 + 光泽跟随 + 点击涟漪，并补上类型。
//
// 编译时与各页入口文件（app.ts / site.ts）作为同一个全局脚本程序一起编译，
// 因此这里定义的 must / escapeHtml / ripple / startAmbient 对入口文件可见；
// 运行时本文件对应的 effects.js 需在入口脚本之前加载。

// ---------- 通用 DOM 助手 ----------
function must<T extends Element>(selector: string): T {
  const el = document.querySelector(selector);
  if (!el) throw new Error(`找不到元素: ${selector}`);
  return el as unknown as T;
}

function escapeHtml(s: string): string {
  return s.replace(/[&<>"]/g, (c) => (
    { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c] as string
  ));
}

// ---------- 点击涟漪 ----------
function ripple(x: number, y: number): void {
  const r = document.createElement("div");
  r.className = "ripple";
  r.style.left = x + "px";
  r.style.top = y + "px";
  document.body.appendChild(r);
  setTimeout(() => r.remove(), 760);
}

// ---------- 轻量 markdown → HTML（总结页/单项页共用）----------
// 支持：标题 / 加粗 / 行内代码 / 链接 / 无序列表 / 引用 / 围栏代码块 / GFM 表格 / 段落。
function mdInline(s: string): string {
  return escapeHtml(s)
    .replace(/`([^`]+)`/g, "<code>$1</code>")
    .replace(/\*\*(.+?)\*\*/g, "<strong>$1</strong>")
    .replace(/\[([^\]]+)\]\(([^)\s]+)\)/g, '<a href="$2" target="_blank" rel="noopener">$1</a>');
}

function mdSplitRow(line: string): string[] {
  return line.replace(/^\s*\|/, "").replace(/\|\s*$/, "").split("|").map((c) => c.trim());
}

function renderMarkdown(md: string): string {
  const lines = md.replace(/\r\n/g, "\n").split("\n");
  const out: string[] = [];
  let i = 0;
  let inList = false;
  const closeList = (): void => {
    if (inList) { out.push("</ul>"); inList = false; }
  };
  while (i < lines.length) {
    const t = lines[i].trim();
    if (t.startsWith("```")) {
      closeList();
      const code: string[] = [];
      i++;
      while (i < lines.length && !lines[i].trim().startsWith("```")) { code.push(lines[i]); i++; }
      i++;
      out.push(`<pre class="code"><code>${escapeHtml(code.join("\n"))}</code></pre>`);
      continue;
    }
    if (t.startsWith("|") && i + 1 < lines.length && /^\|?[\s:|-]*-{2,}[\s:|-]*$/.test(lines[i + 1].trim())) {
      closeList();
      const header = mdSplitRow(t);
      i += 2;
      const rows: string[][] = [];
      while (i < lines.length && lines[i].trim().startsWith("|")) { rows.push(mdSplitRow(lines[i].trim())); i++; }
      const th = header.map((c) => `<th>${mdInline(c)}</th>`).join("");
      const trs = rows.map((r) => `<tr>${r.map((c) => `<td>${mdInline(c)}</td>`).join("")}</tr>`).join("");
      out.push(`<table><thead><tr>${th}</tr></thead><tbody>${trs}</tbody></table>`);
      continue;
    }
    if (t === "") { closeList(); i++; continue; }
    const h = /^(#{1,6})\s+(.*)$/.exec(t);
    if (h) { closeList(); const lv = Math.min(h[1].length, 3); out.push(`<h${lv}>${mdInline(h[2])}</h${lv}>`); i++; continue; }
    if (/^[-*+]\s+/.test(t)) {
      if (!inList) { out.push("<ul>"); inList = true; }
      out.push(`<li>${mdInline(t.replace(/^[-*+]\s+/, ""))}</li>`);
      i++;
      continue;
    }
    if (/^>\s?/.test(t)) { closeList(); out.push(`<blockquote>${mdInline(t.replace(/^>\s?/, ""))}</blockquote>`); i++; continue; }
    closeList();
    out.push(`<p>${mdInline(t)}</p>`);
    i++;
  }
  closeList();
  return out.join("");
}

function truncate(s: string, n: number): string {
  const r = [...s];
  return r.length <= n ? s : r.slice(0, n).join("") + "…";
}

function oneLine(s: string): string {
  return s.replace(/\s+/g, " ").trim();
}

// ---------- 氛围主体 ----------
interface Drop { x: number; y: number; len: number; sp: number; a: number; heavy: boolean; }

// startAmbient 启动背景动画：纸雨 + 光泽跟随。缺失的 canvas 会被安全跳过。
// （原玻璃水珠 #glassrain 系统因每帧数百粒子太吃资源、视觉又不明显，已移除。）
function startAmbient(): void {
  const stage = document.getElementById("stage");
  const rainC = document.getElementById("rain") as HTMLCanvasElement | null;
  if (!stage) return;

  let W = window.innerWidth;
  let H = window.innerHeight;
  let lx = W * 0.5, ly = H * 0.32;
  let tnx = 0.5, tny = 0.32, nx = 0.5, ny = 0.32, hasPointer = false;
  const t0 = performance.now();

  window.addEventListener("pointermove", (e: PointerEvent) => {
    hasPointer = true;
    tnx = e.clientX / W; tny = e.clientY / H;
    lx = e.clientX; ly = e.clientY;
  });

  // --- 背景细雨 ---
  const rctx = rainC ? rainC.getContext("2d") : null;
  let air: Drop[] = [];
  function initAir(): void {
    const n = Math.floor(W / 7);
    air = Array.from({ length: n }, (): Drop => ({
      x: Math.random() * W, y: Math.random() * H,
      len: 8 + Math.random() * 16, sp: 3 + Math.random() * 5,
      a: 0.07 + Math.random() * 0.16, heavy: Math.random() < 0.12,
    }));
  }
  function stepAir(): void {
    if (!rctx) return;
    rctx.clearRect(0, 0, W, H);
    rctx.lineCap = "round";
    for (const d of air) {
      rctx.strokeStyle = `rgba(125,160,150,${d.a})`;
      rctx.lineWidth = d.heavy ? 1.3 : 0.9;
      rctx.beginPath();
      rctx.moveTo(d.x, d.y);
      rctx.lineTo(d.x - 0.5, d.y + d.len);
      rctx.stroke();
      d.y += d.sp; d.x -= 0.25;
      if (d.y > H) { d.y = -d.len; d.x = Math.random() * W; }
    }
  }

  // --- 光泽跟随 ---
  function stepLight(now: number): void {
    if (!hasPointer) {
      const t = (now - t0) / 1000;
      tnx = 0.5 + Math.cos(t * 0.33) * 0.34;
      tny = 0.42 + Math.sin(t * 0.26) * 0.24;
      lx = tnx * W; ly = tny * H;
    }
    nx += (tnx - nx) * 0.08;
    ny += (tny - ny) * 0.08;
    stage!.style.setProperty("--gx", (nx * 100).toFixed(1) + "%");
    stage!.style.setProperty("--gy", (ny * 100).toFixed(1) + "%");
    stage!.style.setProperty("--shx", ((nx - 0.5) * 70).toFixed(1) + "px");
  }

  function frame(now: number): void {
    stepAir(); stepLight(now);
    requestAnimationFrame(frame);
  }

  function resize(): void {
    W = window.innerWidth; H = window.innerHeight;
    if (rainC) { rainC.width = W; rainC.height = H; }
    initAir();
  }

  window.addEventListener("resize", resize);
  resize();
  requestAnimationFrame(frame);
}
