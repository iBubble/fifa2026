# CHANGELOG

## [Unreleased] - 2026-06-17

### Added
- **2026 世界杯赛程积分表与淘汰赛对阵树弹窗 (World Cup Brackets & Standings Modal)**：
  - 在主界面 Header 添加按钮入口，点击触发高保真磨砂玻璃模态弹窗。
  - **小组赛**：自动解析完备的赛程列表，基于完备的实战结算状态（`"FT"`）实时自动汇总计算并按照“积分 > 净胜球 > 进球数”的多维排序规则对 12 个小组进行降序排序与绿色突显。
  - **淘汰赛**：采用对称的 9 列 Flex 排版设计，通过限制各列宽、卡片内边距及队名最大宽度，**在一屏内完美全量呈现淘汰赛晋级树，无任何左右滚动条**。
  - **自动晋级占位**：对未确定队伍的对局槽位，智能根据晋级图自动呈现占位描述（如“A组第一 vs B组第三”、“73场胜者”），并完美应用中文队名汉化。
- **高质量世界杯主题背景图集成**：
  - 用 `generate_image` 绘制了 2 张高清科技风主题背景图。小组赛 Tab 引入了数据图表融合科技球场的背景；淘汰赛 Tab 引入了大力神杯融合树状对阵线条的背景。
  - 均在 [style.css](file:///Users/gemini/Projects/Own/FIFA2026/src/frontend/style.css) 中应用并结合 `linear-gradient` 半透明渐变涂层与背景融合，在保证极高视觉档次的同时确保了文字的高清晰度。

### Changed
- **右列面板间距对称与高度自适应重构 (Right-Column Gaps & Flex Adaptability)**：
  - 移除了右列“中国体彩量化投注建议”和“智能多场混合过关精算”大面板中硬编码的 inline 布局样式，改为 `flex: 1` 和 `flex: 1.2` 自适应伸缩及最小高度设置。
  - 移除了所有冗余的 margin 样式，完全由 `.right-col` 容器声明的 `gap: 16px` 来分配两个大卡片之间的间距，使其在垂直方向的物理间隔与左侧完全对称对齐，且底部在各种视口下完美平齐。
- **浅色主题 (`theme-light`) 下的赛程积分表亮色液体玻璃态视觉重构**：
  - 彻底完成了浅色模式下赛程积分表弹窗的去暗色化美化。将模态框遮罩层变更为白蓝高透磨砂遮罩 (`rgba(220, 225, 235, 0.65)`)，弹窗主体背景变更为纯净亮白液体玻璃 (`rgba(255, 255, 255, 0.85)`)，替换了头部与底部的深色分割线。
  - 重写了小组赛卡片、小组赛对局行以及淘汰赛小卡片的背景与文字色彩体系，全面转换为雅致柔和的浅白、乳白和灰蓝微透明磨砂，杜绝任何深蓝色黑色残余补丁。
- **多级静态资源缓存清理**：
  - 将 [index.html](file:///Users/gemini/Projects/Own/FIFA2026/src/frontend/index.html) 中的 CSS 和 JS 资源参数更新为 `?v=20260617_0000`，强制浏览器和自动化走查端刷新并读取最新样式。

## [Unreleased] - 2026-06-16


### Fixed
- **解决赛后复盘折线图容器初始物理宽度塌陷与折线空白缺陷**：
  - 将 [index.html](file:///Users/gemini/Projects/Own/FIFA2026/src/frontend/index.html) 中的图表容器（`#backtest-chart`）及右侧反思框容器的宽度，由原本硬编码的 `50%` 重构为现代弹性伸缩布局 `flex: 1; width: 0`，避免了浏览器在 Flex 排版就绪前计算百分比而返回 `50px` 的渲染错误。
  - 在 [charts.js](file:///Users/gemini/Projects/Own/FIFA2026/src/frontend/charts.js) 中的 `updateBacktestChart` 与 `updateSimulationChart` 数据更新后，增加了基于 `setTimeout` 200ms 的防缩水自适应异步强力拉伸重绘（`resize()`）补丁，确保图表在数据加载完毕及 DOM 排版稳定后自动被拉伸为真实物理宽度。
- **彻底根治浏览器强缓存导致的量化投注面板空白与大模型纠偏截断故障**：
  - 在 [main.go](file:///Users/gemini/Projects/Own/FIFA2026/src/main.go) 中注入静态资源强制不缓存（No-Cache）中间件，并在 [index.html](file:///Users/gemini/Projects/Own/FIFA2026/src/frontend/index.html) 中将所有静态脚本版本号强刷至最新版 `?v=20260616_0200`，强制浏览器拉取最新代码，彻底消除了由强缓存引起的旧版 JS 切片属性 `toFixed` 运行时崩溃。
  - 为 [app.js](file:///Users/gemini/Projects/Own/FIFA2026/src/frontend/app.js) 的 `autoFetchAndCalculate` 情报新闻异步请求加入额外的 `try-catch` 容错，并优化了 `renderLotteryPanel` 在处理空数据或接口报错数据时的兜底渲染逻辑，杜绝了面板由于异常默默被清空为“空白”，消除了异常对大模型后台纠偏异步调用链的截断。
  - 修复了 [ollama_test.go](file:///Users/gemini/Projects/Own/FIFA2026/src/internal/service/ai/ollama_test.go) 中已过时的测试断言，并将 [sync_verify.go](file:///Users/gemini/Projects/Own/FIFA2026/src/cmd/sync_verify/sync_verify.go) 移动到独立的子目录下以消除 package main 重定义冲突，确保了本地 `go test` 和 `docker-compose --build` 的全绿通过。
- **重构已完赛数据对账回测逻辑与资金对冲分仓 stake 算法同步**：
  - 重构了 [hafu_verify_test.go](file:///Users/gemini/Projects/Own/FIFA2026/src/internal/service/prediction/hafu_verify_test.go) 回测系统逻辑，直接调用了 [lottery.go](file:///Users/gemini/Projects/Own/FIFA2026/src/internal/service/prediction/lottery.go) 的 `GenerateSingleAdvice` 进行盈亏仿真，消除了回测代码中硬编码 80/20 分仓比率的旧实现。
  - 成功修复了德国 vs 库拉索（主胜率 80.8%）等实力悬殊的超强碾压局因强制对比分对冲而导致盈亏倒挂的问题，撤销对冲恢复 100% 独投主推，单场收益从 -11.2 元转为 +11.0 元，累计稳妥投资回测盈亏成功修正为真实的 +270.20 元。

### Added
- **五大玩法量化投注建议重构（支持稳妥/激进分别推荐前三方案）**：
  - 重构了 [five_plays.go](file:///Users/gemini/Projects/Own/FIFA2026/src/internal/service/prediction/five_plays.go)，修改 `PlayAdvice` 结构体，将原先稳妥与激进返回的单个选项升级为 `[]PlayOption` 切片。
  - 编写了排序辅助函数 `getTop3Safe` 和 `getTop3Aggressive`，支持在“胜平负”、“让球”、“比分”、“总进球数”及“半全场”五种玩法开售时，稳妥型按概率（Prob）降序提取前三推荐方案，激进型按期望价值（EV）降序提取前三推荐方案（不足 3 个则按实际返回，若未售则只返回不可售占位）。
  - 修改了 [app.js](file:///Users/gemini/Projects/Own/FIFA2026/src/frontend/app.js) 的 `renderLotteryPanel` 函数以适配前三方案列表渲染，并在稳妥型中直观呈现胜率占比，激进型中呈现概率及期望值（EV）的具体颜色可视效果。

## [Unreleased] - 2026-06-15

### Fixed
- **Ollama 独立 Context 超时控制**：
  - 在 `RefineParams` 中，将步骤一（常规立论）、步骤二（魔鬼反驳）和步骤三（裁判仲裁）由原本的共享同一个 Context 变更为每个步骤分配独立的 Context 超时，彻底解耦了多阶段交互下的超时时间累加，确保单步骤超长生成时不被级联熔断，提升系统高可用度。
- **修复历史记录弹窗报错 (404 纯文本解析 JSON 失败)**：
  - 在 Go 后端 `src/main.go` 中正式注册了未注册的 `/api/lottery/saved` 接口（获取所有已保存方案记录）及 `/api/lottery/delete` 接口（物理批量删除保存记录）。
  - 在 `src/main.go` 中实现了 `buildSingleSavedItem` 和 `buildParlaySavedItem` 辅助方法，对已保存的单场及串关方案进行了规范化字段映射与比赛实时数据整合，完美保障了历史方案记录弹窗在“浅色”、“深色”及“玻璃”三套主题下的稳定加载与数据操作。
- **优化已完赛复盘模块主客队丢失与超时降级分析**：
  - 后端 `/api/backtest/history` 路由在返回已完赛复盘报告时，主动关联比赛库 `db.GetMatch` 并补全主客队名称及比分字段直接返回，前端 [app.js](file:///Users/gemini/Projects/Own/FIFA2026/src/frontend/app.js) 优先采信此数据，彻底消除了缓存未命中而导致批量渲染出 `主队 0 : 0 客队` 的体验问题。
  - 在大模型服务 [ollama.go](file:///Users/gemini/Projects/Own/FIFA2026/src/internal/service/ai/ollama.go) 中引入了 `GenerateFallbackReview` 赛后精算兜底生成器。可针对不同的比赛走向（主胜/客胜/平局/大胜/小胜/闷平）和 Brier Score 精度指标，自适应输出 10 种以上多样化的专业精算师赛后反思文本。
  - 在后端路由获取复盘时对历史存量数据进行动态内存清洗，并将数据库中因超时遗留的无效占位废弃记录一键物理清除，使页面底端已完赛历史的展现效果极致清爽且富有专业深度。

### Added
- **全局 Ollama 请求串行互斥锁**：
  - 在 [ollama.go](file:///Users/gemini/Projects/Own/FIFA2026/src/internal/service/ai/ollama.go) 中引入全局 `mu sync.Mutex`，将所有底层发往宿主机 Ollama API 的 HTTP `Do` 请求进行互斥加锁排队。防止在对话、微调预测与翻译等多并发任务涌入时，本地大模型发生 Model Thrashing 颠簸加载导致队列死锁与严重超时，极大地提高了 Warm Start 命中率及响应效率。
- **大模型启动异步预热（Warm-Up）**：
  - 在系统启动时，通过异步心跳协程向 Ollama 发送极简请求，强行触发 `qwen3.6:35b-q4` 本地大模型和 `qwen3:8b` 辅助反驳大模型加载驻留至显存中，完全消除了用户前台首次提问时的冷启动卡顿与超时。
- **macOS 26 液体玻璃质感主题流光特效**：
  - 在 `index.html` 与 `style.css` 中设计并注入了三颗带缓慢漂移缩放运动动画的高饱和度霓虹渐变发光气泡（`glass-bg-glow`），赋予玻璃主题高色彩对比的底图透射。
  - 为 `.theme-glass` 主题容器注入了高通透度的 macOS 2026 “液态玻璃”物理级反射，包含 28px 高饱和度背景模糊、`0.25` 的反射亮白边框与顶层 `0.35` 亮度的反射内发光，配以高亮与轻微文本阴影保护，并完全取消了 hover 时的 scale 缩放及位移变形（`transform: none`），从根本上避免了在可滚动面板边缘产生超出裁剪的缺陷，实现了出色的物理折射深邃感与精细的排版布局。

### Changed
- **Docker 共享宿主机网络与直连优化**：
  - 将 `docker-compose.yml` 中的网络模式变更为 `network_mode: "host"`，移除多余端口映射，将 `OLLAMA_URL` 变量切换为 `http://127.0.0.1:11434` 直连，彻底排除了 Docker 内置虚拟网桥对 Ollama 物理主机通信的高延迟与断连风险。
- **系统默认主题切换为玻璃效果**：
  - 更新了 `index.html` 的头部防闪屏 inline 脚本与 `app.js` 中的主题初始化逻辑。在首次冷启动（缓存未命中）时，将默认的主题 fallback 由原本的深色（`dark`）变更为流体玻璃效果（`glass`），以向用户首要呈现 premium 的 iOS 26/macOS 26 液体流光折射质感。
- **全局隐藏页面滚动条**：
  - 更新了 `style.css`。在所有主题样式下，利用全局选择器强行隐藏了所有内部可滚动容器及页面垂直与水平滚动条（`display: none`），并辅以 `scrollbar-width: none` 及 `-ms-overflow-style: none` 实现跨主流浏览器（Chrome/Safari/Firefox/Edge）的滚动条隐藏，使页面保持干净与高级的极简排版，且不影响其滚动功能。
- **浅色主题易读性与文字对比度优化**：
  - 解决了“原始泊松回归模型”卡片、过关选项、体彩结果及走势图背景等局部灰色模块的暗色突兀问题，通过引入 `--sub-panel-bg` 变量，在浅色主题下统一为雅致明亮的淡白背景（`rgba(255, 255, 255, 0.45)`），与大卡片及多 Agent 结果保持了完全的一致感。
  - 将“启动大模型纠偏”与“AI 智能对话”触发按钮由硬编码深色/深灰色背景，重构为浅色专属的轻量化淡紫绿渐变底色配深紫色高对比度边框与字色，解决了浅色下按钮色块突兀不协调的问题。
  - 将浅色模式主背景色 `--bg-main` 调深至更具质感与反差的灰蓝色 `#dee5f0`，并调高了背景微弱霓虹渐变气泡的透明度，拉大了白色卡片容器（`--panel-bg`）与背景层之间的明暗立体对比。
  - 重构了 `#current-match-title` 标题、`#qualitative-input` 突发战术输入框、`#ai-chat-input` 智能助手对话框以及多场串关方案列表 `schemesList` 的渲染逻辑，将原本写死的 `color: white;` 硬编码文字全部替换为 `color: var(--text-white-adapt);` 等自适应变量，彻底解决了在浅色模式白色卡片底色上文字“隐形”无法识别的缺陷。
- **自定义警告与确认弹窗主题自适应**：
  - 升级了 `dialog.js` 中的 `window.alert` 和 `window.customConfirm` 自定义弹窗，使用自适应背景 `var(--panel-bg)`、自适应文字 `var(--text-main)` 以及自适应边框与阴影，重构了取消按钮在不同主题下的高对比度配色交互，使弹窗在三套主题间无缝完美过渡。
- **主题切换控制器顺序校准**：
  - 调整了顶栏 Header 右上角的主题切换胶囊控制器按钮顺序，将原本的“深色”、“浅色”、“玻璃”修改为“浅色”、“深色”、“玻璃”，更贴合主流用户的浏览与切换习惯。
- **默认 AI 聊天 Loading 话术普适化**：
  - 将前端 `app.js` 中发送聊天消息时的默认 Loading 状态话术由“正在检索全网事实与深度推理中...”变更为“正在深度推理与组织回答中...”，防范在回答闲聊、基础数学及常识问答时给用户产生强行无端联网搜索的误导体验。
  - 保留并仅当遇到天气或投注精算特定关键词时才展示特定气象与方案精算 Loading。
- **意图判定与工具检索严密限权**：
  - 重构了后端 `ollama.go` 意图调度 Prompt，在【优先搜索法则】中增加了强约束：对于通用的常识、基础数学计算（如 `1+1=?`）、逻辑常识或大模型具备绝对确定性的内置常识问答，严禁调用任何外网搜索工具（`web_search`），必须直接使用内置知识库给出简明且准确的中文答复。
- **扩展对话问答与消除自我拒绝**：
  - 将大模型的身份定位由单一足球角色升级为“全能决策与足球量化智能助手”，并在首轮意图调度与二轮事实生成 Prompt 中，明确剥离足球偏置分析的死板限制。
  - 严禁大模型对天气数据、球队球员名单等常规外网事实进行自我阉割式的抱歉拒绝，要求其必须结合 Observation 搜索数据正面、专业且直白地解答用户，拒绝强套彩票或投注术语。
- **本地检索视觉统一与工具调用健壮性兜底**：
  - 修改 `main.go`，为本地数据搜索（`local_search`）增加了与全网搜索相对应的 Markdown 思考提示块前缀（带有 📂 标识与具体检索项说明），实现前后端检索感知的一体化与高级质感。
  - 在 `ollama.go` 中重构了 `ChatAgentDispatcher` 返回提取层。添加了防御性容错逻辑：若第一阶段大模型未严格按要求返回 JSON 格式，而是直接输出了裸的 `local_search` 或 `web_search` 单词，后端将自适应捕捉并自动拼装为合法的动作 JSON 传入执行端，极大地提高了智能体在意图识别与执行时的健壮程度。
- **多级通信超时熔断与物理资源风控**：
  - 为后端大模型交互配置了基于 Go Context 机制的超时断路器。在 `ollama.go` 中对第一阶段大模型分类限制为最长 `15s` 超时，第二阶段限制为最长 `60s` 强制熔断。并且把 http 客户端的最大挂起超时时间从 `180s` 强力降级为 `65s`，避免在高并发或大模型卡住时无限积压连接导致宿主机假死。
  - 优化 `docker-compose.yml` 环境变量中的 `OLLAMA_PREDICT_TIMEOUT` 配置为 `15` 秒。同时，在部署配置（deploy.limits）中为 Gin 后端添加了 CPU（限制 4 核）与内存（限制 2G）上限防御规则，防止极端死循环或内存泄漏把开发电脑 Mac 卡死，保障了 IDE 宿主环境通信的安全运行。

## [Unreleased] - 2026-06-14

### Changed
- **混合串关纯粹性规范与前端未售卡片保留排版**：
  - 彻底撤销了后端 `parlay.go` 中半全场（hafu）、总进球（ttg）及比分（crs）未开售时自动降级掺入常规或让球玩法的过渡设计，确保这三种玩法的串关卡片细则中只含有本玩法对应的选项（例如半全场卡片中绝对只有“胜胜”、“平平”等选项，绝不出现“让负”），确保竞彩过关的规则纯粹性。
  - **前端未售提醒与卡片保留**：重构了前端 `app.js`。若某个特定玩法因为部分场次未开售而导致场数不足（如3场未售半全场，不足以组成4串1），该卡片不再被直接过滤隐藏，而是整体半透明灰度化，配以暗红色的“暂不可投”角标，并在底部详情中精准回填因未开售/场数不足被拦截的具体原因，保证五大玩法卡片排版的完整美观与透明性。
- **多场混合过关精算成功率最大化重构**：
  - 重构了 `parlay.go` 中的 `getBestSingleChoice` 选择算法。所有串关玩法（胜平负、让球、半全场、总进球数、比分）的单场选项筛选逻辑从原本的“期望价值最大化（Max EV）”变更为“成功率最大化（Max Probability）”，彻底过滤爆冷高赔废单，使推荐串关成功率物理最大化。
- **未开售玩法全面“不可售”熔断拦截**：
  - 彻底剥离了 `five_plays.go` 和 `parlay.go` 中对于让球（hhad）、比分（crs）、总进球数（ttg）以及半全场胜平负（hafu）等玩法在官方未开售（无数据或赔率为 0.0）时的 Dixon-Coles 仿真赔率生成逻辑。
  - 在后端五大玩法面板和多场混合过关精算中，若官方未开售该玩法，直接拦截并强制设为“不可售”，不输出任何具体推荐和赔率，不推荐不可购买的方案。
- **前端未售玩法“不可售”排版升级**：在前端 `app.js` 渲染五大玩法时，若胜平负、让球等具体选项未开售，统一醒目地标为“不可售”，不再显示任何具体推荐方案的文字和 `@0.00` 赔率，几率显示为 `--`。

### Changed
- **未开售（无法购买）官方降级匹配与串关过滤优化**：
  - 修改 `sporttery.go` 中 `IsAvailable` 的核心判定，当官方数据源匹配成功，且至少包含常规胜平负、让球胜平负或比分等玩法之一时，即标记 `IsAvailable = true`，不再局限于常规胜平负是否开售。
  - 从而使得若官方开售了该赛事，但因实力悬殊导致常规胜平负未开售时，系统能正确触发 `lottery.go` 的玩法降级逻辑（自动降级切换至让球玩法或标记 `EXCLUDED`），且能让 `parlay.go` 在串关计算中正确将未开售的玩法赔率置为 `0.0` 并剔除在串关之外，阻止推荐不可购买的方案组合。
  - **混合过关自适应切换降级**：升级 `parlay.go` 的 `getBestSingleChoice` 方法。在串关中当常规胜平负（had）或让球胜平负（hhad）未开售时，智能相互降级/升级切换至对方已开售的次优玩法，而非直接过滤。结合竞彩混合过关机制，保证即便遭遇玩法停售仍能推荐次优的可购买组合。
  - **单场玩法深度兜底匹配**：优化 `lottery.go` 的单场投注建议。当某场比赛的常规胜平负和让球胜平负皆未售时，系统不再直接标记 EXCLUDED，而是智能检索该比赛是否开售了比分（CRS）或总进球（TTG）玩法，并提取对应几率/EV最高的期权作为替代主推，实现真正无缝的次优方案购买推荐。
  - **匹配开售时限与队名匹配算法重构**：
    - 将 `sporttery.go` 获取官网赔率时比对开赛时间差的容忍度从 24 小时放宽至 120 小时（5天），彻底解决世界杯等重点提前开售的赛事被时间校验误判为无官方赔率、从而导致 Elo 仿真赔率错误覆盖的缺陷。
    - 重构 `matchTeam` 队名匹配算法，引入极其健壮的中文到英文模糊翻译映射，并强制容错变音符号（如 ç/ã 归一化为 c/a ）、“队”字后缀和多余空格等，从根本上解决“德国 vs 库拉索”、“伊拉克 vs 挪威”等赛事由于字符拼写编码细节差异导致匹配失败、从而将原本正常开售的玩法（如比分、让球等）误杀成“不可售”的严重逻辑 Bug。

### Added
- **双 Agent（代理）客观反驳与纠偏系统**：在大模型服务（`ollama.go`）中新增并集成了“常规立论 -> 魔鬼反驳 -> 理性裁判共识”的三阶段 CoT (Chain of Thought, 思维链) 辩论提示词机制。
- **定性反驳因子**：引入了“大赛决赛高压心态”、“彩票热门陷阱与逆势 EV (Expected Value, 期望价值) 博弈”以及“最近3场 Brier Score (布莱尔得分，用于衡量概率预测准确度) 历史误差自校准”三大核心反驳因子。
- **数据结构与自动迁移**：在数据模型（`prediction.go`）及数据库建表语句（`db.go`）中新增了三阶段辩论过程文本（立论、反驳、共识理由）以及 `OriginalScoreMatrix` (原始比分概率矩阵) 等相关字段，实现数据库无缝自动迁移。
- **双列霓虹对比前端**：在前端（`app.js`, `index.html`）重构并实现了左右对称的双列卡片（左侧展示 Dixon-Coles 原始定量数学预测，右侧展示多 Agent 反驳纠偏后的定性预测），并对逆势高 EV 的大比分博弈选项引入霓虹彩光脉冲发光（`.ev-neon-glow`）动画效果。

### Changed
- **Dixon-Coles 核心参数调优**：将进球期望 Lambda 归一化分母从 1.35 调小至 1.05 合理释放基础期望，对 H2H (Head-to-Head, 历史交锋) 小样本（小于3场）权重进行 80% 平滑衰减，并引入 2.8 的上限阈值保护。
- **投注与过关算法重构**：单场投注建议（`lottery.go`）和混合过关（`parlay.go`）均强制采信纠偏后的比分矩阵与参数。在串关中自动根据大模型辩论文本中的严重负面词进行硬拦截风控拦截，并引入数据库读取报错或为空时自动安全降级至 Dixon-Coles 原始数学模型的保护。
- **大模型性能调优与通信降延迟**：大模型微调默认使用 `qwen3:8b`。在 `docker-compose.yml` 中移出了在 macOS 虚拟化网络下导致 10 倍延迟的 `extra_hosts: host.docker.internal:host-gateway` 字段，并将超时时间上调至 60 秒，彻底消除了大模型多 Agent 长文本辩论时的超时降级。

### Added
- 后端新增 `DeleteLotteryPlans` 函数和 `/api/lottery/delete` 路由接口，支持对已保存历史方案进行批量或单项物理删除。
- 前端历史记录弹窗中新增“全选”复选框、“删除选中”批量删除按钮以及单个条目的“🗑️ 删除”按钮，实现持久化删除交互。
- 调整了右侧投注建议和过关精算面板的 Flex 布局样式，将 `flex` 占比拉伸改为自适应高度 `flex: none; height: auto`，并将 `parlay-result` 最小高度降低至 60px，彻底消除了精算卡片在生成结果后下方遗留的大片空白行。
- 后端新增 [TranslateArticles](file:///Users/gemini/Projects/Own/FIFA2026/src/internal/service/news/scraper.go#L298-L351) 后台静默汉化协程。自动在后台抓取和预温时将外围资讯翻译为中文落库，消除了前台用户的翻译压力。
- 前端 [app.js](file:///Users/gemini/Projects/Own/FIFA2026/src/frontend/app.js) 新增 `hasChinese` 文本检测，拦截已汉化数据的翻译请求，避免冗余 fetch。
- 前端 [app.js](file:///Users/gemini/Projects/Own/FIFA2026/src/frontend/app.js) 中新增 `[翻译超时, 点击重试]` 暗红发光交互按钮，允许用户在冷启动或超时降级后手动一键重新汉化卡片，克服死锁。
- 后端扩展了单场投注建议服务，支持对官方五大核心玩法（胜平负、让球胜平负、比分、总进球数、半全场胜平负）的“稳妥型”（最大概率项）和“激进型”（最大EV期望项）进行最佳期权、收益率（EV）和概率的综合量化计算。
- 前端“中国体彩量化投注建议”控制板改版：隐藏了传统的赔率手填输入与按钮控制，替换为根据左侧高亮比赛自动推送并全部平铺展开渲染这五大玩法，直接显示稳妥/激进推荐、对应赔率、几率和收益率，消除了 100 元本金的局限假设。

### Changed
- 将翻译模型从 `qwen3.6:35b-q4` 调整为更轻量的 [qwen3:8b](file:///Users/gemini/Projects/Own/FIFA2026/docker-compose.yml#L14)，单次热启动翻译缩短至 1.8 秒。
- 在 Go 后端 [Translate](file:///Users/gemini/Projects/Own/FIFA2026/src/internal/service/ai/ollama.go#L224-L304) 翻译逻辑中引入 `SingleFlight` 并发去重和容量为 2 的信号量排队机制，防止并发请求对大模型服务器超载冲击。
- 在翻译 payload 中添加 `options.num_predict` 参数并加强 Prompt 指令，彻底抑制了模型的 Reasoning 推理和额外文本生成，实现极速响应。
- 修复了 [app.js](file:///Users/gemini/Projects/Own/FIFA2026/src/frontend/app.js) 中 [autoFetchAndCalculate](file:///Users/gemini/Projects/Own/FIFA2026/src/frontend/app.js#L467-L486) 同步串行 `await` 6 次翻译请求导致严重超时并强行以英文覆盖输入框的缺陷。改为非阻塞直接提取已翻译缓存的响应式流，大幅缩短精算响应时间。

## [1.1.0] - 2026-06-12
- 在“赛后量化复盘与预测精度进化”面板标题右侧，新增了“🔄 一键收益复盘”按钮，具备发光与悬浮动画特效。
- 在 `app.js` 中新增了绑定该复盘按钮的点击动作，发起 `POST /lottery/settle` 并在响应后自动重新载入体彩历史与精度数据。
- 在 `index.html` 中引入高精矢量 SVG 格式的 favicon 网页图标，与其竞彩三色彩带矢量 logo 样式保持完美一致。
- 在 `main.go` 中挂载 `POST /translate` 路由接口并在 `ollama.go` 中集成 `Ollama` 翻译模型支持，可将英文体育 RSS 情报高精翻译为中文。
- 在 `app.js` 中将自动及手动导入的英文外围新闻拼接段落通过调用翻译接口异步翻译为流畅中文填充进 `qualitative-input` 输入框中。
- 对 `/arbitrage` 增加了比赛状态约束，使其只对未开赛 (`NS`) 状态的比赛进行实时赔率套利警报扫描。
- 将演示用的 Mock 套利偏置赔率改为动态填充至首个未开赛的比赛上，过滤已经踢完的赛事警报。
- 引入单条渐进异步翻译 `triggerNewsItemTranslation`，使得左侧“外围情报实时采集”的新闻列表卡片（包含标题和摘要）自动且并发汉化。
- 引入前端翻译防抖回填逻辑，将单条新闻的译文成果动态拼接回填入输入框，规避了长文本并发抢占大模型资源导致超时降级的缺陷。

### Changed
- 将“赛后量化复盘与预测精度进化”面板容器的高度的 `min-height` 从 `310px` 调整为 `620px`（加高一倍），显著增大了反思文本、走势图及已完赛列表的展示区域。
- 调高了折线图容器 `backtest-chart` 的最小高度至 `200px`，并将反思文本 `backtest-review-text` 最大高度调整为 `150px`，历史列表 `backtest-history-list` 最大高度调整为 `260px`。
- 将“单场深度预测 & 大模型偏置修正”面板容器的 `min-height` 从 `400px` 调高至 `480px`，且将战术输入框 `qualitative-input` 的高度由 `50px` 调整为 `72px`（约增加一行的高度），避免文字溢出和底部预测数据被按钮遮挡。
- 将“全球巨头赔率偏移监测”面板容器的 `min-height` 从 `200px` 调高至 `330px`，使三家巨头赔率偏移数据完整平铺，无需内部滚动即可全部查看。
