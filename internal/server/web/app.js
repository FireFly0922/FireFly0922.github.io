"use strict";
// daily-agent 本地勾选页交互（TypeScript）。
// 氛围效果在 effects.ts；本文件只管业务：采集 → 勾选 → 总结 → 上传。
// 编译：tsc -p web-src/tsconfig.tool.json → 输出 internal/server/web/{effects.js,app.js}（Go 内嵌）。
// 依赖 effects.ts 提供的 must / escapeHtml / ripple / startAmbient（同一全局程序编译）。
startAmbient();
const SOURCE_LABEL = {
    git: { zh: "git · 今日提交", en: "GIT" },
    obsidian: { zh: "obsidian · 今日笔记", en: "OBSIDIAN" },
    zotero: { zh: "zotero · 今日阅读", en: "ZOTERO" },
};
const els = {
    date: must("#date"),
    status: must("#status"),
    groups: must("#groups"),
    reportBox: must("#report-box"),
    report: must("#report"),
    collect: must("#btn-collect"),
    summarize: must("#btn-summarize"),
    upload: must("#btn-upload"),
};
const state = { items: [], report: "" };
function setStatus(msg, isErr = false) {
    els.status.textContent = msg;
    els.status.classList.toggle("err", isErr);
}
async function api(path, body) {
    const res = await fetch(path, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body ?? {}),
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
        const msg = data.error ?? `HTTP ${res.status}`;
        throw new Error(msg);
    }
    return data;
}
function selectedIDs() {
    const boxes = els.groups.querySelectorAll("input[type=checkbox]:checked");
    return Array.from(boxes, (c) => c.value);
}
function renderItems() {
    els.groups.innerHTML = "";
    const order = ["git", "obsidian", "zotero"];
    for (const src of order) {
        const items = state.items.filter((it) => it.source === src);
        if (items.length === 0)
            continue;
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
        const data = await api("/api/collect");
        state.items = data.items ?? [];
        state.report = "";
        els.date.textContent = data.date;
        els.reportBox.hidden = true;
        renderItems();
        els.summarize.disabled = state.items.length === 0;
        els.upload.disabled = true;
        setStatus(`采集到 ${state.items.length} 条`);
    }
    catch (e) {
        setStatus(e.message, true);
    }
});
els.summarize.addEventListener("click", async () => {
    const ids = selectedIDs();
    if (ids.length === 0) {
        setStatus("请先勾选条目", true);
        return;
    }
    setStatus("总结中…（调用 LLM）");
    try {
        const data = await api("/api/summarize", { ids });
        state.report = data.report ?? "";
        els.report.textContent = state.report;
        els.reportBox.hidden = false;
        els.upload.disabled = false;
        setStatus("日报已生成，可上传");
    }
    catch (e) {
        setStatus(e.message, true);
    }
});
els.upload.addEventListener("click", async () => {
    const ids = selectedIDs();
    if (ids.length === 0) {
        setStatus("请先勾选条目", true);
        return;
    }
    setStatus("上传中…");
    try {
        const data = await api("/api/upload", { ids });
        const msg = data.push?.message ?? "已保存";
        setStatus(data.warning ? `已保存快照，但：${data.warning}` : msg, Boolean(data.warning));
    }
    catch (e) {
        setStatus(e.message, true);
    }
});
for (const btn of [els.collect, els.summarize, els.upload]) {
    btn.addEventListener("click", (e) => ripple(e.clientX, e.clientY));
}
// ---------- 标签页切换：每日采集 / 写随笔 ----------
const tabCollect = must("#tab-collect");
const tabMusing = must("#tab-musing");
const panelCollect = must("#panel-collect");
const panelMusing = must("#panel-musing");
function showTab(which) {
    const isCollect = which === "collect";
    panelCollect.hidden = !isCollect;
    panelMusing.hidden = isCollect;
    tabCollect.classList.toggle("active", isCollect);
    tabMusing.classList.toggle("active", !isCollect);
}
tabCollect.addEventListener("click", () => showTab("collect"));
tabMusing.addEventListener("click", () => showTab("musing"));
const mTitle = must("#m-title");
const mCategory = must("#m-category");
const mContent = must("#m-content");
const mStatus = must("#m-status");
const btnPublish = must("#btn-publish");
function setMStatus(msg, isErr = false) {
    mStatus.textContent = msg;
    mStatus.classList.toggle("err", isErr);
}
btnPublish.addEventListener("click", async (e) => {
    ripple(e.clientX, e.clientY);
    const title = mTitle.value.trim();
    const content = mContent.value.trim();
    if (title === "" || content === "") {
        setMStatus("标题与正文不能为空", true);
        return;
    }
    setMStatus("发布中…");
    try {
        const data = await api("/api/musing", {
            title,
            category: mCategory.value,
            content,
        });
        const msg = data.push?.message ?? "已保存";
        setMStatus(data.warning ? `已保存，但：${data.warning}` : `已发布：${msg}`, Boolean(data.warning));
        mTitle.value = "";
        mContent.value = "";
    }
    catch (err) {
        setMStatus(err.message, true);
    }
});
