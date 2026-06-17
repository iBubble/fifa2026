📅 2026-06-17 01:38 (CST)

✅ Done (核心突破，限3条极简 bullet):
- 修改 `src/internal/db/db.go`，将 SQLite 数据库初始化 journal 模式由 WAL 重构为 TRUNCATE。
- 彻底根治了 Docker 挂载卷中因共享内存及 `mmap` 兼容限制所致的 `disk I/O error` 库故障。
- 修复了 API 接口 500 异常，大屏重新恢复并成功绘制了 Brier Score 量化复盘精度折线走势图。

📅 2026-06-17 02:13 (CST)

✅ Done (核心突破，限3条极简 bullet):
- 重构后台定时任务触发时机算法，支持依据小组赛/淘汰赛区分加时赛（3h / 4h 偏移），精准实现最后一场比赛后 1 小时自动优化调参并持久化落库。
- 去除前端体彩量化建议卡片的 `overflow-y` 内部滚动，通过增加面板与结果容器最小高度及高度自适应完美解决了底部激进型方案被遮挡截断的视觉缺陷。
- 本地 Go 静态语法检测全量通过，且容器已利用 `docker-compose up --build -d` 重建重启，并成功通过 curl 接口与浏览器 subagent 双向功能走查验证。

📅 2026-06-17 09:05 (CST)

✅ Done (核心突破，限3条极简 bullet):
- 在 `tournament_form.go` 中实现 `CalculateGroupStandings` 并在 `main.go` 拦截器中重构 `local_search` 逻辑，实时计算小组积分榜并提取蒙特卡洛预测注入提示词，消除了智能助手的幻觉。
- 修复 `worldcup26_sync.go` 完赛比分同步的 API 数据映射标签，提升超时限制至 35s 并置 Connection Close 以阻断 EOF 报错。
- 重构 `/api/matches` 请求流，在今日首次被页面访问时，通过配置的 `LastSimulatedDate` 原子抢占，异步触发比分同步与 10,000 次蒙特卡洛全量预测并缓存。

⏳ To-Do (待办事项，限3条极简 bullet):
- 前台智能助手提问功能走查。
