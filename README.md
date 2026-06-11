# FIFA 2026 足球量化分析预测系统

本项目是一款为 2026 世界杯打造的足球量化预测、大模型偏置修正及套利大屏分析系统。系统采用前后端分离架构，集成了基于比赛时间轴的后台自动数据推进、定性资讯大模型融合与预测精度在线自校准闭环。

## 🌟 核心功能

1. **双变量泊松回归预测 (Dixon-Coles Engine)**
   - 采用 **Dixon-Coles 双变量泊松回归算法**，接收主客队实时战力，推算双方期望进球率（$\lambda_H$, $\lambda_A$）及平局相关算子（$\rho$）。
   - 精确计算 6x6 的比分联合概率矩阵、大小球（Over/Under 2.5）期望及胜平负胜率。

2. **定性情报融合与大模型修正 (AI Parameter Refiner)**
   - 支持本地 **Ollama 实例** 接入，结合具体赛事的定性资讯（如伤停、天气、战意），推理输出参数微调偏置项，实现定量数学模型与定性推理的深度契合。

3. **在线复盘与预测精度自校准 (Brier Score Adaptive Tuning)**
   - 比赛完赛后，后台常驻协程基于真实赛果自动结算胜平负预测的 **Brier Score (布莱尔分数)**。
   - 依据布莱尔分数的偏差方向，在线执行梯度更新律对平局修正系数进行自校准，动态修正后续比赛的 Dixon-Coles 参数初始值。

4. **多平台去抽水过滤与套利扫描 (Arbitrage & Kelly Allocation)**
   - 自动拉取博彩机构实时数据并利用 **Shin 氏算法** 去抽水还原，过滤出期望价值 $EV > 0$ 的投注选项。
   - 并行多场比赛下使用**多臂凯利公式 (Multi-bet Kelly)** 在线二次规划，求解最优资金配置分配向量。

5. **外部资讯与分析报告持久化 (Data Persistence)**
   - 并发抓取权威体育 RSS 资讯源并进行 SQLite 物理持久化，所有资讯配备可直接直达阅读的**精准文章详情 URL**。
   - 所有单场量化预测报告及比分矩阵自动固化入库，保障历史报告随时可调阅回溯。

6. **暗黑霓虹数据大屏 (Interactive Aesthetics)**
   - 采用暗黑玻璃微光霓虹设计。
   - 全局队名前缀支持 **Emoji 国旗**，支持高亮预览比赛荧光大标题与完赛复盘卡片滚屏滑动。

## 🛠️ 技术栈 (Technology Stack)

- **后端开发 (Backend)**：Go (1.22-alpine) 核心服务 / Gin Web 框架 / 纯 Go 无依赖 SQLite 驱动 (`modernc.org/sqlite`)。
- **算法与量化 (Quantitative Models)**：Dixon-Coles 回归 / 梯度下降参数校准 / Shin 氏去抽水 / 二次规划多臂凯利公式。
- **大语言模型 (LLM)**：Ollama 容器网络连接（默认模型为 `qwen3.6:35b-q4`）。
- **前端展示 (Frontend)**：HTML5 / 原生 CSS3 霓虹美学 / Vanilla JS (ES6) 异步通信 / ECharts (v5) 可视化图表。

## 🧩 模块详解 (Module Descriptions)

### 1. 泊松预测与全赛程推演模块 (`prediction/`)
- **Dixon-Coles 进球率演算 (`dixon_coles.go`)**：接收两队当前 Elo，经指数映射回归模型推导期望进球数，生成并归一化比分矩阵。
- **全赛程蒙特卡洛仿真 (`montecarlo.go`)**：后台集成高并行的 Monte Carlo 模拟，支持一键对 2026 世界杯赛程进行 **10,000 次** 闭推演，计算各参赛国的小组出线、晋级八强、四强、决赛及夺冠统计学期望。

### 2. 自动化数据同步模块 (`prediction/live_sync.go`)
- **实时同步引擎**：常驻后台协程，每 10 秒扫描未完赛赛程。
- **Live 状态比分推进**：比赛处于进行状态时，结合时间指数衰减曲线实时自动推进进球发生，大屏 Ticker 实时渲染。
- **自动完赛 FT 锁定**：完赛后自动根据最终概率分布矩阵抽取最符合现实的完赛比分写入 SQLite 并触发复盘。

### 3. 复盘自校准进化模块 (`prediction/backtest.go`, `prediction/elo.go`)
- **Brier 精度校正**：结算时精算布莱尔分数，评估定量模型与 LLM 修正的联合预测误差。
- **参数梯度进化**：计算真实平局状态与预测概率偏差，在线动态推演 `rhoOffset` 修正 Dixon-Coles 平局参数。
- **Elo战力积分**：根据完赛结果计算两队的 Elo 积分转移量，持久化至球队历史记录并即时生效。

### 4. 情报去噪持久化模块 (`news/scraper.go`, `db/news.go`)
- **RSS 信息捕获**：高度并发抓取全球权威体育 RSS 资讯源。
- **足球垂直过滤**：新闻标题与摘要多语种足球关键词语义匹配过滤。
- **实体关联**：存储至 SQLite `news_articles` 表，无最新专属资讯时，基于主客队模糊检索推荐全球真实要闻，配备精准 URL，杜绝幻觉。

## 🚀 快速启动

1. **依赖环境**：Docker 及 Docker Compose。
2. **服务拉起**：
   ```bash
   docker compose up -d --build
   ```
3. **访问入口**：
   - 大屏端：[http://localhost:20260](http://localhost:20260)
   - 容器将自动通过 `host.docker.internal:11434` 连通宿主机上的 Ollama 实例。

## 📂 项目结构

- `src/main.go`：Gin 后端服务主入口，管理核心 API。
- `src/internal/db/`：SQLite 存储管理（含 matches, news_articles, prediction_reports, bets 等）。
- `src/internal/service/`：
  - `prediction/`：Dixon-Coles 算法、Elo 评级、Brier 自适应反馈与 LiveSync 后端同步。
  - `news/`：实时资讯 RSS 抓取服务。
- `src/frontend/`：霓虹大屏前端展示。
