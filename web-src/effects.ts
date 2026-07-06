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
interface Bead { x: number; y: number; r: number; vy: number; sliding: boolean; grow: number; }
interface Thread { x: number; y: number; len: number; sp: number; w: number; a: number; wob: number; }

// startAmbient 启动全套背景动画。缺失的 canvas 会被安全跳过。
function startAmbient(): void {
  const stage = document.getElementById("stage");
  const rainC = document.getElementById("rain") as HTMLCanvasElement | null;
  const glassC = document.getElementById("glassrain") as HTMLCanvasElement | null;
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

  // --- 玻璃上的水珠与水痕 ---
  const gctx = glassC ? glassC.getContext("2d") : null;
  let beads: Bead[] = [];
  let threads: Thread[] = [];
  function makeBead(top: boolean): Bead {
    return { x: Math.random() * W, y: top ? -8 : Math.random() * H, r: 1.6 + Math.random() * 3.2, vy: 0, sliding: false, grow: Math.random() * 0.005 };
  }
  function makeThread(y: number): Thread {
    return { x: Math.random() * W, y, len: 36 + Math.random() * 150, sp: 1.6 + Math.random() * 3.4, w: 0.7 + Math.random() * 1.3, a: 0.05 + Math.random() * 0.13, wob: Math.random() * 7 };
  }
  function initBeads(): void {
    beads = Array.from({ length: Math.min(210, Math.floor((W * H) / 11000)) }, () => makeBead(false));
  }
  function initThreads(): void {
    threads = Array.from({ length: Math.max(10, Math.floor(W / 28)) }, () => makeThread(Math.random() * H));
  }
  function drawBead(d: Bead): void {
    if (!gctx) return;
    const g = gctx.createRadialGradient(d.x - d.r * 0.3, d.y - d.r * 0.3, d.r * 0.1, d.x, d.y, d.r);
    g.addColorStop(0, "rgba(255,255,255,0.42)");
    g.addColorStop(0.45, "rgba(150,185,172,0.10)");
    g.addColorStop(0.85, "rgba(110,150,142,0.16)");
    g.addColorStop(1, "rgba(255,255,255,0.34)");
    gctx.fillStyle = g;
    gctx.beginPath(); gctx.arc(d.x, d.y, d.r, 0, 7); gctx.fill();
    let dx = lx - d.x, dy = ly - d.y;
    const m = Math.hypot(dx, dy) || 1; dx /= m; dy /= m;
    gctx.fillStyle = "rgba(255,255,255,0.85)";
    gctx.beginPath(); gctx.arc(d.x - dx * d.r * 0.4, d.y - dy * d.r * 0.4, Math.max(0.6, d.r * 0.22), 0, 7); gctx.fill();
  }
  function drawThread(t: Thread): void {
    if (!gctx) return;
    const g = gctx.createLinearGradient(t.x, t.y - t.len, t.x, t.y);
    g.addColorStop(0, "rgba(180,205,198,0)");
    g.addColorStop(1, `rgba(195,218,210,${t.a})`);
    gctx.strokeStyle = g; gctx.lineWidth = t.w; gctx.lineCap = "round";
    gctx.beginPath(); gctx.moveTo(t.x, t.y - t.len);
    gctx.quadraticCurveTo(t.x + Math.sin(t.y * 0.05) * t.wob, t.y - t.len / 2, t.x, t.y); gctx.stroke();
    gctx.fillStyle = `rgba(225,238,233,${Math.min(0.5, t.a + 0.15)})`;
    gctx.beginPath(); gctx.arc(t.x, t.y, t.w * 1.4, 0, 7); gctx.fill();
  }
  function stepGlass(): void {
    if (!gctx) return;
    gctx.clearRect(0, 0, W, H);
    for (const t of threads) {
      drawThread(t);
      t.y += t.sp;
      if (t.y - t.len > H) Object.assign(t, makeThread(-t.len));
    }
    for (const d of beads) {
      if (!d.sliding) {
        d.r += d.grow;
        if (d.r > 5.2 && Math.random() < 0.02) d.sliding = true;
      } else {
        d.vy = Math.min(d.vy + 0.14, 2.4 + d.r * 0.25);
        d.y += d.vy;
        if (Math.random() < 0.3) beads.push({ x: d.x + (Math.random() - 0.5) * 1.5, y: d.y - d.r, r: Math.max(1, d.r * 0.34), vy: 0, sliding: false, grow: 0 });
        if (d.y > H + d.r) Object.assign(d, makeBead(true));
      }
      drawBead(d);
    }
    if (beads.length > 460) beads.splice(0, beads.length - 460);
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
    stepAir(); stepGlass(); stepLight(now);
    requestAnimationFrame(frame);
  }

  function resize(): void {
    W = window.innerWidth; H = window.innerHeight;
    if (rainC) { rainC.width = W; rainC.height = H; }
    if (glassC) { glassC.width = W; glassC.height = H; }
    initAir(); initBeads(); initThreads();
  }

  window.addEventListener("resize", resize);
  resize();
  requestAnimationFrame(frame);
}
