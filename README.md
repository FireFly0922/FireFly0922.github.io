# 个人学习仪表盘 · daily-agent

一个跑在本地的 Go 程序：每天采集 **git / Obsidian / Zotero** 的学习痕迹 → 本地页面手动勾选确认 → agent 生成「今天 vs 昨天」对比日报 → push 到站点数据。配套一个 **muelsyse 主题的静态个人网站**（首页 / 学习记录 / 关于 / 联系），挂到公网供他人查看。

> 架构是「两半式」：
> - **本地半**（Go 程序）：读本地文件、持有 API key，负责采集 + 确认 + 总结 + 上传。
> - **网页半**（`site/` 静态站）：纯展示，只读被 push 上去的数据快照，碰不到本地文件、不持 key。

---

## 1. 目录结构

```
personal_web/
  cmd/daily-agent/        # 主程序入口
  internal/
    collect/              # 三源读取器：git / obsidian / zotero（+ stub 假数据）
    agent/                # 手写 tool-use 主循环
    llm/                  # LLM 客户端：mock（假回复）/ anthropic（真实）
    memory/ store/ push/  # 参照物 / 写快照 / commit&push
    server/               # 本地勾选页（net/http），web/ 内嵌前端
  web-src/                # 前端 TypeScript 源码（编译成 JS）
    effects.ts            # muelsyse 氛围效果（两页共用）
    app.ts   → 勾选页逻辑
    site.ts  → 公网站首页/学习记录逻辑
    day.ts   → 学习记录详情页逻辑
    tsconfig*.json        # 三个编译配置（tool / site / day）
  site/                   # ★ 公网静态站（部署目标）
    index.html day.html muelsyse.css muelsyse.png
    effects.js site.js day.js         # 由 tsc 从 web-src 编译产出
    data/index.json  data/days/*.json # 数据快照（上传写入）
    reports/*.md                      # 日报正文
  .env                    # ANTHROPIC_API_KEY（已 gitignore，绝不提交）
  daily-agent.exe         # 交叉编译出的 Windows 程序
```

---

## 2. 环境准备

- **Go**：本机在 WSL2 Ubuntu 里（`/usr/local/go`）。若想在 Windows 原生装：`winget install --id GoLang.Go -e`。
- **国内代理**（关键，否则拉依赖超时）：
  ```bash
  go env -w GOPROXY=https://goproxy.cn,direct
  ```
- **前端工具**：Node + 全局 `tsc`（`npm i -g typescript`）。改前端才需要，不改可跳过。

---

## 3. 启动本地程序

编译并运行（在 WSL 里，`/mnt/d/...` 对应 Windows 的 `D:\...`）：

```bash
cd /mnt/d/personal_web
# 交叉编译出 Windows 原生 exe（推荐，localhost 无 WSL 网络坑）
GOOS=windows GOARCH=amd64 go build -o daily-agent.exe ./cmd/daily-agent
```

在 **PowerShell** 里跑真实模式：

```powershell
cd D:\personal_web
.\daily-agent.exe -collectors real -llm real `
  -vault    "C:\Users\lenovo\Documents\Obsidian Vault" `
  -zotero   "C:\Users\lenovo\Zotero\zotero.sqlite" `
  -git-repos "D:\你的代码仓库A,D:\你的代码仓库B"
```

打开：
- **勾选页（工具）** → http://localhost:8765/ ：
  - 「每日采集」标签：点「采集」列出今日痕迹 → 勾选 → 「总结」生成日报 → 「上传」写快照并可 commit。
  - 「写随笔」标签：写乐评 / 书评 / 播客思考 / 随想（支持 Markdown）→ 「发布」，出现在公网「关于我」页。
- **公网站预览** → http://localhost:8765/site/ ：首页 / 学习记录（热力图·仪表盘·时间线）/ 文章 / 关于我；点条目进详情/文章/随笔页看全文。

### 常用命令行参数

| 参数 | 说明 | 默认 |
|---|---|---|
| `-collectors` | `stub` 假数据 / `real` 真实三源 | `stub` |
| `-vault` | Obsidian vault 路径 | 空 |
| `-zotero` | `zotero.sqlite` 路径（只读打开，避锁） | 空 |
| `-git-repos` | 本地 git 仓库路径，逗号分隔（**是本地文件夹，不是 GitHub 网址**） | `.` |
| `-llm` | `mock` 假回复不花钱 / `real` 真实 Anthropic | `mock` |
| `-model` | LLM 模型 | `claude-sonnet-5` |
| `-push` | 上传时是否 `git push`（无远端时保持关） | `false` |
| `-site` | 站点数据目录 | `./site` |
| `-addr` | 监听地址 | `127.0.0.1:8765` |

---

## 4. 修改前端内容（改完要重新编译）

### 4.1 改文案 / 身份（不用编译）
公网站首页的名字、简介、关于我、联系方式都在 **`site/index.html`** 里，直接改文字即可：
- 首页大标题「你的名字」、副标、那段 lead 文案；
- 「关于我」区块的 prose 与 方向/坐标/状态；
- 「联系」区块的 email / GitHub 链接；
- 主视觉图：替换 **`site/muelsyse.png`**（建议 3:4）。

> ⚠️ `site/index.html` 含中文，用编辑器改即可；**不要用 PowerShell `Get-Content -Raw` 读写它**（会把中文写成乱码）。

### 4.2 改样式（不用编译）
所有视觉都在 **`site/muelsyse.css`**（纸格背景、玻璃拟态、热力色阶 `--lv0..--lv4`、详情页样式等）。勾选页样式在 `internal/server/web/style.css`。

### 4.3 改交互逻辑（要编译 TS）
逻辑在 `web-src/*.ts`，改完必须重新编译：

```powershell
cd D:\personal_web
tsc -p web-src\tsconfig.json        # 勾选页  → internal/server/web/{effects.js,app.js}
tsc -p web-src\tsconfig.site.json   # 公网站  → site/{effects.js,site.js}
tsc -p web-src\tsconfig.day.json    # 详情页  → site/{effects.js,day.js}
```

- 勾选页的 JS 被 Go **内嵌**进 exe，改完 `web-src/app.ts` 或 `effects.ts` 后，除了 `tsc` 还要**重新交叉编译 exe**（见 §3）。
- 公网站 / 详情页的 JS 直接放在 `site/`，`tsc` 编译后刷新页面即可，无需重编 exe。

---

## 5. 接入真实 API key

Agent 总结默认用 `-llm mock`（假回复，不花钱、不联网）。要用真实 Claude：

1. 复制模板并填 key（key 从 https://console.anthropic.com 获取）：
   ```powershell
   Copy-Item .env.example .env
   notepad .env
   ```
   `.env` 内容：
   ```
   ANTHROPIC_API_KEY=sk-ant-你的真实key
   ```
   > `.env` 已在 `.gitignore`，**绝不会被提交或 push 到公网**（API key 只存在于本地是安全红线）。

2. 启动时加 `-llm real`（可选 `-model claude-sonnet-5` 或更便宜的 `claude-haiku-4-5`）：
   ```powershell
   .\daily-agent.exe -collectors real -llm real -vault "..." -zotero "..." -git-repos "..."
   ```
   缺 key 时程序会直接报错提示，不会静默失败。

### 5.1 用第三方 Anthropic 兼容网关（如 DeepSeek）

不想用官方 Anthropic，可指向任意 Anthropic 兼容端点。`.env` 改成：
```
ANTHROPIC_BASE_URL=https://api.deepseek.com/anthropic
ANTHROPIC_AUTH_TOKEN=sk-你的token
ANTHROPIC_MODEL=deepseek-v4-pro[1m]
```
程序读取规则：
- `ANTHROPIC_AUTH_TOKEN` 存在 → 走 `Authorization: Bearer`（第三方网关）；否则用 `ANTHROPIC_API_KEY` 走 `x-api-key`（官方）。
- `ANTHROPIC_BASE_URL` 覆盖端点，`ANTHROPIC_MODEL` 覆盖 `-model`。
- 请求路径为 `<base_url>/v1/messages`。

> 注：像 DeepSeek reasoning 这类**带思考（thinking）的模型**，tool-use 多轮时程序会自动把 `thinking` 块原样回传（已内建），无需额外配置。
>
> 你粘贴的 `ANTHROPIC_DEFAULT_*_MODEL` / `CLAUDE_CODE_SUBAGENT_MODEL` / `CLAUDE_CODE_EFFORT_LEVEL` 是 **Claude Code CLI** 自己的环境变量，daily-agent 不读取，放不放都不影响本程序。

---

## 6. 数据与部署

### 数据怎么来
- **每日痕迹**：勾选页「上传」→ 写 `site/data/days/*.json`（含每条全文 `content`）、`site/reports/*.md`、更新 `site/data/index.json`（喂热力图/仪表盘），并聚合笔记到 `site/data/articles.json`（喂「文章」博客）。
- **随笔**：工具页「写随笔」发布 → 写 `site/data/musings.json`（喂「关于我」页）。
- 只有你**勾选/发布**的内容才会进 `site/`、才会公开。
- 加 `-push` 时，上传/发布会顺带 `git commit && git push`，git 历史即长期时间线。

### 6.1 连接 GitHub（让「git 提交」能跳转）
详情页里「git · 提交」条目会自动链到对应仓库的提交页。原理：采集时对每个 `-git-repos` 仓库跑 `git remote get-url origin`，归一化成 `https://github.com/<你>/<仓库>/commit/<hash>`。所以只要**那个代码仓库配了 GitHub 远端**即可：
```bash
cd D:\你的代码仓库
git remote -v                       # 看有没有 origin
git remote add origin https://github.com/你的用户名/仓库名.git   # 没有就加
```
没有远端的仓库，详情页会退化为直接显示提交信息（不影响使用）。

### 6.2 把网站部署到公网（人人可访问）

**只部署 `site/` 这个纯静态目录**；本地 `daily-agent`（持 API key、读本地文件）**永远不部署**。

**第 1 步 · 先把项目推到 GitHub**（一次性）
```bash
cd D:\personal_web
git init
git add .
git commit -m "init personal site"
git branch -M main
git remote add origin https://github.com/你的用户名/仓库名.git
git push -u origin main
```
> `.env`（API key）和 `daily-agent.exe` 已在 `.gitignore`，不会被推上去。首次 push 要登录 GitHub：浏览器授权，或用 Personal Access Token 当密码。

**第 2 步 · 选一条托管路线（二选一）**

**路线 A · Vercel（最省事，推荐）**
1. 打开 https://vercel.com → 用 GitHub 登录 → **Add New → Project** → 选你的仓库。
2. **关键：Root Directory 改成 `site`**；Framework 选 *Other*，Build/Output 命令留空。
3. **Deploy**。几十秒后拿到公网地址 `https://xxx.vercel.app`，任何人都能访问。
4. 以后每次 `git push`，Vercel 自动重新部署。

**路线 B · GitHub Pages（免费）**
1. 仓库 **Settings → Pages → Build and deployment → Source 选 `GitHub Actions`**。
2. 项目已自带 `.github/workflows/pages.yml`（自动把 `site/` 发布到 Pages），push 后自动运行。
3. 几分钟后地址为 `https://<用户名>.github.io/<仓库名>/`。
> ⚠️ GitHub Pages 的「分支+目录」源**只支持根目录或 `/docs`**，**不支持 `/site`**——所以本项目用 Actions 发布 `site/`（已配好），**别**在 Settings 里手选分支+`/site` 目录。

**第 3 步 · 日常更新（部署后全自动）**
1. 本地带 `-push` 跑：`.\daily-agent.exe -push -collectors real -llm real -vault "..." -zotero "..." -git-repos "..."`。
2. 采集→勾选→总结→上传，或「写随笔」发布 → 程序自动 `commit && push`。
3. Vercel / Pages 检测到 push → 自动重建 → 几十秒后公网更新。
> `-push` 生效前提：`D:\personal_web` 是 git 仓库且配了 origin 远端（第 1 步已完成）。

---

## 7. FAQ

- **拉依赖超时 / 被墙**：`go env -w GOPROXY=https://goproxy.cn,direct`（Go 官方 proxy 在国内不可达；Anthropic API 不受影响）。
- **中文变乱码**：编辑 `site/*.html` 用普通编辑器或 UTF-8 工具；避免 PowerShell `Get-Content -Raw` 读改含中文的文件。
- **Zotero 读不到 / 库被锁**：程序用 `?mode=ro&immutable=1` 只读打开，正常不会锁；「今天 0 条」通常是今天没在 Zotero 里动过条目。
- **git 采集为空**：`-git-repos` 要填本地仓库文件夹路径；本项目目录若不是 git 仓库需先 `git init`。
- **截图工具超时**：页面有持续的雨滴动画，自动化截图会等不到静止帧，属正常，不影响使用。
- **启动报 `bind: Only one usage of each socket address`（或 address already in use）**：8765 端口被占用，通常是上一个 daily-agent 没退干净还在后台跑。两种解法：
  ```powershell
  # 解法 A：换个端口
  .\daily-agent.exe -addr 127.0.0.1:8766 -collectors real -llm real -vault "..." -zotero "..." -git-repos "..."

  # 解法 B：找出并杀掉占用 8765 的进程
  Get-NetTCPConnection -LocalPort 8765 -State Listen | ForEach-Object { Get-Process -Id $_.OwningProcess }
  Stop-Process -Id <上面查到的PID> -Force
  ```
  （VS Code 里 `Ctrl+C` 有时不会立刻释放端口，或开了多个终端各起了一个实例。）
- **改了前端 / 重开网页看不到改动**：几乎都是**浏览器缓存**（尤其 VS Code 内置 Simple Browser）。程序已对本地静态文件发 `Cache-Control: no-store`，但仍建议：
  1. **硬刷新**：`Ctrl+F5`（或 `Ctrl+Shift+R`）；
  2. VS Code Simple Browser 顽固时，改用 **Edge/Chrome** 打开 `http://localhost:8765/site/`；
  3. **自检文件是否真更新**：浏览器直接打开 `http://localhost:8765/site/muelsyse.css`，`Ctrl+F` 搜 `:has(a)`——搜到说明文件是新的，纯属渲染缓存，硬刷新即可；搜不到说明前端没重新编译，跑一遍 §4.3 的 `tsc`。

---

## 8. 开发者：跑测试

```bash
cd /mnt/d/personal_web
go test ./...        # 全部单测
go vet ./...
```

---

## 9. 完整运行清单（真实数据 + 真实 API key）

从零到看见效果，按顺序做：

**① 一次性准备（只做一次）**
```powershell
# a. 填 key：复制模板后填入 DeepSeek/Anthropic 配置（见 §5 / §5.1）
Copy-Item .env.example .env ; notepad .env
# b. 设国内代理（WSL 里）
wsl bash -lic "go env -w GOPROXY=https://goproxy.cn,direct"
```

**② 编译（改了代码才需要重跑对应命令）**
```powershell
# 前端（改了 web-src/*.ts 才需要）
tsc -p web-src\tsconfig.json ; tsc -p web-src\tsconfig.site.json ; tsc -p web-src\tsconfig.day.json
# 后端 exe（改了 .go 才需要；注意：exe 运行时被锁，先停掉再编）
wsl bash -lic "cd /mnt/d/personal_web && GOOS=windows GOARCH=amd64 go build -o daily-agent.exe ./cmd/daily-agent"
```

**③ 启动（真实数据 + 真实 key）**
```powershell
.\daily-agent.exe -collectors real -llm real `
  -vault    "C:\Users\lenovo\Documents\Obsidian Vault" `
  -zotero   "C:\Users\lenovo\Zotero\zotero.sqlite" `
  -git-repos "D:\5.1-train,D:\dehaze-jetson,D:\fogalert-system,D:\mcp_learning"
```
看到 `daily-agent 启动：http://127.0.0.1:8765` 即成功。若报端口占用，见 §7 FAQ。

**④ 生成当天数据（工具页）**
浏览器开 `http://localhost:8765/`：点**采集** → 勾选要公开的条目 → 点**总结**（真实调用 DeepSeek，稍等几秒）→ 点**上传**（写入 `site/data`、`site/reports`）。

**⑤ 看公网站效果**
浏览器开 `http://localhost:8765/site/`：首页 / 学习记录（热力图·仪表盘·时间线）；点某条记录 → 详情页看日报全文 + 资料具体内容。**第一次务必 `Ctrl+F5` 硬刷新**清掉旧缓存。

**⑥ （可选）发布到公网**
见 §6.1，把 `site/` 推到 GitHub Pages。
