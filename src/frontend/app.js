const API_BASE = "/api";
let currentMatchID = "";
let currentPredictions = null; // 保存当前比赛测算出的投注项
let matchesMap = {}; // 保存比赛映射用于复盘历史翻译

let lastNewsFingerprint = "";
let lastOddsFingerprint = "";
let lastHistoryFingerprint = "";

// 1. 初始化赛程列表并默认选中比赛
// 1. 初始化赛程列表并默认选中比赛 (原地 DOM 增量更新防闪烁)
async function loadMatches(skipAutoSelect = false) {
  try {
    const res = await fetch(`${API_BASE}/matches`);
    const matches = await res.json();
    const listDom = document.getElementById("match-list");
    const activeMatchIDBefore = currentMatchID;

    matches.forEach(m => {
      matchesMap[m.id] = m;
      const matchTime = new Date(m.scheduledAt).toLocaleString('zh-CN', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' });

      // 动态设置状态色背景与 90% 透明度渐变，未开赛不设置背景
      let bgStyle = "";
      let statusText = "未开赛";
      let scoreColor = "var(--neon-green)";
      let textColor = "";
      let subTextColorStyle = "color: var(--text-muted);";

      if (m.status === "Live") {
        bgStyle = "background: linear-gradient(to right, rgba(40, 167, 69, 0.9), rgba(40, 167, 69, 0));";
        statusText = "进行中";
        scoreColor = "#00ff88";
        subTextColorStyle = "color: rgba(255, 255, 255, 0.85);";
      } else if (m.status === "FT") {
        bgStyle = "background: linear-gradient(to right, rgba(136, 0, 255, 0.9), rgba(136, 0, 255, 0));";
        statusText = "已完赛";
        scoreColor = "#ffffff";
        textColor = "color: rgba(255,255,255,0.8);";
        subTextColorStyle = "color: rgba(255, 255, 255, 0.85);";
      } else if (m.status === "NS") {
        statusText = "未开赛";
      } else {
        statusText = m.status;
      }

      const cardStyle = `${bgStyle} border: 1px solid var(--panel-border); border-radius: 8px; padding: 10px; cursor: pointer; transition: all 0.2s; ${textColor}`;

      let item = listDom.querySelector(`[data-match-id="${m.id}"]`);
      if (item) {
        // 原地增量更新 DOM 属性，防止整体重绘闪烁
        const scoreSpan = item.querySelector(".match-score-text");
        const statusSpan = item.querySelector(".match-status-text");
        
        const scoreStr = `${m.homeScore} - ${m.awayScore}`;
        const statusStr = `${m.venue} | ${matchTime} | ${statusText}`;

        if (scoreSpan && scoreSpan.innerText !== scoreStr) {
          scoreSpan.innerText = scoreStr;
          scoreSpan.style.color = scoreColor;
        }
        if (statusSpan && statusSpan.innerText !== statusStr) {
          statusSpan.innerText = statusStr;
        }
        
        // 保持卡片的基础背景渐变与文字颜色属性
        item.style.cssText = cardStyle;
        
        // 恢复高亮样式
        if (m.id === activeMatchIDBefore) {
          item.style.borderColor = "var(--neon-green)";
          item.style.boxShadow = "0 0 8px rgba(0, 255, 136, 0.3)";
        }
      } else {
        // 冷启动创建新节点
        item = document.createElement("div");
        item.className = "match-item";
        item.dataset.matchId = m.id;
        item.style.cssText = cardStyle;

        if (skipAutoSelect && m.id === activeMatchIDBefore) {
          item.style.borderColor = "var(--neon-green)";
          item.style.boxShadow = "0 0 8px rgba(0, 255, 136, 0.3)";
        }

        const checkboxHtml = m.status === "FT"
          ? `<div style="width: 15px; height: 15px; display: flex; align-items: center; justify-content: center; color: var(--text-muted); font-size: 11px; flex-shrink: 0;" title="已完赛，不可串关">🔒</div>`
          : `<input type="checkbox" class="match-select-chk" data-match-id="${m.id}" style="cursor: pointer; width: 15px; height: 15px; accent-color: var(--neon-purple); flex-shrink: 0;" onclick="event.stopPropagation(); onMatchCheckChange();">`;

        item.innerHTML = `
          <div style="display: flex; align-items: center; gap: 10px; width: 100%;">
            ${checkboxHtml}
            <div style="flex: 1; min-width: 0;">
              <div style="display: flex; justify-content: space-between; font-weight: 600;">
                <span style="white-space: nowrap; overflow: hidden; text-overflow: ellipsis;">${translateTeamName(m.homeTeam)} vs ${translateTeamName(m.awayTeam)}</span>
                <span class="match-score-text" style="color: ${scoreColor}; flex-shrink: 0; margin-left: 8px;">${m.homeScore} - ${m.awayScore}</span>
              </div>
              <div class="match-status-text" style="display: flex; justify-content: space-between; font-size: 11px; ${subTextColorStyle} margin-top: 4px;">
                <span style="white-space: nowrap; overflow: hidden; text-overflow: ellipsis; padding-right: 4px;">${m.venue}</span>
                <span style="flex-shrink: 0;">${matchTime} | ${statusText}</span>
              </div>
            </div>
          </div>
        `;
        item.onclick = () => selectMatch(m.id, item);
        listDom.appendChild(item);
      }
    });

    // 默认选择比赛策略 (仅在没有选中比赛，且不需要 skipAutoSelect 时进行)
    if (!skipAutoSelect && !currentMatchID && matches && matches.length > 0) {
      let defaultMatch = matches.find(m => m.status === "Live");
      if (!defaultMatch) {
        defaultMatch = matches.find(m => m.status !== "FT" && m.status !== "Live");
      }
      if (!defaultMatch) {
        defaultMatch = matches[0];
      }

      if (defaultMatch) {
        const defaultItem = listDom.querySelector(`[data-match-id="${defaultMatch.id}"]`);
        if (defaultItem) {
          selectMatch(defaultMatch.id, defaultItem);
          // 自动滚动，使当前选中的比赛位于第一位
          const relativeTop = defaultItem.getBoundingClientRect().top - listDom.getBoundingClientRect().top;
          listDom.scrollTop += relativeTop;
        }
      }
    } else if (activeMatchIDBefore) {
      // 确保高亮对象样式稳固
      const previousItem = listDom.querySelector(`[data-match-id="${activeMatchIDBefore}"]`);
      if (previousItem) {
        previousItem.style.borderColor = "var(--neon-green)";
        previousItem.style.boxShadow = "0 0 8px rgba(0, 255, 136, 0.3)";
      }
    }

    // 根据是否有进行中的比赛，动态修改整站更新倒计时的频率 (如有Live比赛，设为60秒，否则600秒)
    const hasLive = matches.some(m => m.status === "Live");
    const newInterval = hasLive ? 60 : 600;
    if (newInterval !== defaultIntervalSeconds) {
      defaultIntervalSeconds = newInterval;
      countdownSeconds = defaultIntervalSeconds;
      updateCountdownDisplay();
    }
  } catch (err) {
    console.error("加载赛程失败:", err);
  }
}

// 2. 选择比赛高亮并自动拉取官方赔率
async function selectMatch(matchID, element) {
  currentMatchID = matchID;
  document.querySelectorAll(".match-item").forEach(el => {
    el.style.borderColor = "var(--panel-border)";
    el.style.boxShadow = "none";
  });
  element.style.borderColor = "var(--neon-green)";
  element.style.boxShadow = "0 0 8px rgba(0, 255, 136, 0.3)";

  const teamSpan = element.querySelector("div span");
  if (teamSpan) {
    document.getElementById("current-match-title").innerHTML = teamSpan.innerHTML;
  }
  
  // 切换比赛时立刻展示 Loading 态，清除上一场的残留数据以防幻觉
  const predDom = document.getElementById("prediction-result");
  const rankBar = document.getElementById("match-h2h-rank-bar");
  if (rankBar) {
    rankBar.style.display = "none";
    rankBar.innerHTML = "";
  }
  if (predDom) {
    predDom.style.display = "block";
    predDom.innerHTML = `
      <div style="background: rgba(136,0,255,0.04); border: 1px dashed var(--neon-purple); padding: 15px; border-radius: 8px; text-align: center; color: var(--neon-purple); font-weight: 600; font-size: 12px;">
        <span style="display:inline-block; width:12px; height:12px; border:2px solid var(--neon-purple); border-top-color:transparent; border-radius:50%; animation: spin 1s linear infinite; margin-right:6px; vertical-align: middle;"></span>
        量化精算模型推演中，请稍候...
      </div>
      <style>@keyframes spin { to { transform: rotate(360deg); } }</style>
    `;
  }
  
  // 异步加载官方竞彩赔率并回填
  await loadOfficialOdds(matchID);
  
  // 切换比赛时重置倒计时并立刻启动全自动量化流水线
  countdownSeconds = 600;
  triggerAutoCalculation();
}

// 3. 触发双变量泊松预测与大模型修正
document.getElementById("predict-btn").onclick = async () => {
  if (!currentMatchID) {
    alert("请先在左侧选择一场比赛！");
    return;
  }
  const info = document.getElementById("qualitative-input").value;
  const useLLM = document.getElementById("use-llm-chk").checked;
  const btn = document.getElementById("predict-btn");
  btn.innerText = "量化管道推演中...";
  btn.disabled = true;

  // 手动触发时展示 Loading 态，并特别标明大模型修正中
  const predDom = document.getElementById("prediction-result");
  if (predDom) {
    predDom.style.display = "block";
    predDom.innerHTML = `
      <div style="background: rgba(136,0,255,0.04); border: 1px dashed var(--neon-purple); padding: 15px; border-radius: 8px; text-align: center; color: var(--neon-purple); font-weight: 600; font-size: 12px;">
        <span style="display:inline-block; width:12px; height:12px; border:2px solid var(--neon-purple); border-top-color:transparent; border-radius:50%; animation: spin 1s linear infinite; margin-right:6px; vertical-align: middle;"></span>
        大模型偏置修正与精算推演中，请稍候...
      </div>
      <style>@keyframes spin { to { transform: rotate(360deg); } }</style>
    `;
  }

  try {
    const res = await fetch(`${API_BASE}/predict`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ matchId: currentMatchID, info, useLLM })
    });
    const report = await res.json();
    currentPredictions = report;
    
    // 渲染预测报告
    renderPredictionResult(report);
    
  } catch (err) {
    alert("量化推演失败: " + err);
  } finally {
    btn.innerText = "触发双变量泊松估值预测";
    btn.disabled = false;
  }
};

// 4. 渲染精确比分与大模型 Markdown 战术报告
function renderPredictionResult(report) {
  const dom = document.getElementById("prediction-result");
  dom.style.display = "block";

  // 渲染历史 H2H 交锋数据与国际量化实力排名横条
  const rankBar = document.getElementById("match-h2h-rank-bar");
  if (rankBar) {
    if (report.homeRank && report.awayRank) {
      let h2hHtml = "";
      if (report.h2h && report.h2h.totalMatches > 0) {
        const matchInfo = matchesMap[report.matchId];
        const homeCn = matchInfo ? translateTeamNameText(matchInfo.homeTeam) : "主队";
        const awayCn = matchInfo ? translateTeamNameText(matchInfo.awayTeam) : "客队";
        h2hHtml = `<span style="color: var(--neon-purple); font-weight: 800; text-shadow: 0 0 8px rgba(136, 0, 255, 0.3);">${report.h2h.homeWins}（${homeCn}胜） ${report.h2h.draws}（平） ${report.h2h.awayWins}（${awayCn}胜）</span>`;
      } else {
        h2hHtml = `<span style="color: var(--text-muted); font-style: italic;">[无历史交锋记录]</span>`;
      }
      rankBar.innerHTML = `
        <span>实力排名: <strong style="color: var(--neon-green);">${report.homeRank}</strong></span>
        ${h2hHtml}
        <span>实力排名: <strong style="color: var(--neon-green);">${report.awayRank}</strong></span>
      `;
      rankBar.style.display = "flex";
    } else {
      rankBar.style.display = "none";
    }
  }
  
  // 找出最可能比分 (概率前三)
  const sortedMatrix = [...report.scoreMatrix].sort((a,b) => b.prob - a.prob).slice(0, 3);
  let html = `
    <div style="background: rgba(0,0,0,0.15); padding: 10px; border-radius: 8px; margin-bottom: 8px;">
      <h4 style="color: var(--neon-green); font-size: 13px; margin-bottom: 4px;">Dixon-Coles 精确比分概率 (前三名):</h4>
      ${sortedMatrix.map(c => `<div>● ${c.homeScore} - ${c.awayScore} (几率: <span style="color: var(--neon-green); font-weight:600;">${(c.prob*100).toFixed(2)}%</span>)</div>`).join("")}
      <div style="margin-top: 6px; display: flex; gap: 15px;">
        <span>大球 (Over 2.5): <strong style="color:var(--neon-green);">${(report.over25Prob*100).toFixed(1)}%</strong></span>
        <span>小球 (Under 2.5): <strong style="color:var(--neon-green);">${(report.under25Prob*100).toFixed(1)}%</strong></span>
      </div>
    </div>
  `;

  if (report.llmRefined) {
    const lambdaHomeDiff = report.refinedParams && report.originalParams ? (report.refinedParams.lambdaHome - report.originalParams.lambdaHome).toFixed(4) : "0.0000";
    const lambdaAwayDiff = report.refinedParams && report.originalParams ? (report.refinedParams.lambdaAway - report.originalParams.lambdaAway).toFixed(4) : "0.0000";
    const rhoDiff = report.refinedParams && report.originalParams ? (report.refinedParams.rho - report.originalParams.rho).toFixed(4) : "0.0000";

    html += `
      <div style="border-left: 2px solid var(--neon-purple); padding-left: 8px; margin-top: 8px;">
        <h4 style="color: var(--neon-purple); font-size: 13px; margin-bottom: 4px;">Ollama 大模型定性偏置报告 (Qwen/Llama)</h4>
        <p style="font-style: italic; line-height: 1.4; font-size: 12px; margin-bottom: 6px;">"${report.tacticsAnalysis}"</p>
        <div style="background: rgba(136,0,255,0.08); border: 1px solid rgba(136,0,255,0.15); padding: 8px; border-radius: 6px;">
          <div style="font-size: 11px;">主队进球期望修正偏置: <strong style="color: var(--neon-purple);">${lambdaHomeDiff}</strong></div>
          <div style="font-size: 11px;">客队进球期望修正偏置: <strong style="color: var(--neon-purple);">${lambdaAwayDiff}</strong></div>
          <div style="font-size: 11px;">平局算子修正偏置: <strong style="color: var(--neon-purple);">${rhoDiff}</strong></div>
        </div>
      </div>
    `;
  }
  dom.innerHTML = html;
}

// 已下线历史账本结算逻辑

// 9. 蒙特卡洛全量赛事模拟触发
document.getElementById("simulate-btn").onclick = async () => {
  const btn = document.getElementById("simulate-btn");
  btn.innerText = "1万次推演并发运算中...";
  btn.disabled = true;
  try {
    const res = await fetch(`${API_BASE}/simulate`, { method: "POST" });
    const results = await res.json();
    updateSimulationChart(results); // 更新 ECharts 渲染
  } catch (err) {
    alert("推演失败: " + err);
  } finally {
    btn.innerText = "运行全世界杯蒙特卡洛仿真";
    btn.disabled = false;
  }
};

// 10. 套利扫描器周期轮询 (每 10 秒)
async function runArbitrageScanner() {
  try {
    const res = await fetch(`${API_BASE}/arbitrage`);
    const opps = await res.json();
    const panel = document.getElementById("arbitrage-panel");
    const dom = document.getElementById("arbitrage-alerts");
    
    if (opps.length > 0) {
      panel.classList.add("alert-glow");
      dom.innerHTML = opps.map(o => `
        <div style="border-bottom: 1px dashed rgba(0, 255, 136, 0.2); padding-bottom: 4px; margin-bottom: 4px;">
          <div style="font-weight:800; color: var(--neon-green);">🎯 检测到套利机会: ROI +${o.roi.toFixed(2)}%</div>
          <div>赛事: ${translateTeamName(o.homeTeam)} vs ${translateTeamName(o.awayTeam)}</div>
          <div>L值: ${o.lValue.toFixed(4)}</div>
          <div style="font-size: 10px; color: var(--text-muted); margin-top: 2px;">
            ${o.legs.map(l => `● ${l.bookmaker} (${l.outcome}): 赔率 ${l.odds.toFixed(2)} -> 投注 $${l.stakeAmt.toFixed(0)}<br>`).join("")}
          </div>
        </div>
      `).join("");
    } else {
      panel.classList.remove("alert-glow");
      dom.innerHTML = `● 暂无套利机会，系统监控中...`;
    }
  } catch (err) {
    console.error("套利扫描出错:", err);
  }
}

// 多臂凯利与账本汇总面板已下线，相关 Slider 绑定已移除

// 初始化冷启动
document.addEventListener("DOMContentLoaded", () => {
  loadMatches();
  renderLotteryPanel(); // 初始化体彩面板并载入实战收益历史
  loadNews(); // 新增真实新闻初始化拉取
  runOddsShiftsTracker(); // 赔率偏移初始化拉取
  loadBacktestHistory(); // 加载复盘精度看板数据
  setInterval(runArbitrageScanner, 10000); // 10秒定时轮询套利警报
  setInterval(runOddsShiftsTracker, 5000); // 5秒定时更新全球赔率偏移
  runArbitrageScanner();
  setupSSE(); // 订阅比分即时推送
  startCountdownTimer(); // 开启全自动倒计时精算流

  // 一键收益复盘按钮绑定
  const settleBtn = document.getElementById("lottery-settle-btn");
  if (settleBtn) {
    settleBtn.onclick = async () => {
      settleBtn.disabled = true;
      const originalText = settleBtn.innerHTML;
      settleBtn.innerHTML = "⏳ 复盘中...";
      try {
        const res = await fetch(`${API_BASE}/lottery/settle`, { method: "POST" });
        const data = await res.json();
        alert(`🎉 赛后复盘完成！共结算了 ${data.settled || 0} 个方案。`);
        // 重新刷新体彩面板 & 量化复盘精度看板
        await renderLotteryPanel();
        await loadBacktestHistory();
      } catch (err) {
        alert("复盘结算失败: " + err);
      } finally {
        settleBtn.disabled = false;
        settleBtn.innerHTML = originalText;
      }
    };
  }

  // 绑定过关方式切换事件，实时刷新子选项
  document.querySelectorAll('input[name="parlay-mode"]').forEach(el => {
    el.addEventListener('change', () => {
      const chks = document.querySelectorAll(".match-select-chk:checked");
      updateParlayOptions(chks.length);
    });
  });
});

// 全自动倒计时精算流
let defaultIntervalSeconds = 600; // 默认10分钟 (600秒)
let countdownSeconds = defaultIntervalSeconds;
let countdownTimer = null;

function startCountdownTimer() {
  if (countdownTimer) clearInterval(countdownTimer);
  countdownTimer = setInterval(() => {
    countdownSeconds--;
    if (countdownSeconds <= 0) {
      triggerAutoCalculation();
    } else {
      updateCountdownDisplay();
    }
  }, 1000);
}

function updateCountdownDisplay() {
  const m = Math.floor(countdownSeconds / 60);
  const s = countdownSeconds % 60;
  document.getElementById("countdown-clock").innerText = `${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`;
}

async function triggerAutoCalculation() {
  const badge = document.getElementById("quant-pipeline-countdown");
  const clock = document.getElementById("countdown-clock");
  clock.innerText = "精算中...";
  badge.style.boxShadow = "0 0 15px var(--neon-green)";
  badge.style.borderColor = "var(--neon-green)";

  // 执行全量活数据拉取与计算
  await loadMatches(true); // 周期性更新比分列表
  await loadNews();
  await runOddsShiftsTracker();
  if (currentMatchID) {
    await autoFetchAndCalculate();
  }
  await loadBacktestHistory();

  // 恢复倒计时
  countdownSeconds = defaultIntervalSeconds;
  badge.style.boxShadow = "none";
  badge.style.borderColor = "rgba(136,0,255,0.25)";
  updateCountdownDisplay();
}

// 自动串联情报并计算
async function autoFetchAndCalculate() {
  if (!currentMatchID) return;

  try {
    // 1. 获取最新情报列表并串联
    const resNews = await fetch(`${API_BASE}/news?matchId=${currentMatchID}`);
    const articles = await resNews.json();
    
    // 挑选前 3 篇真实资讯并合并，作为外围大模型定性偏置的上下文
    const topArticles = articles.slice(0, 3);
    const translatedParts = [];
    for (let art of topArticles) {
      translatedParts.push(`【${art.sourceSite}】${art.title} —— ${art.summary}`);
    }
    const displayIntel = translatedParts.join("\n");

    document.getElementById("qualitative-input").value = displayIntel;
    document.getElementById("use-llm-chk").checked = true;

    // 2. 自动触发双变量泊松估值
    const resPredict = await fetch(`${API_BASE}/predict`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ matchId: currentMatchID, info: displayIntel, useLLM: true })
    });
    const report = await resPredict.json();
    currentPredictions = report;

    // 3. 自动渲染预测结果
    renderPredictionResult(report);

    // 4. 自动触发体彩量化投注单生成
    const oddsH = parseFloat(document.getElementById("lottery-odds-h").value) || 1.95;
    const oddsD = parseFloat(document.getElementById("lottery-odds-d").value) || 3.20;
    const oddsA = parseFloat(document.getElementById("lottery-odds-a").value) || 3.80;

    let matchIDs = [currentMatchID];
    const items = Array.from(document.querySelectorAll(".match-item"));
    for (let item of items) {
      const mid = item.dataset.matchId;
      if (mid && mid !== currentMatchID) {
        matchIDs.push(mid);
        break;
      }
    }

    const resLottery = await fetch(`${API_BASE}/lottery/recommend`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ matchIds: matchIDs, odds: [oddsH, oddsD, oddsA], predictReport: report })
    });
    const data = await resLottery.json();
    lastLotteryData = data;
    await renderLotteryPanel(data);

  } catch (err) {
    console.error("全自动量化流计算失败:", err);
  }
}

// 轮询并更新全球博彩巨头赔率偏移
async function runOddsShiftsTracker() {
  try {
    const res = await fetch(`${API_BASE}/odds/shifts?matchId=${currentMatchID}`);
    const shifts = await res.json();

    // 指纹比对防闪烁
    const newFingerprint = JSON.stringify(shifts);
    if (newFingerprint === lastOddsFingerprint) {
      return;
    }
    lastOddsFingerprint = newFingerprint;

    const listDom = document.getElementById("odds-shifts-list");
    listDom.innerHTML = "";

    shifts.forEach(s => {
      const isDown = s.direction === "DOWN";
      const color = isDown ? "var(--neon-green)" : "#ff4a4a";
      const arrow = isDown ? "↓ 降水" : "↑ 升水";
      const badge = isDown ? "主力大单防范" : "散单资金流出";
      
      const item = document.createElement("div");
      item.style = "background: rgba(255,255,255,0.02); border: 1px solid var(--panel-border); border-radius: 6px; padding: 6px; display: flex; flex-direction: column; gap: 2px;";
      item.innerHTML = `
        <div style="display: flex; justify-content: space-between; font-weight: 600; font-size:10px;">
          <span>${s.bookmaker}</span>
          <span style="color: ${color};">${arrow} (${Math.abs(s.shiftPct).toFixed(1)}%)</span>
        </div>
        <div style="display: flex; justify-content: space-between; font-size: 9px; color: var(--text-muted);">
          <span>${s.outcome} | 初 ${s.initialOdds.toFixed(2)} → 即 ${s.currentOdds.toFixed(2)}</span>
          <span style="color: ${color}; opacity: 0.8;">[${badge}]</span>
        </div>
        <div style="font-size: 8px; margin-top: 1px; text-align: right;">
          <a href="${s.sourceUrl}" target="_blank" style="color: var(--neon-purple); text-decoration: none;">🔗 ${s.bookmaker} 官方数据源 ↗</a>
        </div>
      `;
      listDom.appendChild(item);
    });
  } catch (err) {
    console.error("加载赔率偏移失败:", err);
  }
}

// 载入真实外围情报
async function loadNews() {
  const listDom = document.getElementById("news-list");
  try {
    const res = await fetch(`${API_BASE}/news?matchId=${currentMatchID}`);
    if (!res.ok) {
      throw new Error(`HTTP 异常，状态码: ${res.status}`);
    }
    const articles = await res.json();
    if (!Array.isArray(articles)) {
      throw new Error("返回的数据格式不正确，未包含情报列表");
    }

    // 指纹比对防闪烁
    const newFingerprint = JSON.stringify(articles);
    if (newFingerprint === lastNewsFingerprint) {
      return;
    }
    lastNewsFingerprint = newFingerprint;

    listDom.innerHTML = "";
    if (articles.length === 0) {
      listDom.innerHTML = `<div style="color: var(--text-muted); font-size: 11px; padding: 10px; text-align: center;">暂无实时外围情报</div>`;
      return;
    }

    articles.forEach((art, idx) => {
      const dateStr = new Date(art.time).toLocaleString('zh-CN', { hour: '2-digit', minute: '2-digit' });
      const item = document.createElement("div");
      item.style = "background: rgba(255,255,255,0.02); border: 1px solid var(--panel-border); border-radius: 8px; padding: 8px; display: flex; flex-direction: column; gap: 4px;";
      
      item.innerHTML = `
        <div style="font-weight: 600; line-height: 1.3;">
          <a href="${art.sourceUrl}" target="_blank" style="color: var(--neon-purple); text-decoration: none; font-size:11px;" class="news-title" data-raw="${art.title}">
            🔗 ${art.title}
          </a>
        </div>
        <div style="color: var(--text-muted); font-size: 10px; line-height: 1.4;" class="news-summary" data-raw="${art.summary}">
          ${art.summary}
        </div>
        <div style="display: flex; justify-content: space-between; align-items: center; margin-top: 4px; font-size: 9px; color: var(--text-muted);">
          <span>${art.sourceSite} | ${dateStr}</span>
        </div>
      `;
      listDom.appendChild(item);
    });

    // 渲染新闻完毕后，直接拼装前3条新闻到大模型输入框内
    updateQualitativeInputFromArticles(articles);

    // 默认点亮大模型定性偏置修正开关
    const useLLMChk = document.getElementById("use-llm-chk");
    if (useLLMChk) {
      useLLMChk.checked = true;
    }

  } catch (err) {
    console.error("加载情报新闻失败:", err);
    listDom.innerHTML = `<div style="color: #ff4a4a; font-size: 10px; padding: 10px; text-align: center;">⚠️ 实时情报加载失败 (请检查网络链接)</div>`;
  }
}



// 防抖拼装前 3 篇已翻译的中文情报更新至 qualitative-input 文本框
let updateInputTimeout = null;
function updateQualitativeInputFromArticles(allArticles) {
  if (updateInputTimeout) clearTimeout(updateInputTimeout);
  updateInputTimeout = setTimeout(() => {
    const topArticles = allArticles.slice(0, 3);
    const translatedParts = [];
    for (let art of topArticles) {
      translatedParts.push(`【${art.sourceSite}】${art.title} —— ${art.summary}`);
    }
    const combinedIntel = translatedParts.join("\n");
    const inputDom = document.getElementById("qualitative-input");
    if (inputDom) {
      inputDom.value = combinedIntel;
    }
  }, 100);
}

let lastLotteryData = null;

// 绑定倾向下拉框改变事件，实现无延迟即时本金重算
document.getElementById("lottery-risk-level").onchange = () => {
  if (lastLotteryData) {
    renderLotteryPanel(lastLotteryData);
  }
};

async function renderLotteryPanel(recommendData = null) {
  const resultDom = document.getElementById("lottery-result");
  const riskLevel = document.getElementById("lottery-risk-level").value;
  
  let html = "";
  
  if (recommendData) {
    const s = recommendData.single;
    if (s.status === "EXCLUDED") {
      html += `
        <div style="border-left: 2px solid red; padding-left: 6px; margin-bottom: 8px; color: #ff4a4a;">
          <strong>🚨 单场风控过滤警告</strong><br>
          ${s.reason}
        </div>
      `;
    } else {
      let primaryAmt, hedgeAmt, hedgeText;
      if (riskLevel === "激进") {
        primaryAmt = 100;
        hedgeAmt = 0;
        hedgeText = `<span style="color: var(--text-muted);">已放弃比分防守，100元全仓博取高奖金</span>`;
      } else {
        primaryAmt = 80;
        hedgeAmt = 20;
        hedgeText = `分配 20%: <span style="color: white; font-weight:600;">${s.hedgeBets[0].outcome} @ ${s.hedgeBets[0].odds.toFixed(2)}</span> (投入 <strong style="color:var(--neon-green);">${hedgeAmt}元</strong>)`;
      }

      html += `
        <div style="border-left: 2px solid var(--neon-green); padding-left: 6px; margin-bottom: 8px;">
          <strong style="color: var(--neon-green); font-size:12px;">🎯 竞彩单场优化方案 (总本金: 100元)</strong><br>
          主投 80%: <span style="color: white; font-weight:600;">${s.primaryBet} @ ${s.primaryOdds.toFixed(2)}</span> (投入 <strong style="color:var(--neon-green);">${primaryAmt}元</strong>)<br>
          防守对冲: ${hedgeText}<br>
          <span style="font-size:10px; color: var(--text-muted); display:block; margin-top:4px; line-height: 1.4;">${s.reason}</span>
        </div>
      `;
    }

    if (recommendData.parlay) {
      html += `
        <div style="border-left: 2px solid var(--neon-purple); padding-left: 6px; margin-top: 8px; border-top: 1px solid var(--panel-border); padding-top: 6px; margin-bottom: 8px;">
          <strong style="color: var(--neon-purple); font-size:12px;">🔗 2串1 时序对冲混合过关建议</strong><br>
          <span style="font-size:10px; color: var(--text-muted); line-height: 1.4; display:block; margin-top:2px;">${recommendData.parlay.reason}</span>
        </div>
      `;
    }
  } else {
    html += `<div style="margin-bottom: 12px;">● 请在左侧选择比赛，设置参考赔率后生成策略...</div>`;
  }

  // 追加历史投注复盘明细和收益对比
  try {
    const res = await fetch(`${API_BASE}/lottery/history`);
    const data = await res.json();
    if (data && data.history && data.history.length > 0) {
      const sum = data.summary;
      
      const safeColor = sum.totalSafeProfit >= 0 ? "var(--neon-green)" : "#ff4a4a";
      const aggColor = sum.totalAggProfit >= 0 ? "var(--neon-green)" : "#ff4a4a";

      html += `
        <div style="border-top: 1px dashed rgba(255,255,255,0.1); margin: 12px 0 8px 0; padding-top: 10px;">
          <h4 style="color: white; font-size: 13px; font-weight: 800; margin-bottom: 6px; display: flex; align-items: center; gap: 4px;">
            📊 体彩实战收益历史复盘
          </h4>
          
          <div style="background: rgba(255,255,255,0.02); border: 1px solid var(--panel-border); border-radius: 6px; padding: 6px; margin-bottom: 8px; font-size: 11px;">
            <div style="display: flex; justify-content: space-between; margin-bottom: 2px;">
              <span>🛡️ 稳妥型累计净收益:</span>
              <strong style="color: ${safeColor};">${sum.totalSafeProfit.toFixed(1)}元 (ROI: ${sum.safeRoi}%)</strong>
            </div>
            <div style="display: flex; justify-content: space-between;">
              <span>⚡ 激进型累计净收益:</span>
              <strong style="color: ${aggColor};">${sum.totalAggProfit.toFixed(1)}元 (ROI: ${sum.aggRoi}%)</strong>
            </div>
          </div>

          <div style="font-size: 10px; color: var(--text-muted); margin-bottom: 4px; font-weight: 600;">历史投注明细 (滚动查看):</div>
          <div style="max-height: 120px; overflow-y: auto; display: flex; flex-direction: column; gap: 6px; padding-right: 2px;">
            ${data.history.map(h => {
              const homeCn = translateTeamName(h.homeTeam);
              const awayCn = translateTeamName(h.awayTeam);
              
              const safeText = h.safeProfit >= 0 ? `+${h.safeProfit.toFixed(1)}` : h.safeProfit.toFixed(1);
              const aggText = h.aggProfit >= 0 ? `+${h.aggProfit.toFixed(1)}` : h.aggProfit.toFixed(1);
              
              const sColor = h.safeProfit >= 0 ? "var(--neon-green)" : "#ff4a4a";
              const aColor = h.aggProfit >= 0 ? "var(--neon-green)" : "#ff4a4a";

              const primaryBadge = h.primaryHit ? "🎯 主推中" : "❌ 主推失";
              const hedgeBadge = h.hedgeHit ? "🛡️ 对冲中" : "❌ 对冲失";

              return `
                <div style="background: rgba(255,255,255,0.01); border: 1px solid rgba(255,255,255,0.04); border-radius: 4px; padding: 6px; font-size: 10px;">
                  <div style="display: flex; justify-content: space-between; font-weight: 600; color: #fff; margin-bottom: 2px;">
                    <span>${homeCn} ${h.homeScore} : ${h.awayScore} ${awayCn}</span>
                    <span style="font-size: 9px; color: var(--text-muted);">${primaryBadge} | ${hedgeBadge}</span>
                  </div>
                  <div style="display: flex; justify-content: space-between; color: var(--text-muted); font-size: 9px;">
                    <span>稳妥型: <strong style="color: ${sColor};">${safeText}元</strong> (返 ${h.safeReturn}元)</span>
                    <span>激进型: <strong style="color: ${aColor};">${aggText}元</strong> (返 ${h.aggReturn}元)</span>
                  </div>
                </div>
              `;
            }).join("")}
          </div>
        </div>
      `;
    }
  } catch (err) {
    console.error("加载体彩历史复盘失败:", err);
  }

  resultDom.innerHTML = html;
}

// 绑定体彩量化建议按钮事件
document.getElementById("lottery-btn").onclick = async () => {
  if (!currentMatchID) {
    alert("请先在左侧选择一场比赛！");
    return;
  }
  const oddsH = parseFloat(document.getElementById("lottery-odds-h").value) || 1.95;
  const oddsD = parseFloat(document.getElementById("lottery-odds-d").value) || 3.20;
  const oddsA = parseFloat(document.getElementById("lottery-odds-a").value) || 3.80;

  let matchIDs = [currentMatchID];
  const items = Array.from(document.querySelectorAll(".match-item"));
  for (let item of items) {
    const mid = item.dataset.matchId;
    if (mid && mid !== currentMatchID) {
      matchIDs.push(mid);
      break;
    }
  }

  const resultDom = document.getElementById("lottery-result");
  resultDom.innerText = "体彩量化测算中...";

  try {
    const res = await fetch(`${API_BASE}/lottery/recommend`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ 
        matchIds: matchIDs, 
        odds: [oddsH, oddsD, oddsA],
        predictReport: currentPredictions
      })
    });
    const data = await res.json();
    lastLotteryData = data;
    await renderLotteryPanel(data);
  } catch (err) {
    resultDom.innerText = "体彩量化测算失败: " + err;
  }
};

// 12. 自动拉取中国体彩官方赔率并高亮标记回填
async function loadOfficialOdds(matchID) {
  try {
    const res = await fetch(`${API_BASE}/lottery/official?matchId=${matchID}`);
    const odds = await res.json();
    const inputH = document.getElementById("lottery-odds-h");
    const inputD = document.getElementById("lottery-odds-d");
    const inputA = document.getElementById("lottery-odds-a");
    const headerTitle = document.querySelector(".right-col h3");

    if (odds.isAvailable) {
      inputH.value = odds.homeOdds.toFixed(2);
      inputD.value = odds.drawOdds.toFixed(2);
      inputA.value = odds.awayOdds.toFixed(2);
      if (odds.isSimulation) {
        headerTitle.innerHTML = `中国体彩量化投注建议 <span style="font-size:10px; color:#ffb700; font-weight:normal; border:1px solid #ffb700; padding:2px 6px; border-radius:4px; margin-left:6px; background:rgba(255,183,0,0.08);">● 官方仿真赔率</span>`;
      } else {
        headerTitle.innerHTML = `中国体彩量化投注建议 <span style="font-size:10px; color:var(--neon-green); font-weight:normal; border:1px solid var(--neon-green); padding:2px 6px; border-radius:4px; margin-left:6px; background:rgba(0,255,136,0.08);">● 官方实时赔率</span>`;
      }
    } else {
      inputH.value = "";
      inputD.value = "";
      inputA.value = "";
      headerTitle.innerHTML = `中国体彩量化投注建议 <span style="font-size:10px; color:var(--text-muted); font-weight:normal; border:1px solid var(--panel-border); padding:2px 6px; border-radius:4px; margin-left:6px; background:rgba(255,255,255,0.02);">● 官方未开售</span>`;
    }
  } catch (err) {
    console.error("加载官方体彩赔率出错:", err);
  }
}

// 13. 加载并更新已完赛场次的 Brier Score 曲线与 Ollama 误差反馈心得
async function loadBacktestHistory() {
  try {
    const res = await fetch(`${API_BASE}/backtest/history`);
    const history = await res.json();

    // 指纹比对防闪烁
    const newFingerprint = JSON.stringify(history);
    if (newFingerprint === lastHistoryFingerprint) {
      return;
    }
    lastHistoryFingerprint = newFingerprint;
    
    const historyListDom = document.getElementById("backtest-history-list");
    if (history && history.length > 0) {
      updateBacktestChart(history);
      const last = history[history.length - 1];
      
      const homeEloShow = last.homeEloDiff >= 0 ? `+${last.homeEloDiff.toFixed(1)}` : last.homeEloDiff.toFixed(1);
      const awayEloShow = last.awayEloDiff >= 0 ? `+${last.awayEloDiff.toFixed(1)}` : last.awayEloDiff.toFixed(1);

      document.getElementById("backtest-review-text").innerHTML = `
        <strong>[场次复盘]</strong>: Brier精度得分: <span style="color:var(--neon-green); font-weight:600;">${last.brierScore.toFixed(3)}</span><br>
        <strong>[Elo实力变化]</strong>: 主队 ${homeEloShow} | 客队 ${awayEloShow}<br>
        <span style="color:var(--text-muted); display:block; margin-top:4px;"><strong>[大模型反思心得]</strong>: "${last.tacticsReview}"</span>
      `;

      // 渲染已完赛历史详情卡片列表 (倒序展示最新的已完赛复盘在最上)
      if (historyListDom) {
        historyListDom.innerHTML = history.slice().reverse().map(h => {
          const matchInfo = matchesMap[h.matchId] || { homeTeam: "主队", awayTeam: "客队", homeScore: 0, awayScore: 0, status: "FT" };
          const homeCn = translateTeamName(matchInfo.homeTeam);
          const awayCn = translateTeamName(matchInfo.awayTeam);
          const hEloDiff = h.homeEloDiff >= 0 ? `+${h.homeEloDiff.toFixed(1)}` : h.homeEloDiff.toFixed(1);
          const aEloDiff = h.awayEloDiff >= 0 ? `+${h.awayEloDiff.toFixed(1)}` : h.awayEloDiff.toFixed(1);
          
          return `
            <div style="background: rgba(255,255,255,0.02); border: 1px solid var(--panel-border); border-radius: 6px; padding: 8px; display: flex; flex-direction: column; gap: 4px; margin-bottom: 6px;">
              <div style="display: flex; justify-content: space-between; font-weight: 600; color: white;">
                <span>${homeCn} ${matchInfo.homeScore} : ${matchInfo.awayScore} ${awayCn}</span>
                <span style="color: var(--neon-green); font-size: 10px;">Brier: ${h.brierScore.toFixed(3)}</span>
              </div>
              <div style="font-size: 10px; color: var(--text-muted);">
                Elo变化: 主队 ${hEloDiff} | 客队 ${aEloDiff}
              </div>
              <div style="font-size: 10px; color: var(--text-muted); background: rgba(136,0,255,0.04); border-left: 2px solid var(--neon-purple); padding: 4px 6px; border-radius: 2px; margin-top: 2px; line-height: 1.4;">
                <strong>复盘反思:</strong> "${h.tacticsReview}"
              </div>
            </div>
          `;
        }).join("");
      }
    } else {
      document.getElementById("backtest-review-text").innerHTML = `● 暂无已结算赛后反思，模型自动进化中...`;
      if (historyListDom) {
        historyListDom.innerHTML = `<div style="color: var(--text-muted); text-align: center; padding: 10px;">暂无已结算完赛历史</div>`;
      }
    }
  } catch (err) {
    console.error("加载复盘历史异常:", err);
  }
}

// 订阅即时比分与状态推送 (SSE)
function setupSSE() {
  const source = new EventSource(`${API_BASE}/matches/stream`);

  source.onmessage = async (event) => {
    console.log("[SSE] 收到即时推送通知:", event.data);
    if (event.data === "match_update") {
      // 收到推送时，一站式触发重算与增量更新，杜绝冗余多次刷新
      await triggerAutoCalculation();
    }
  };

  source.onerror = (err) => {
    console.error("[SSE] 连接异常，尝试重新连接:", err);
  };
}

// 14. 混合过关前端勾选与生成精算推荐交互
function onMatchCheckChange() {
  const chks = document.querySelectorAll(".match-select-chk:checked");
  const count = chks.length;
  document.getElementById("checked-matches-count").innerText = count;
  const btn = document.getElementById("generate-parlay-btn");
  if (count >= 2) {
    btn.disabled = false;
    btn.style.background = "rgba(136,0,255,0.18)";
    btn.style.color = "var(--neon-green)";
    btn.style.borderColor = "var(--neon-green)";
    btn.style.cursor = "pointer";
  } else {
    btn.disabled = true;
    btn.style.background = "rgba(136,0,255,0.08)";
    btn.style.color = "var(--neon-purple)";
    btn.style.borderColor = "var(--neon-purple)";
    btn.style.cursor = "not-allowed";
  }
  updateParlayOptions(count);
}

function updateParlayOptions(count) {
  const subDiv = document.getElementById("parlay-sub-options");
  if (count < 2) {
    subDiv.innerHTML = `<span style="color: var(--text-muted); font-style: italic;">请先在左侧选择至少2场比赛...</span>`;
    return;
  }
  const checkedMode = document.querySelector('input[name="parlay-mode"]:checked');
  const mode = checkedMode ? checkedMode.value : "m_n";
  subDiv.innerHTML = "";

  if (mode === "m_n") {
    let opts = [];
    if (count === 2) {
      opts = [
        { label: "2串1", value: "2x1", checked: true },
        { label: "2串3", value: "2x3" }
      ];
    } else if (count === 3) {
      opts = [
        { label: "3串1", value: "3x1", checked: true },
        { label: "3串3", value: "3x3" },
        { label: "3串4", value: "3x4" },
        { label: "3串7", value: "3x7" }
      ];
    } else if (count === 4) {
      opts = [
        { label: "4串1", value: "4x1" },
        { label: "4串4", value: "4x4", checked: true },
        { label: "4串5", value: "4x5" },
        { label: "4串6", value: "4x6" },
        { label: "4串11", value: "4x11" }
      ];
    } else if (count === 5) {
      opts = [
        { label: "5串1", value: "5x1", checked: true },
        { label: "5串5", value: "5x5" },
        { label: "5串6", value: "5x6" },
        { label: "5串10", value: "5x10" },
        { label: "5串16", value: "5x16" },
        { label: "5串20", value: "5x20" },
        { label: "5串26", value: "5x26" }
      ];
    } else {
      opts = [
        { label: `${count}串1`, value: `${count}x1`, checked: true },
        { label: `${count}串6`, value: `${count}x6` },
        { label: `${count}串7`, value: `${count}x7` },
        { label: `${count}串15`, value: `${count}x15` },
        { label: `${count}串20`, value: `${count}x20` },
        { label: `${count}串22`, value: `${count}x22` },
        { label: `${count}串35`, value: `${count}x35` },
        { label: `${count}串50`, value: `${count}x50` },
        { label: `${count}串57`, value: `${count}x57` }
      ];
    }
    opts.forEach(opt => {
      const checkedAttr = opt.checked ? "checked" : "";
      subDiv.innerHTML += `
        <label style="display: inline-flex; align-items: center; gap: 3px; cursor: pointer; color: white;">
          <input type="radio" name="parlay-sub-opt" value="${opt.value}" ${checkedAttr} style="accent-color: var(--neon-green); cursor: pointer;"> ${opt.label}
        </label>
      `;
    });
  } else {
    for (let i = 2; i <= count; i++) {
      const checkedAttr = i === count ? "checked" : "";
      subDiv.innerHTML += `
        <label style="display: inline-flex; align-items: center; gap: 3px; cursor: pointer; color: white;">
          <input type="checkbox" name="parlay-sub-opt" value="${i}" ${checkedAttr} style="accent-color: var(--neon-green); cursor: pointer;"> ${i}串1
        </label>
      `;
    }
  }
}

document.getElementById("generate-parlay-btn").onclick = async () => {
  const chks = document.querySelectorAll(".match-select-chk:checked");
  const matchIds = Array.from(chks).map(el => el.dataset.matchId);
  if (matchIds.length < 2) return;

  const checkedMode = document.querySelector('input[name="parlay-mode"]:checked');
  const parlayMode = checkedMode ? checkedMode.value : "m_n";
  const subOpts = Array.from(document.querySelectorAll('input[name="parlay-sub-opt"]:checked')).map(el => el.value);

  if (subOpts.length === 0) {
    alert("请至少勾选一个具体的过关选项！");
    return;
  }

  const resultDom = document.getElementById("parlay-result");
  const btn = document.getElementById("generate-parlay-btn");
  const oldText = btn.innerText;
  btn.innerText = "精算中...";
  btn.disabled = true;

  resultDom.innerHTML = `<div style="text-align:center; padding:20px; color:var(--neon-purple); font-size:12px;">
    <span style="display:inline-block; width:12px; height:12px; border:2px solid var(--neon-purple); border-top-color:transparent; border-radius:50%; animation: spin 1s linear infinite; margin-right:6px; vertical-align: middle;"></span>
    过关模型五套方案精算中，请稍候...
  </div>`;

  try {
    const res = await fetch(`${API_BASE}/parlay/recommend`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ 
        matchIds,
        parlayMode,
        parlayOptions: subOpts
      })
    });
    const data = await res.json();
    if (data.error) {
      resultDom.innerHTML = `<div style="color:#ff4a4a; padding:10px; font-size:11px;">❌ 计算失败: ${data.error}</div>`;
      btn.innerText = oldText;
      btn.disabled = false;
      return;
    }

    // 更新侧边栏的小字简略版，增加重新查看弹窗的按钮
    resultDom.innerHTML = `
      <div style="text-align:center; padding:10px 0; display:flex; flex-direction:column; align-items:center; gap:8px;">
        <span style="color:var(--neon-green); font-weight:600; font-size:11.5px;">🎉 精算完成！方案已在弹窗呈现。</span>
        <button id="reopen-parlay-modal-btn" style="background: rgba(0, 255, 136, 0.12); border: 1px solid var(--neon-green); border-radius: 6px; color: var(--neon-green); padding: 5px 12px; font-size: 11px; cursor: pointer; font-weight: 600; transition: all 0.2s; outline: none; box-shadow: 0 2px 8px rgba(0, 255, 136, 0.15);">
          👁️ 重新打开结果弹窗
        </button>
      </div>
    `;
    
    const reopenBtn = document.getElementById("reopen-parlay-modal-btn");
    reopenBtn.onclick = () => {
      document.getElementById("parlay-modal").style.display = "flex";
    };
    reopenBtn.onmouseover = () => {
      reopenBtn.style.background = "rgba(0, 255, 136, 0.25)";
      reopenBtn.style.boxShadow = "0 4px 12px rgba(0, 255, 136, 0.35)";
    };
    reopenBtn.onmouseout = () => {
      reopenBtn.style.background = "rgba(0, 255, 136, 0.12)";
      reopenBtn.style.boxShadow = "0 2px 8px rgba(0, 255, 136, 0.15)";
    };

    btn.innerText = oldText;
    btn.disabled = false;

    // 弹窗数据灌入
    const totalSelected = matchIds.length;
    const activeParlayCount = data.recommended ? data.recommended.length : 0;
    const excludedCount = totalSelected - activeParlayCount;

    let descHtml = `系统已为您对所勾选的场次完成 <strong>${totalSelected}</strong> 场比赛的精算。针对中国体彩官方五大玩法（胜平负、让球、半全场、总进球、比分），我们已最大期望价值（EV）进行了科学组合：`;
    
    // 过滤出含有 ⚠️ 的软风险预警
    const softWarnings = data.excluded ? data.excluded.filter(ex => 
      ex.matchId !== "had" && 
      ex.matchId !== "hhad" && 
      ex.matchId !== "hafu" && 
      ex.matchId !== "ttg" && 
      ex.matchId !== "crs" && 
      ex.reason && ex.reason.includes("⚠️")
    ) : [];

    if (softWarnings.length > 0) {
      let warningDetailHtml = "";
      softWarnings.forEach(w => {
         warningDetailHtml += `<li style="margin-bottom:2px;"><strong>${w.homeTeam} vs ${w.awayTeam}</strong>: ${w.reason}</li>`;
      });
      descHtml = `
        <div style="background: rgba(255, 183, 0, 0.08); border: 1px solid rgba(255, 183, 0, 0.25); border-radius: 8px; padding: 10px; margin-bottom: 12px; color: #ffb700; font-size: 11px; line-height: 1.5;">
          <div style="display: flex; align-items: center; gap: 8px; margin-bottom: 5px;">
            <span style="font-size:16px;">⚠️</span>
            <strong style="color: #ffb700;">智能量化防雷风险提示：</strong>
          </div>
          <ul style="margin: 0; padding-left: 18px; line-height: 1.4;">
            ${warningDetailHtml}
          </ul>
          <span style="font-size:10px; color: var(--text-muted); display:block; margin-top:5px;">💡 温馨提示：系统已尊重您的决定，将上述赛事完整纳入过关组合，请在投注时留意潜在冷门风险。</span>
        </div>
        ${descHtml}
      `;
    }
    document.querySelector(".modal-desc").innerHTML = descHtml;

    const schemesList = document.getElementById("modal-schemes-list");
    schemesList.innerHTML = "";

    if (!data.parlays || data.parlays.length === 0) {
      let exclHtml = "";
      if (data.excluded && data.excluded.length > 0) {
        data.excluded.forEach(ex => {
          if (ex.matchId !== "had" && ex.matchId !== "hhad" && ex.matchId !== "hafu" && ex.matchId !== "ttg" && ex.matchId !== "crs") {
            exclHtml += `
              <div style="background: rgba(255, 74, 74, 0.08); border: 1px solid rgba(255, 74, 74, 0.2); border-radius: 6px; padding: 8px; margin-bottom: 6px; color: #ff9d9d; font-size: 11px; display: flex; flex-direction: column; gap: 3px;">
                <div style="display:flex; justify-content:space-between; font-weight:600;">
                  <span>⚠️ 拦截场次: ${ex.homeTeam} vs ${ex.awayTeam}</span>
                </div>
                <div style="color: var(--text-muted); font-size: 10px;">拦截原因: ${ex.reason}</div>
              </div>
            `;
          }
        });
      }
      schemesList.innerHTML = `
        <div style="grid-column: 1 / -1; display:flex; flex-direction:column; align-items:center; justify-content:center; padding:25px 10px; text-align:center; background: rgba(0,0,0,0.2); border-radius: 12px; border: 1px dashed var(--panel-border); width: 100%;">
          <span style="font-size: 28px; margin-bottom: 8px;">🛡️</span>
          <h3 style="color: #ff4a4a; margin-bottom: 6px; font-size: 13px; font-weight:700;">智能量化风控防御系统已拦截</h3>
          <p style="font-size:11px; color: var(--text-muted); max-width: 480px; line-height: 1.5; margin-bottom: 12px; padding: 0 10px;">
            当前所勾选的比赛存在高危偏置属性（如天气突变、情报利空、均势平局或历史天敌克制等），已被防御机制排除。剩余未被排出的有效场次不足 2 场，故无法生成过关推荐方案：
          </p>
          <div style="width: 100%; text-align: left; max-height: 140px; overflow-y: auto; padding: 0 10px; box-sizing: border-box;">
            ${exclHtml || '<div style="color:var(--text-muted); text-align:center; font-size:11px;">暂无排除明细</div>'}
          </div>
          <p style="font-size:11px; color: var(--neon-purple); margin-top: 12px; font-weight:600;">
            💡 操作提示: 请重新选择其他中低风险的未开赛场次，再次发起精算。
          </p>
        </div>
      `;
      document.getElementById("copy-parlay-summary-btn").disabled = true;
      document.getElementById("copy-parlay-summary-btn").style.opacity = "0.5";
      document.getElementById("copy-parlay-summary-btn").style.cursor = "not-allowed";
      document.getElementById("parlay-modal").style.display = "flex";
      return;
    }

    document.getElementById("copy-parlay-summary-btn").disabled = false;
    document.getElementById("copy-parlay-summary-btn").style.opacity = "1";
    document.getElementById("copy-parlay-summary-btn").style.cursor = "pointer";

    // 寻找概率最高的一套方案作为“主推”
    let maxProb = -1;
    let bestIndex = 0;
    data.parlays.forEach((p, idx) => {
      if (p.comboProb > maxProb) {
        maxProb = p.comboProb;
        bestIndex = idx;
      }
    });

    data.parlays.forEach((p, idx) => {
      const isBest = idx === bestIndex;
      const cardClass = isBest ? "scheme-card best-pick" : "scheme-card";
      const badgeText = isBest ? "🔥 胜率主推" : "📊 玩法方案";

      // 提取本玩法下串关单场明细（借助之前在 Excluded 里面转存的信息）
      let detailDesc = "单场选择暂缺";
      if (data.excluded && data.excluded.length > 0) {
        const matchingDetail = data.excluded.find(e => e.homeTeam === p.parlayType);
        if (matchingDetail) {
          detailDesc = matchingDetail.awayTeam;
        }
      }

      // 计算环形图圆周 2 * PI * r = 2 * 3.14159 * 38 = 238.76
      const radius = 38;
      const circ = 2 * Math.PI * radius;
      const offset = circ - (p.comboProb * circ);

      const roiColor = p.totalEv >= 0 ? "var(--neon-green)" : "#ff4a4a";
      const roiSign = p.totalEv >= 0 ? "+" : "";

      schemesList.innerHTML += `
        <div class="${cardClass}">
          <div class="scheme-header">
            <span style="font-size:13px; font-weight:800; color:white;">${p.parlayType}</span>
            <span class="scheme-badge">${badgeText}</span>
          </div>
          
          <div class="scheme-prob-ring">
            <svg class="prob-circle-svg">
              <circle class="prob-circle-bg" cx="45" cy="45" r="${radius}"></circle>
              <circle class="prob-circle-val" cx="45" cy="45" r="${radius}" 
                style="stroke-dasharray: ${circ}; stroke-dashoffset: ${offset};"></circle>
            </svg>
            <span class="prob-text">${(p.comboProb * 100).toFixed(1)}%</span>
          </div>

          <div class="scheme-meta">
            <div class="scheme-payout-row">
              <span style="color:var(--text-muted);">过关总赔率</span>
              <strong style="color:var(--neon-green);">@${p.comboOdds.toFixed(2)}</strong>
            </div>
            <div class="scheme-payout-row">
              <span style="color:var(--text-muted);">总投注方案</span>
              <strong style="color:white;">${p.winsCount || 1} 注 (${(p.cost || 2.0).toFixed(0)} 元)</strong>
            </div>
            <div class="scheme-payout-row">
              <span style="color:var(--text-muted);">极限最高奖金</span>
              <strong style="color:#ffb700;">${p.singleTicketPayout.toFixed(2)} 元</strong>
            </div>
            <div class="scheme-payout-row">
              <span style="color:var(--text-muted);">期望 ROI</span>
              <strong style="color:${roiColor};">${roiSign}${(p.totalEv * 100).toFixed(1)}%</strong>
            </div>
            <div class="scheme-payout-row">
              <span style="color:var(--text-muted);">建议配资比例</span>
              <strong style="color:white;">${(p.kellyStake * 100).toFixed(1)}%</strong>
            </div>
          </div>

          <div class="scheme-detail-desc">
            <strong>方案细则:</strong><br>${detailDesc}
          </div>
        </div>
      `;
    });

    // 打开弹窗
    document.getElementById("parlay-modal").style.display = "flex";

    // 绑定复制按钮事件
    document.getElementById("copy-parlay-summary-btn").onclick = () => {
      let copyText = `🏆 FIFA 2026 智能过关方案精算推荐 (${matchIds.length}场串关):\n\n`;
      data.parlays.forEach(p => {
        let detailDesc = "单场选择暂缺";
        if (data.excluded && data.excluded.length > 0) {
          const matchingDetail = data.excluded.find(e => e.homeTeam === p.parlayType);
          if (matchingDetail) {
            detailDesc = matchingDetail.awayTeam;
          }
        }
        // 将 HTML 中的 <br> 替换为换行和缩进，便于粘贴阅读
        const textDesc = detailDesc.replace(/<br\s*\/?>/gi, "\n  ");
        copyText += `【${p.parlayType}方案】\n`;
        copyText += `- 单场明细:\n  ${textDesc}\n`;
        copyText += `- 组合总赔率: @${p.comboOdds.toFixed(2)}\n`;
        copyText += `- 投注详情: ${p.winsCount || 1}注 (共${(p.cost || 2.0).toFixed(0)}元)\n`;
        copyText += `- 预估正确率: ${(p.comboProb * 100).toFixed(1)}%\n`;
        copyText += `- 全对极限奖金: ${p.singleTicketPayout.toFixed(2)}元\n`;
        copyText += `- 期望 ROI: ${(p.totalEv * 100).toFixed(1)}%\n\n`;
      });
      copyText += `* 数据基于去抽水 Shin 氏算法与 Dixon-Coles 泊松仿真演算，投注有风险，量化仅供参考。`;
      navigator.clipboard.writeText(copyText).then(() => {
        alert("🎉 五套过关方案文本已成功复制到剪贴板！");
      }).catch(err => {
        alert("复制失败: " + err);
      });
    };

  } catch (err) {
    resultDom.innerHTML = `<div style="color:#ff4a4a; padding:10px; font-size:11px;">❌ 网络异常: ${err.message}</div>`;
    btn.innerText = oldText;
    btn.disabled = false;
  }
};

// 弹窗关闭事件绑定
document.getElementById("close-modal-btn").onclick = () => {
  document.getElementById("parlay-modal").style.display = "none";
};
window.onclick = (event) => {
  const modal = document.getElementById("parlay-modal");
  if (event.target === modal) {
    modal.style.display = "none";
  }
};

// 全自动端到端（E2E）精算测试调试钩子
window.addEventListener("DOMContentLoaded", () => {
  if (window.location.search.includes("auto_verify=true")) {
    setTimeout(() => {
      const chks = document.querySelectorAll(".match-select-chk");
      let count = 0;
      chks.forEach(chk => {
        if (count < 4) {
          chk.checked = true;
          count++;
        }
      });
      onMatchCheckChange();
      setTimeout(() => {
        const btn = document.getElementById("generate-parlay-btn");
        if (btn && !btn.disabled) {
          btn.click();
        }
      }, 500);
    }, 2500); // 等待2.5秒以使赛程拉取完毕
  }
});

