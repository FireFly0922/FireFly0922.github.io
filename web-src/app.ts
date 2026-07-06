// daily-agent 本地勾选页交互（TypeScript）。
// 氛围效果在 effects.ts；本文件只管业务：采集 → 勾选 → 总结 → 上传。
// 编译：tsc -p web-src/tsconfig.tool.json → 输出 internal/server/web/{effects.js,app.js}（Go 内嵌）。
// 依赖 effects.ts 提供的 must / escapeHtml / ripple / startAmbient（同一全局程序编译）。

startAmbient();

type Source = "git" | "obsidian" | "zotero";

interface Item {
  id: string;
  source: Source;
  title: string;
  detail: string;
  tags?: string[];
}

interface CollectResp { date: string; items: Item[]; }
interface SummarizeResp { report: string; }
interface UploadResp { saved: boolean; push?: { message: string }; warning?: string; }

const SOURCE_LABEL: Record<Source, { zh: string; en: string }> = {
  git: { zh: "git · 今日提交", en: "GIT" },
  obsidian: { zh: "obsidian · 今日笔记", en: "OBSIDIAN" },
  zotero: { zh: "zotero · 今日阅读", en: "ZOTERO" },
};

const els = {
  date: must<HTMLElement>("#date"),
  status: must<HTMLElement>("#status"),
  groups: must<HTMLElement>("#groups"),
  reportBox: must<HTMLElement>("#report-box"),
  report: must<HTMLElement>("#report"),
  collect: must<HTMLButtonElement>("#btn-collect"),
  summarize: must<HTMLButtonElement>("#btn-summarize"),
  upload: must<HTMLButtonElement>("#btn-upload"),
};

const state: { items: Item[]; report: string } = { items: [], report: "" };

function setStatus(msg: string, isErr = false): void {
  els.status.textContent = msg;
  els.status.classList.toggle("err", isErr);
}

async function api<T>(path: string, body?: unknown): Promise<T> {
  const res = await fetch(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body ?? {}),
  });
  const data: unknown = await res.json().catch(() => ({}));
  if (!res.ok) {
    const msg = (data as { error?: string }).error ?? `HTTP ${res.status}`;
    throw new Error(msg);
  }
  return data as T;
}

function selectedIDs(): string[] {
  const boxes = els.groups.querySelectorAll<HTMLInputElement>("input[type=checkbox]:checked");
  return Array.from(boxes, (c) => c.value);
}

function renderItems(): void {
  els.groups.innerHTML = "";
  const order: Source[] = ["git", "obsidian", "zotero"];
  for (const src of order) {
    const items = state.items.filter((it) => it.source === src);
    if (items.length === 0) continue;

    const group = document.createElement("div");
    group.className = "glass group";
    const label = SOURCE_LABEL[src];

    const rows = items.map((it) => {
      const tags = (it.tags ?? []).map((t) => `<span class="chip">#${escapeHtml(t)}</span>`).join("");
      return `<li><label class="rowlab">
        <input type="checkbox" value="${escapeHtml(it.id)}" checked>
        <span>
          <span class="t">${escapeHtml(it.title)}</span>
          <span class="n">${escapeHtml(it.detail)}</span>
          ${tags ? `<span>${tags}</span>` : ""}
        </span>
      </label></li>`;
    }).join("");

    group.innerHTML = `
      <div class="ph"><b>${label.zh}</b><span>${label.en}</span><span class="note">${items.length} 条</span></div>
      <ul class="log">${rows}</ul>`;
    els.groups.appendChild(group);
  }
}

els.collect.addEventListener("click", async () => {
  setStatus("采集中…");
  try {
    const data = await api<CollectResp>("/api/collect");
    state.items = data.items ?? [];
    state.report = "";
    els.date.textContent = data.date;
    els.reportBox.hidden = true;
    renderItems();
    els.summarize.disabled = state.items.length === 0;
    els.upload.disabled = true;
    setStatus(`采集到 ${state.items.length} 条`);
  } catch (e) {
    setStatus((e as Error).message, true);
  }
});

els.summarize.addEventListener("click", async () => {
  const ids = selectedIDs();
  if (ids.length === 0) { setStatus("请先勾选条目", true); return; }
  setStatus("总结中…（调用 LLM）");
  try {
    const data = await api<SummarizeResp>("/api/summarize", { ids });
    state.report = data.report ?? "";
    els.report.textContent = state.report;
    els.reportBox.hidden = false;
    els.upload.disabled = false;
    setStatus("日报已生成，可上传");
  } catch (e) {
    setStatus((e as Error).message, true);
  }
});

els.upload.addEventListener("click", async () => {
  const ids = selectedIDs();
  if (ids.length === 0) { setStatus("请先勾选条目", true); return; }
  setStatus("上传中…");
  try {
    const data = await api<UploadResp>("/api/upload", { ids });
    const msg = data.push?.message ?? "已保存";
    setStatus(data.warning ? `已保存快照，但：${data.warning}` : msg, Boolean(data.warning));
  } catch (e) {
    setStatus((e as Error).message, true);
  }
});

for (const btn of [els.collect, els.summarize, els.upload]) {
  btn.addEventListener("click", (e: MouseEvent) => ripple(e.clientX, e.clientY));
}

// ---------- 标签页切换：每日采集 / 写随笔 ----------
const tabCollect = must<HTMLButtonElement>("#tab-collect");
const tabMusing = must<HTMLButtonElement>("#tab-musing");
const panelCollect = must<HTMLElement>("#panel-collect");
const panelMusing = must<HTMLElement>("#panel-musing");

function showTab(which: "collect" | "musing"): void {
  const isCollect = which === "collect";
  panelCollect.hidden = !isCollect;
  panelMusing.hidden = isCollect;
  tabCollect.classList.toggle("active", isCollect);
  tabMusing.classList.toggle("active", !isCollect);
}
tabCollect.addEventListener("click", () => showTab("collect"));
tabMusing.addEventListener("click", () => showTab("musing"));

// ---------- 写随笔：发布到 /api/musing ----------
interface MusingResp { saved: boolean; id: string; push?: { message: string }; warning?: string }

const mTitle = must<HTMLInputElement>("#m-title");
const mCategory = must<HTMLSelectElement>("#m-category");
const mContent = must<HTMLTextAreaElement>("#m-content");
const mStatus = must<HTMLElement>("#m-status");
const btnPublish = must<HTMLButtonElement>("#btn-publish");

function setMStatus(msg: string, isErr = false): void {
  mStatus.textContent = msg;
  mStatus.classList.toggle("err", isErr);
}

btnPublish.addEventListener("click", async (e: MouseEvent) => {
  ripple(e.clientX, e.clientY);
  const title = mTitle.value.trim();
  const content = mContent.value.trim();
  if (title === "" || content === "") { setMStatus("标题与正文不能为空", true); return; }
  setMStatus("发布中…");
  try {
    const data = await api<MusingResp>("/api/musing", {
      title,
      category: mCategory.value,
      content,
    });
    const msg = data.push?.message ?? "已保存";
    setMStatus(data.warning ? `已保存，但：${data.warning}` : `已发布：${msg}`, Boolean(data.warning));
    mTitle.value = "";
    mContent.value = "";
  } catch (err) {
    setMStatus((err as Error).message, true);
  }
});
