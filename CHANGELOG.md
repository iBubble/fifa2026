# CHANGELOG

## [Unreleased] - 2026-06-13

### Added
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
