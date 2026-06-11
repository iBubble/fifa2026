const API_BASE = "/api";
let currentMatchID = "";
let currentPredictions = null; // 保存当前比赛测算出的投注项
let matchesMap = {}; // 保存比赛映射用于复盘历史翻译

// 1. 初始化赛程列表并默认选中比赛
async function loadMatches() {
  try {
    const res = await fetch(`${API_BASE}/matches`);
    const matches = await res.json();
    const listDom = document.getElementById("match-list");
    listDom.innerHTML = "";

    matches.forEach(m => {
      matchesMap[m.id] = m;
      const matchTime = new Date(m.scheduledAt).toLocaleString('zh-CN', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' });
      const item = document.createElement("div");
      item.className = "match-item";
      item.dataset.matchId = m.id;
      item.style = "background: rgba(255,255,255,0.03); border: 1px solid var(--panel-border); border-radius: 8px; padding: 10px; cursor: pointer; transition: all 0.2s;";
      item.innerHTML = `
        <div style="display: flex; justify-content: space-between; font-weight: 600;">
          <span>${translateTeamName(m.homeTeam)} vs ${translateTeamName(m.awayTeam)}</span>
          <span style="color: var(--neon-green);">${m.homeScore} - ${m.awayScore}</span>
        </div>
        <div style="display: flex; justify-content: space-between; font-size: 11px; color: var(--text-muted); margin-top: 4px;">
          <span>${m.venue}</span>
          <span>${matchTime} | ${m.status === 'NS' ? '未开赛' : m.status === 'Live' ? '进行中' : m.status}</span>
        </div>
      `;
      item.onclick = () => selectMatch(m.id, item);
      listDom.appendChild(item);
    });

    // 默认选择比赛策略
    if (matches && matches.length > 0) {
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
        }
      }
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
    document.getElementById("current-match-title").innerText = teamSpan.innerText;
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
  loadNews(); // 新增真实新闻初始化拉取
  runOddsShiftsTracker(); // 赔率偏移初始化拉取
  loadBacktestHistory(); // 加载复盘精度看板数据
  setInterval(runArbitrageScanner, 10000); // 10秒定时轮询套利警报
  setInterval(runOddsShiftsTracker, 5000); // 5秒定时更新全球赔率偏移
  runArbitrageScanner();
  startCountdownTimer(); // 开启全自动倒计时精算流

});

// 全自动倒计时精算流
let countdownSeconds = 600; // 10分钟
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
  await loadNews();
  await runOddsShiftsTracker();
  if (currentMatchID) {
    await autoFetchAndCalculate();
  }
  await loadBacktestHistory();

  // 恢复倒计时
  countdownSeconds = 600;
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
    
    // 挑选前 3 篇真实资讯进行合并，作为外围大模型定性偏置的上下文
    const combinedIntel = articles.slice(0, 3).map(art => `${art.title} —— ${art.summary}`).join("\n");
    document.getElementById("qualitative-input").value = combinedIntel;
    document.getElementById("use-llm-chk").checked = true;

    // 2. 自动触发双变量泊松估值
    const resPredict = await fetch(`${API_BASE}/predict`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ matchId: currentMatchID, info: combinedIntel, useLLM: true })
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

    // 渲染体彩优化单
    let html = "";
    const s = data.single;
    if (s.status === "EXCLUDED") {
      html += `
        <div style="border-left: 2px solid red; padding-left: 6px; margin-bottom: 8px; color: #ff4a4a;">
          <strong>🚨 单场风控过滤警告</strong><br>
          ${s.reason}
        </div>
      `;
    } else {
      html += `
        <div style="border-left: 2px solid var(--neon-green); padding-left: 6px; margin-bottom: 8px;">
          <strong style="color: var(--neon-green); font-size:12px;">🎯 竞彩单场对冲方案 (主推+避险)</strong><br>
          主投推荐: <span style="color: white; font-weight:600;">${s.primaryBet} @ ${s.primaryOdds.toFixed(2)}</span> (分配 ${s.primaryStake * 100}%)<br>
          防守对冲: <span style="color: white; font-weight:600;">${s.hedgeBets[0].outcome} @ ${s.hedgeBets[0].odds.toFixed(2)}</span> (分配 ${s.hedgeBets[0].stakePct * 100}%)<br>
          <span style="font-size:10px; color: var(--text-muted); display:block; margin-top:4px; line-height: 1.4;">${s.reason}</span>
        </div>
      `;
    }

    if (data.parlay) {
      html += `
        <div style="border-left: 2px solid var(--neon-purple); padding-left: 6px; margin-top: 8px; border-top: 1px solid var(--panel-border); padding-top: 6px;">
          <strong style="color: var(--neon-purple); font-size:12px;">🔗 2串1 时序避险对冲建议</strong><br>
          <span style="font-size:10px; color: var(--text-muted); line-height: 1.4; display:block; margin-top:2px;">${data.parlay.reason}</span>
        </div>
      `;
    }
    document.getElementById("lottery-result").innerHTML = html;

  } catch (err) {
    console.error("全自动量化流计算失败:", err);
  }
}

// 轮询并更新全球博彩巨头赔率偏移
async function runOddsShiftsTracker() {
  try {
    const res = await fetch(`${API_BASE}/odds/shifts?matchId=${currentMatchID}`);
    const shifts = await res.json();
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

    listDom.innerHTML = "";
    if (articles.length === 0) {
      listDom.innerHTML = `<div style="color: var(--text-muted); font-size: 11px; padding: 10px; text-align: center;">暂无实时外围情报</div>`;
      return;
    }

    articles.forEach(art => {
      const dateStr = new Date(art.time).toLocaleString('zh-CN', { hour: '2-digit', minute: '2-digit' });
      const item = document.createElement("div");
      item.style = "background: rgba(255,255,255,0.02); border: 1px solid var(--panel-border); border-radius: 8px; padding: 8px; display: flex; flex-direction: column; gap: 4px;";
      item.innerHTML = `
        <div style="font-weight: 600; line-height: 1.3;">
          <a href="${art.sourceUrl}" target="_blank" style="color: var(--neon-purple); text-decoration: none; font-size:11px;">
            🔗 ${art.title}
          </a>
        </div>
        <div style="color: var(--text-muted); font-size: 10px; line-height: 1.4;">
          ${art.summary}
        </div>
        <div style="display: flex; justify-content: space-between; align-items: center; margin-top: 4px;">
          <span style="font-size: 9px; color: var(--text-muted);">${art.sourceSite} | ${dateStr}</span>
          <span style="background: rgba(136,0,255,0.1); border: 1px solid var(--neon-purple); border-radius: 4px; padding: 1px 6px; color: var(--text-muted); font-size: 8px; font-weight: 600;">已自动导入</span>
        </div>
      `;
      listDom.appendChild(item);
    });

    // 自动将最新的前 3 条真实情报，实时更新到大模型定性偏置修正输入框中，并默认开启修正开关
    if (articles.length > 0) {
      const topArticles = articles.slice(0, 3);
      const combinedIntel = topArticles.map(art => `【${art.sourceSite}】${art.title} - ${art.summary}`).join("\n");
      const inputDom = document.getElementById("qualitative-input");
      if (inputDom) {
        inputDom.value = combinedIntel;
      }
      const useLLMChk = document.getElementById("use-llm-chk");
      if (useLLMChk) {
        useLLMChk.checked = true;
      }
    }
  } catch (err) {
    console.error("加载情报新闻失败:", err);
    listDom.innerHTML = `<div style="color: #ff4a4a; font-size: 10px; padding: 10px; text-align: center;">⚠️ 实时情报加载失败 (请检查网络链接)</div>`;
  }
}

let lastLotteryData = null;

// 绑定倾向下拉框改变事件，实现无延迟即时本金重算
document.getElementById("lottery-risk-level").onchange = () => {
  if (lastLotteryData) {
    renderLotteryResult(lastLotteryData);
  }
};

function renderLotteryResult(data) {
  lastLotteryData = data;
  const resultDom = document.getElementById("lottery-result");
  const riskLevel = document.getElementById("lottery-risk-level").value;
  
  let html = "";
  const s = data.single;
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

  if (data.parlay) {
    html += `
      <div style="border-left: 2px solid var(--neon-purple); padding-left: 6px; margin-top: 8px; border-top: 1px solid var(--panel-border); padding-top: 6px;">
        <strong style="color: var(--neon-purple); font-size:12px;">🔗 2串1 时序对冲混合过关建议</strong><br>
        <span style="font-size:10px; color: var(--text-muted); line-height: 1.4; display:block; margin-top:2px;">${data.parlay.reason}</span>
      </div>
    `;
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
    renderLotteryResult(data);
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
