# 🏆 FIFA 2026 足球量化分析预测系统

<div align="left">

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://golang.org)
[![Docker Build](https://img.shields.io/badge/Docker-Compose-2496ED?style=for-the-badge&logo=docker&logoColor=white)](https://www.docker.com)
[![SQLite](https://img.shields.io/badge/SQLite-3-003B57?style=for-the-badge&logo=sqlite&logoColor=white)](https://www.sqlite.org)
[![ECharts](https://img.shields.io/badge/ECharts-5-AA00FF?style=for-the-badge&logo=apache-echarts&logoColor=white)](https://echarts.apache.org)

</div>

本项目是一款专为 **2026 世界杯** 打造的足球量化预测、大模型定性偏置修正及自动套利大屏分析系统。系统采用前后端分离架构，并在后台集成了基于赛程时间轴的实时数据演化推进、定性资讯大模型融合与预测精度在线自校准闭环。

---

## 🗺️ 系统数据流与预测校准闭环

为了直观展现系统各算法及数据模块的动态协作逻辑，系统整体闭环架构设计如下：

```mermaid
graph TD
    A[外部数据源: RSS资讯/赔率] -->|并发拉取/强匹配过滤| B[(SQLite: 资讯/赔率库)]
    B -->|定性伤停/战意情报| C[Ollama LLM: Qwen35B/Llama]
    B -->|动态Elo评分/球队特征| D[Dixon-Coles 回归预测]
    C -->|参数定性微调 offsets| D
    D -->|生成归一化联合概率矩阵| E[深度预测报告]
    E -->|10,000次推演| F[蒙特卡洛全赛事模拟]
    E -->|期望价值 EV > 0 过滤| G[多臂凯利公式最优资金流]
    
    H[后台 LiveSync 常驻同步] -->|时间衰减泊松推进| I[Live 实时比分 / FT 完赛结算]
    I -->|结算触发复盘| J[Backtest 精度精算]
    J -->|Brier Score 反馈梯度| K[Elo战力修正 & Dixon-Coles 自动纠偏]
    K -->|自适应进化调节| D
```

---

## 🌟 核心功能

> [!IMPORTANT]
> **1. 双变量泊松回归预测 (Dixon-Coles Engine)**
> - 采用经典的 **Dixon-Coles 算法** 计算两队期望进球率（$\lambda_H$, $\lambda_A$）及平局算子（$\rho$）。
> - 精算 6x6 比分概率矩阵，有效消除低分截断误差，输出胜平负无偏概率。

> [!TIP]
> **2. 混合型定性偏置修正 (AI Parameter Refiner)**
> - 自动提取 SQLite 中持久化的实时战术、天气、关键伤停情报。
> - 联动本地 **Ollama 实例**（基于 `qwen3.6:35b-q4` 模型）智能解算定量模型偏置量，融合“定量数学”与“定性推理”。

> [!NOTE]
> **3. 布莱尔分数自适应反馈进化 (Online Parameter Tuning)**
> - 完赛时自动精算 **Brier Score (布莱尔分数)** 校验联合预测误差。
> - 依据布莱尔误差方向反推修正梯度，在线更新 `rhoOffset` 偏置，实现预测精准度随着完赛场次递增的**闭环自我进化**。

> [!WARNING]
> **4. 情报去噪声与物理持久化 (Anti-Hallucination Persistence)**
> - 并发抓取的全球 RSS 情报完全固化至 `news_articles`，剔除任何幻觉杜撰，全部提供能直达原文的**精准文章详情 URL**。
> - 所有的单场深度预测报告及概率矩阵自动归档至 `prediction_reports`。
> 
> [!TIP]
> **5. 多源比分共识同步与动态延迟防封 (Multi-Source Consensus & Adaptive Sync)**
> - **多源共识机制**：并发（Goroutine）抓取百度、LiveScore 与 CCTV 的数据，采取比分最大值合并与已完赛状态（`FT`）高优覆盖。支持对 CCTV 云盾安全挑战的自动检测与优雅降级容错。
> - **动态防封轮询**：有 `Live` 比赛时自动维持 60秒 频率，无比赛时自动降低为 10分钟 低频休眠，彻底消除被封 IP 的隐患。
> - **增量 DOM 零闪烁**：重构 SSE `/api/matches/stream` 即时广播，前端基于 `data-match-id` 原地增量更新比分和霓虹渐变背景，配合 JSON 数据指纹比对校验，实现零白屏晃动与零滚动条重置的极致大屏体验。

## 🔬 系统核心量化算法设计

本系统后台集成了三套经过工业级校验的量化精算与风险控制算法：

### 1. 双变量泊松回归预测算法 (Dixon-Coles Engine)
泊松模型假设进球数服从独立的泊松分布，但低比分下（如 0-0, 1-0, 0-1, 1-1）主客队进球数存在统计相关性。系统通过经典的 **Dixon-Coles 修正** 进行了纠偏：
- **联合概率公式**：
  $$P(X=x, Y=y) = \text{Poisson}(x; \lambda_H) \times \text{Poisson}(y; \lambda_A) \times \tau(x, y)$$
- **Dixon-Coles 修正算子 \(\tau(x, y)\)**：
  $$\tau(x, y) = \begin{cases} 
  1 - \lambda_H \lambda_A \rho & (x=0, y=0) \\ 
  1 + \lambda_A \rho & (x=1, y=0) \\ 
  1 + \lambda_H \rho & (x=0, y=1) \\ 
  1 - \rho & (x=1, y=1) \\ 
  1 & (\text{其他情况}) 
  \end{cases}$$
  其中 \(\rho\) 为平局修正系数。
- **Brier Score 精度自动纠偏**：完赛后计算实际结果与预测矩阵的 Brier Score（布莱尔得分），通过学习率 \(\eta=0.05\) 在线自适应修正偏置 \(\text{rhoOffset}\)，实现预测精准度随着场次递增的**闭环自我进化**。

### 2. 体彩单场对冲量化投注算法 (Hedged Betting Criterion)
针对体彩胜平负单场玩法，系统在输出高概率主推的同时，配置了时序对冲模型以最大程度锁定本金安全：
- **资金分配**：采用经典的 **80-20 资金配比**。将 \(80\%\) 的本金投入到通过 Elo 实力和 Dixon-Coles 复合概率计算得出的**主推项**（如主胜/客胜/平局）。
- **波胆时序对冲**：将剩余 \(20\%\) 的资金分配到**平局对冲项**（通常为 1-1 波胆赔率 \(O_{crs}\)）。在主力推荐不中的情况下，通过高奖金的波胆对冲锁定下限保本，最大化降低潜在回撤风险。
- **多维度风险过滤**：当单项最高胜率 \(P_{max} < 38\%\) 或最大最小胜率差 \(\Delta P < 10\%\) 时，或者大模型检测到负面信息（如主力伤缺、内讧、天气极端恶劣）、天敌克制属性时，系统自动进行**风控拦截排除**。

### 3. 智能多场混合过关套利精算算法 (EV Joint Optimization & Multi-Leg Kelly)
针对多场串关（\(M\) 串 \(N\)）玩法，系统构建了多状态空间期望收益（EV）联合优化模型：
- **无偏概率去抽水融合**：利用 **Shin 氏去抽水算法**（Shin's Devigging）反解体彩官方赔率对应的真实无偏概率 \(P_{market}\)，并与泊松模型概率进行动态加权融合：
  $$P_{final} = W_{market} \cdot P_{market} + W_{model} \cdot P_{model}$$
  *(交战历史越充分，模型权重 \(W_{model}\) 越被信任)*。
- **多状态全概率空间推演**：对于包含 \(K\) 场比赛的串关系统，穷举推演其所有的 \(2^K\) 种可能完赛状态状态空间。对任意掩码状态 \(S\)，计算其发生概率 \(P(S)\) 及各个子彩票票单的理论奖金和（考虑官方单注封顶限额），最终计算出复合期望收益率 \(\text{Total EV}\)。
- **多臂凯利公式最优资金管理**：依据组合过关的联合胜率 \(P_{\text{combo}}\) 与 \(\text{Total EV}\) 计算最优配资比例：
  $$\text{KellyStake} = P_{\text{combo}} \times EV_{\text{total}}$$
  并实施严格资金安全垫风控约束（强限制在本金的 \(1\%\) 至 \(5\%\) 之间），实现数学期望收益最大化。

---

## 🛠️ 技术栈 (Technology Stack)

* **后端 (Backend)**：Go (1.22-alpine) 核心服务 / Gin Web 框架 / 跨平台纯 Go 驱动 SQLite。
* **算法模型 (Models)**：Dixon-Coles 回归 / 梯度自校准 / Shin 氏去抽水 / 二次规划多臂凯利公式。
* **大语言模型 (LLM)**：Ollama 容器连通（支持高精度 `qwen3.6:35b-q4` 模型，执行定性偏置修正与赛后量化反思）。
* **前端 (Frontend)**：HTML5 / 原生 CSS3 暗黑霓虹美学 / Vanilla JS (ES6) / ECharts (v5) 可视化。

---

## 📂 项目结构

```bash
├── README.md               # 项目客观描述与自述文件
├── Dockerfile              # 后端服务容器构建配置文件
├── docker-compose.yml      # 本地多服务编排拉起配置
├── data/
│   ├── db/                 # SQLite 数据库持久化目录 (git 排除)
│   └── seasons/            # 冷启动静态分组及球队历史特征配置
├── src/
│   ├── main.go             # Gin 后端路由与服务主入口
│   ├── frontend/           # 霓虹大屏前端展示资源 (html/css/js)
│   └── internal/
│       ├── db/             # SQLite 实体表交互层 (news/predictions/matches)
│       ├── models/         # 核心量化模型实体定义 (tournament/prediction)
│       └── service/        # 核心算法服务 (dixon_coles/live_sync/backtest/scraper)
```

---

## 🚀 快速启动

1. **服务启动**：
   ```bash
   docker compose up -d --build
   ```
2. **访问入口**：
   - 霓虹大屏主页：[http://localhost:20260](http://localhost:20260)
   - 容器将自动通过 `host.docker.internal:11434` 连接宿主机部署的本地大模型。
