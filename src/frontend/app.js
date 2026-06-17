const API_BASE = "/api";
let currentMatchID = "";
let currentPredictions = null; // 保存当前比赛测算出的投注项
let matchesMap = {}; // 保存比赛映射用于复盘历史翻译
let allMatchesData = []; // 保存所有比赛的数据用于赛程积分计算

let lastNewsFingerprint = "";
let lastOddsFingerprint = "";
let lastHistoryFingerprint = "";

// 1. 初始化赛程列表并默认选中比赛
// 1. 初始化赛程列表并默认选中比赛 (原地 DOM 增量更新防闪烁)
async function loadMatches(skipAutoSelect = false) {
  try {
    const res = await fetch(`${API_BASE}/matches`);
    const matches = await res.json();
    allMatchesData = matches;
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

    // 如果赛程积分表弹窗当前是打开状态，自动重绘以实现比分变动后积分表与对阵图动态刷新
    const standingsModal = document.getElementById("schedule-standings-modal");
    if (standingsModal && standingsModal.style.display === "flex") {
      renderScheduleStandings();
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
  
  // 展示 AI 智能对话触发按钮区域，并根据当前窗口状态重置或保留聊天记录
  const chatSection = document.getElementById("ai-chat-section");
  if (chatSection) {
    chatSection.style.display = "block";
    if (isAIChatOpen) {
      const historyDom = document.getElementById("ai-chat-history");
      if (historyDom) {
        const matchInfo = matchesMap[currentMatchID];
        const homeCn = matchInfo ? translateTeamNameText(matchInfo.homeTeam) : "主队";
        const awayCn = matchInfo ? translateTeamNameText(matchInfo.awayTeam) : "客队";
        if (historyDom.children.length <= 1) {
          historyDom.innerHTML = `
            <div style="background: rgba(136,0,255,0.06); border-left: 2px solid var(--neon-purple); padding: 6px 10px; border-radius: 4px; color: var(--text-main); line-height: 1.4; align-self: flex-start; max-width: 85%;">
              你好！我是量化精算AI助手。已为你锁定 <strong>${homeCn} vs ${awayCn}</strong> 场次。你可以向我追问关于这场比赛的战术变数偏置、让球风控或赔率套利等细节。
            </div>
          `;
        } else {
          const switchHtml = `
            <div style="background: rgba(255,255,255,0.03); border: 1px dashed rgba(255,255,255,0.1); padding: 4px 8px; border-radius: 4px; color: var(--text-muted); font-size: 10px; align-self: center; text-align: center; width: calc(100% - 20px); margin: 4px 0;">
              📅 已切换关注场次为：<strong>${homeCn} vs ${awayCn}</strong>
            </div>
          `;
          historyDom.insertAdjacentHTML("beforeend", switchHtml);
          historyDom.scrollTop = historyDom.scrollHeight;
          localStorage.setItem("ai_chat_history", historyDom.innerHTML);
        }
      }
    } else {
      resetAIChatWindow();
    }
  }

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

// 4. 渲染精确比分与大模型 Markdown 战术报告 (重构为左右霓虹并列对比版)
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

  // 1. 提取左半部分（纯数学定量预测）前三名
  const leftSorted = [...(report.originalScoreMatrix || [])].sort((a,b) => b.prob - a.prob).slice(0, 3);
  // 2. 提取右半部分（多Agent反驳纠偏）前三名
  const rightSorted = [...(report.scoreMatrix || [])].sort((a,b) => b.prob - a.prob).slice(0, 3);

  // 3. 多 Agent 内部辩论三阶段 CoT 卡片
  let CoTHtml = "";
  if (report.llmRefined) {
    CoTHtml = `
      <div style="display: flex; flex-direction: column; gap: 8px; margin-top: 8px;">
        <div style="background: rgba(0, 255, 136, 0.04); border-left: 3px solid var(--neon-green); padding: 6px 10px; border-radius: 4px;">
          <strong style="color: var(--neon-green); font-size: 11px; display: block; margin-bottom: 2px;">🟢 常规立论 (Proponent Opinion)</strong>
          <p style="font-size: 11px; line-height: 1.4; color: var(--text-main); margin: 0;">${report.proponentOpinion || "基于基本面与进攻效率倾向，进行常规泊松拉升模拟。"}</p>
        </div>
        <div style="background: rgba(255, 74, 74, 0.04); border-left: 3px solid #ff4a4a; padding: 6px 10px; border-radius: 4px;">
          <strong style="color: #ff4a4a; font-size: 11px; display: block; margin-bottom: 2px;">🔴 魔鬼反驳 (Critique Analysis)</strong>
          <p style="font-size: 11px; line-height: 1.4; color: var(--text-main); margin: 0;">${report.critiqueAnalysis || "无高危大赛心态波动或冷门漏洞检出。"}</p>
        </div>
        <div style="background: rgba(136, 0, 255, 0.04); border-left: 3px solid var(--neon-purple); padding: 6px 10px; border-radius: 4px;">
          <strong style="color: var(--neon-purple); font-size: 11px; display: block; margin-bottom: 2px;">🟣 决策共识 (Consensus Reason)</strong>
          <p style="font-size: 11px; line-height: 1.4; color: var(--text-main); margin: 0;">${report.consensusReason || "中立裁决达成，完成最终概率与偏置参数的平滑融合。"}</p>
        </div>
      </div>
    `;
  } else {
    const isCached = !!_llmCache[report.matchId || currentMatchID];
    const isPending = _llmPending.has(report.matchId || currentMatchID);
    if (isPending) {
      CoTHtml = `
        <div style="background: rgba(0,255,157,0.04); border: 1px dashed var(--neon-green); padding: 12px; border-radius: 6px; text-align: center; font-size: 12px; margin-top: 6px; display: flex; align-items: center; justify-content: center; gap: 10px;">
          <style>@keyframes llmPulse { 0%,100% { opacity: 0.6; } 50% { opacity: 1; } }</style>
          <span style="color: var(--neon-green); animation: llmPulse 1.5s ease-in-out infinite;">⏳ 大模型后台推理中，完成后自动刷新...</span>
        </div>
      `;
    } else {
      CoTHtml = `
        <div style="background: rgba(255,255,255,0.02); border: 1px dashed var(--panel-border); padding: 12px; border-radius: 6px; text-align: center; font-size: 12px; margin-top: 6px; display: flex; align-items: center; justify-content: center; gap: 10px;">
          <span style="color: var(--text-muted);">💡 纯定量模型结果</span>
          <button onclick="autoFetchAndCalculate(true)" class="correct-bias-btn">🔄 启动大模型纠偏</button>
        </div>
      `;
    }
  }

  // 4. 重建双列 HTML 结构
  let html = `
    <div style="display: flex; gap: 15px; flex-wrap: wrap; margin-bottom: 8px; width: 100%;">
      <!-- 左列: 纯定量泊松回归 (Dixon-Coles) -->
      <div style="flex: 1; min-width: 250px; background: var(--sub-panel-bg); border: 1px solid var(--panel-border); border-radius: 8px; padding: 10px; display: flex; flex-direction: column; gap: 6px;">
        <h4 style="color: var(--text-main); font-size: 13px; font-weight: 800; border-bottom: 1px solid rgba(255,255,255,0.06); padding-bottom: 4px; margin-top: 0; margin-bottom: 4px; display: flex; justify-content: space-between;">
          <span>📐 原始泊松回归模型</span>
          <span style="font-size: 10px; color: var(--text-muted); font-weight: normal;">(不含定性修正)</span>
        </h4>
        <div style="display: flex; flex-direction: column; gap: 4px;">
          ${leftSorted.length > 0 ? leftSorted.map(c => `
            <div style="background: rgba(255,255,255,0.02); border: 1px solid rgba(255,255,255,0.04); border-radius: 5px; padding: 4px 8px; display: flex; justify-content: space-between; font-size:11.5px;">
              <span>● 比分 ${c.homeScore} - ${c.awayScore}</span>
              <span style="color: var(--text-main); font-weight: 600;">${(c.prob*100).toFixed(2)}%</span>
            </div>
          `).join("") : `<div style="color:var(--text-muted); font-style:italic;">矩阵未载入</div>`}
        </div>
        <div style="margin-top: 6px; border-top: 1px solid rgba(255,255,255,0.05); padding-top: 6px; display: flex; justify-content: space-between; font-size: 11.5px; color: var(--text-muted);">
          <span>大球 (Over 2.5): <strong style="color:var(--text-white-adapt);">${((report.originalOver2_5Prob || 0)*100).toFixed(1)}%</strong></span>
          <span>小球 (Under 2.5): <strong style="color:var(--text-white-adapt);">${((report.originalUnder2_5Prob || 0)*100).toFixed(1)}%</strong></span>
        </div>
      </div>

      <!-- 右列: 多 Agent 客观反驳决策 (CoT) -->
      <div style="flex: 1; min-width: 250px; background: rgba(136,0,255,0.03); border: 1px solid rgba(136,0,255,0.15); border-radius: 8px; padding: 10px; display: flex; flex-direction: column; gap: 6px; box-shadow: 0 0 10px rgba(136,0,255,0.06);">
        <h4 style="color: var(--neon-purple); font-size: 13px; font-weight: 800; border-bottom: 1px solid rgba(136,0,255,0.15); padding-bottom: 4px; margin-top: 0; margin-bottom: 4px; display: flex; justify-content: space-between; text-shadow: 0 0 8px rgba(136,0,255,0.35);">
          <span>🧠 多 Agent 反驳纠偏版</span>
          <span style="font-size: 10px; color: var(--neon-green); font-weight: bold;">(采信共识推荐)</span>
        </h4>
        <div style="display: flex; flex-direction: column; gap: 4px;">
          ${rightSorted.length > 0 ? rightSorted.map(c => {
            const isBigScore = (c.homeScore + c.awayScore) >= 3;
            const isHighValue = report.critiqueAnalysis && (report.critiqueAnalysis.includes("大比分") || report.critiqueAnalysis.includes("博冷") || report.critiqueAnalysis.includes("逆势") || report.critiqueAnalysis.includes("诱导"));
            const applyGlow = isBigScore && (isHighValue || c.prob > 0.12);
            const glowStyle = applyGlow ? "background: rgba(136,0,255,0.15); border: 1.5px solid var(--neon-purple); box-shadow: 0 0 10px var(--neon-purple); font-weight: 800;" : "background: rgba(255,255,255,0.02); border: 1px solid rgba(255,255,255,0.04);";
            const glowClass = applyGlow ? "class='ev-neon-glow'" : "";
            return `
              <div ${glowClass} style="${glowStyle} border-radius: 5px; padding: 4px 8px; display: flex; justify-content: space-between; font-size:11.5px;">
                <span>● 比分 ${c.homeScore} - ${c.awayScore} ${applyGlow ? "🔥 [逆势高EV]" : ""}</span>
                <span style="color: var(--neon-green); font-weight: 600;">${(c.prob*100).toFixed(2)}%</span>
              </div>
            `;
          }).join("") : `<div style="color:var(--text-muted); font-style:italic;">矩阵未载入</div>`}
        </div>
        <div style="margin-top: 6px; border-top: 1px solid rgba(136,0,255,0.12); padding-top: 6px; display: flex; justify-content: space-between; font-size: 11.5px; color: var(--text-muted);">
          <span>大球 (Over 2.5): <strong style="color:var(--neon-green);">${((report.over25Prob || 0)*100).toFixed(1)}%</strong></span>
          <span>小球 (Under 2.5): <strong style="color:var(--neon-green);">${((report.under25Prob || 0)*100).toFixed(1)}%</strong></span>
        </div>
      </div>
    </div>

    <!-- 辩论 CoT 展示面板 -->
    <div id="prediction-cot-panel" class="${isAIChatOpen ? 'cot-collapsed' : ''}" style="border-top: 1px dashed var(--panel-border); padding-top: 8px; margin-top: 4px; transition: all 0.4s cubic-bezier(0.25, 0.8, 0.25, 1); overflow: hidden; max-height: 500px;">
      <h3 style="border-left-color: var(--neon-purple); font-size: 13.5px; margin-bottom: 6px; font-weight: 800;">🧠 双 Agent 客观反驳决策过程 (CoT)</h3>
      ${CoTHtml}
    </div>
  `;

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

  // 初始化智能对话组件相关的交互事件
  initAIChat();

  // 初始化赛程积分表事件绑定
  initScheduleStandings();
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

// LLM 纠偏结果缓存 { matchId: { report, timestamp } }
const _llmCache = {};
// 正在进行 LLM 推理的比赛集合（防止同一场比赛重复请求）
const _llmPending = new Set();

// 自动串联情报并计算（缓存优先 + 后台静默计算 + 强制重算）
// forceRecalc=true 时清除缓存并强制重新调用大模型
async function autoFetchAndCalculate(forceRecalc = false) {
  if (!currentMatchID) return;
  const myMatchID = currentMatchID;

  try {
    // 1. 获取最新情报列表并串联 (加入安全 try-catch 防御)
    let articles = [];
    try {
      const resNews = await fetch(`${API_BASE}/news?matchId=${myMatchID}`);
      if (resNews.ok) {
        const data = await resNews.json();
        if (Array.isArray(data)) {
          articles = data;
        }
      }
    } catch (e) {
      console.warn("自动计算流中获取情报失败:", e);
    }
    
    const topArticles = articles.slice(0, 3);
    const translatedParts = [];
    for (let art of topArticles) {
      translatedParts.push(`【${art.sourceSite}】${art.title} —— ${art.summary}`);
    }
    const displayIntel = translatedParts.join("\n");

    document.getElementById("qualitative-input").value = displayIntel;
    document.getElementById("use-llm-chk").checked = true;

    // 2. 检查 LLM 缓存 —— 命中则直接渲染，跳过所有网络请求
    if (!forceRecalc && _llmCache[myMatchID]) {
      const cached = _llmCache[myMatchID].report;
      currentPredictions = cached;
      renderPredictionResult(cached);
      await _refreshLotteryPanel(cached, myMatchID);
      return;
    }

    // 3. 缓存未命中：先秒出定量泊松预测
    const resBase = await fetch(`${API_BASE}/predict`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ matchId: myMatchID, info: displayIntel, useLLM: false })
    });
    const baseReport = await resBase.json();
    if (currentMatchID === myMatchID) {
      currentPredictions = baseReport;
      renderPredictionResult(baseReport);
      await _refreshLotteryPanel(baseReport, myMatchID);
    }

    // 4. 强制重算时先清除旧缓存
    if (forceRecalc) {
      delete _llmCache[myMatchID];
      _llmPending.delete(myMatchID);
    }

    // 5. 后台静默发起 LLM 请求（不 abort、不阻塞）
    if (!_llmPending.has(myMatchID)) {
      _llmPending.add(myMatchID);
      // 如果有历史分析结果，作为参考锚点传给大模型进行校准
      let llmInfo = displayIntel;
      const prevCache = _llmCache[myMatchID];
      if (prevCache && prevCache.report) {
        const pr = prevCache.report;
        llmInfo += `\n【上次大模型分析参考(请以此为锚点校准)】lambdaHome偏移=${pr.lambdaHomeOffset||0}, lambdaAway偏移=${pr.lambdaAwayOffset||0}, rho偏移=${pr.rhoOffset||0}, 战术="${pr.tacticsAnalysis||''}"`;
      }
      fetch(`${API_BASE}/predict`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ matchId: myMatchID, info: llmInfo, useLLM: true })
      }).then(r => r.json()).then(llmReport => {
        _llmPending.delete(myMatchID);
        if (llmReport.llmRefined) {
          // 无论用户在不在这场，都缓存结果
          _llmCache[myMatchID] = { report: llmReport, timestamp: Date.now() };
          // 仅当用户仍停留在这场时才刷新 UI
          if (currentMatchID === myMatchID) {
            currentPredictions = llmReport;
            renderPredictionResult(llmReport);
            _refreshLotteryPanel(llmReport, myMatchID);
          }
        }
      }).catch(err => {
        _llmPending.delete(myMatchID);
        console.warn("大模型后台推理失败:", err);
      });
    }

  } catch (err) {
    console.error("全自动量化流计算失败:", err);
  }
}

// 辅助：刷新体彩投注单面板
async function _refreshLotteryPanel(report, matchId) {
  try {
    const oddsH = parseFloat(document.getElementById("lottery-odds-h").value) || 1.95;
    const oddsD = parseFloat(document.getElementById("lottery-odds-d").value) || 3.20;
    const oddsA = parseFloat(document.getElementById("lottery-odds-a").value) || 3.80;
    let matchIDs = [matchId];
    const items = Array.from(document.querySelectorAll(".match-item"));
    for (let item of items) {
      const mid = item.dataset.matchId;
      if (mid && mid !== matchId) { matchIDs.push(mid); break; }
    }
    const res = await fetch(`${API_BASE}/lottery/recommend`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ matchIds: matchIDs, odds: [oddsH, oddsD, oddsA], predictReport: report })
    });
    const data = await res.json();
    if (currentMatchID === matchId) {
      lastLotteryData = data;
      await renderLotteryPanel(data);
    }
  } catch (err) {
    console.error("量化面板刷新异常:", err);
    const resDom = document.getElementById("lottery-result");
    if (resDom) {
      resDom.innerHTML = `<div style="color:#ff4a4a; font-size:11px; padding:10px;">⚠️ 前端量化建议渲染异常: ${err.message}</div>`;
    }
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
  try {
    let html = "";
  
  if (recommendData) {
    if (recommendData.error) {
      html += `<div style="color: #ff4a4a; font-size: 11px; padding: 10px;">⚠️ 接口请求失败: ${recommendData.error}</div>`;
    } else if (recommendData.fivePlays && recommendData.fivePlays.length > 0) {
      // 1. 生成 5 个 Tab 切换栏
      html += `
        <div style="margin-bottom: 4px;">
          <strong style="color: var(--neon-green); font-size: 11px; display: block; border-left: 2px solid var(--neon-green); padding-left: 6px; margin-bottom: 8px;">
            📊 官方五大玩法量化精算明细
          </strong>
          
          <div class="lottery-tabs" style="display: flex; gap: 3px; margin-bottom: 8px; border-bottom: 1px solid rgba(255,255,255,0.06); padding-bottom: 6px;">
            <button class="lottery-tab-btn active" data-tab="had" style="flex: 1; padding: 5px 1px; font-size: 11.5px; background: rgba(0,255,136,0.1); border: 1px solid var(--neon-green); border-radius: 4px; color: var(--neon-green); cursor: pointer; transition: all 0.2s; outline: none; font-weight: 600;">胜平负</button>
            <button class="lottery-tab-btn" data-tab="hhad" style="flex: 1; padding: 5px 1px; font-size: 11.5px; background: rgba(255,255,255,0.02); border: 1px solid var(--panel-border); border-radius: 4px; color: var(--text-muted); cursor: pointer; transition: all 0.2s; outline: none; font-weight: 600;">让球</button>
            <button class="lottery-tab-btn" data-tab="crs" style="flex: 1; padding: 5px 1px; font-size: 11.5px; background: rgba(255,255,255,0.02); border: 1px solid var(--panel-border); border-radius: 4px; color: var(--text-muted); cursor: pointer; transition: all 0.2s; outline: none; font-weight: 600;">比分</button>
            <button class="lottery-tab-btn" data-tab="ttg" style="flex: 1; padding: 5px 1px; font-size: 11.5px; background: rgba(255,255,255,0.02); border: 1px solid var(--panel-border); border-radius: 4px; color: var(--text-muted); cursor: pointer; transition: all 0.2s; outline: none; font-weight: 600;">总进球</button>
            <button class="lottery-tab-btn" data-tab="hafu" style="flex: 1; padding: 5px 1px; font-size: 11.5px; background: rgba(255,255,255,0.02); border: 1px solid var(--panel-border); border-radius: 4px; color: var(--text-muted); cursor: pointer; transition: all 0.2s; outline: none; font-weight: 600;">半全场</button>
          </div>
          <div style="display: flex; flex-direction: column; gap: 8px;">
      `;

      // 2. 生成玩法卡片，默认除首个外隐藏
      recommendData.fivePlays.forEach(play => {
        const cardDisplay = play.playCode === "had" ? "block" : "none";

        let safeHtml = "";
        const isSafeUnsold = !play.safe || play.safe.length === 0 || play.safe[0].odds <= 0 || play.safe[0].option === "未开售" || play.safe[0].option === "不可售" || play.safe[0].option === "--";
        if (isSafeUnsold) {
          safeHtml = `
            <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 4px;">
              <span style="color: var(--text-muted); font-size: 10px; display: flex; align-items: center; gap: 4px;">
                <span style="background: rgba(0, 255, 136, 0.15); color: var(--neon-green); padding: 1px 4px; border-radius: 3px; font-size: 9px; font-weight: bold;">稳妥型</span>
              </span>
              <span style="color: var(--text-muted); font-style: italic; font-weight: 500; font-size: 12.5px;">不可售</span>
            </div>
          `;
        } else {
          let listItems = "";
          play.safe.forEach((opt, idx) => {
            listItems += `
              <div style="display: flex; justify-content: space-between; align-items: center; font-size: 12px; line-height: 1.4; margin-bottom: 2px;">
                <span style="color: var(--text-white-adapt); font-weight: 600;">${idx + 1}. ${opt.option} <span style="color: var(--neon-green);">@${opt.odds.toFixed(2)}</span></span>
                <span style="color: var(--text-muted); font-size: 11px;">几率: <strong style="color: var(--text-white-adapt);">${(opt.prob * 100).toFixed(1)}%</strong></span>
              </div>
            `;
          });
          safeHtml = `
            <div style="margin-bottom: 6px; border-bottom: 1px solid rgba(255,255,255,0.03); padding-bottom: 6px;">
              <div style="display: flex; align-items: center; gap: 4px; margin-bottom: 4px;">
                <span style="background: rgba(0, 255, 136, 0.15); color: var(--neon-green); padding: 1px 4px; border-radius: 3px; font-size: 9px; font-weight: bold;">稳妥型 (几率前三)</span>
              </div>
              <div style="display: flex; flex-direction: column; gap: 2px; padding-left: 4px;">
                ${listItems}
              </div>
            </div>
          `;
        }

        let aggressiveHtml = "";
        const isAggressiveUnsold = !play.aggressive || play.aggressive.length === 0 || play.aggressive[0].odds <= 0 || play.aggressive[0].option === "未开售" || play.aggressive[0].option === "不可售" || play.aggressive[0].option === "--";
        if (isAggressiveUnsold) {
          aggressiveHtml = `
            <div style="display: flex; justify-content: space-between; align-items: center;">
              <span style="color: var(--text-muted); font-size: 10px; display: flex; align-items: center; gap: 4px;">
                <span style="background: rgba(136, 0, 255, 0.15); color: #e2b3ff; padding: 1px 4px; border-radius: 3px; font-size: 9px; font-weight: bold;">激进型</span>
              </span>
              <span style="color: var(--text-muted); font-style: italic; font-weight: 500; font-size: 12.5px;">不可售</span>
            </div>
          `;
        } else {
          let listItems = "";
          play.aggressive.forEach((opt, idx) => {
            const evText = opt.ev >= 0 ? `+${opt.ev.toFixed(2)}` : opt.ev.toFixed(2);
            const evColor = opt.ev >= 0 ? "var(--neon-green)" : "#ff4a4a";
            listItems += `
              <div style="display: flex; justify-content: space-between; align-items: center; font-size: 12px; line-height: 1.4; margin-bottom: 2px;">
                <span style="color: var(--text-white-adapt); font-weight: 600;">${idx + 1}. ${opt.option} <span style="color: var(--neon-green);">@${opt.odds.toFixed(2)}</span></span>
                <span style="color: var(--text-muted); font-size: 11px;">几率: <strong style="color: var(--text-white-adapt);">${(opt.prob * 100).toFixed(1)}%</strong> (EV: <strong style="color: ${evColor};">${evText}</strong>)</span>
              </div>
            `;
          });
          aggressiveHtml = `
            <div>
              <div style="display: flex; align-items: center; gap: 4px; margin-bottom: 4px;">
                <span style="background: rgba(136, 0, 255, 0.15); color: #e2b3ff; padding: 1px 4px; border-radius: 3px; font-size: 9px; font-weight: bold;">激进型 (期望前三)</span>
              </div>
              <div style="display: flex; flex-direction: column; gap: 2px; padding-left: 4px;">
                ${listItems}
              </div>
            </div>
          `;
        }

            html += `
              <div class="lottery-play-card" id="play-card-${play.playCode}" style="display: ${cardDisplay}; background: rgba(136, 0, 255, 0.02); border: 1px solid var(--panel-border); border-radius: 6px; padding: 8px; font-size: 12.5px;">
                <div style="font-weight: 700; color: var(--text-white-adapt); margin-bottom: 6px; display: flex; align-items: center; justify-content: space-between; font-size: 13.5px;">
                  <span>🎫 ${play.playName}</span>
                </div>
                
                <div style="display: flex; flex-direction: column; gap: 5px;">
                  ${safeHtml}
                  ${aggressiveHtml}
                </div>
              </div>
            `;
      });
      html += `
          </div>
        </div>
      `;
    } else {
      html += `<div style="color: #ff4a4a; font-size: 11px; padding: 10px;">⚠️ 暂无精算投注数据 (后端未返回五大玩法明细)</div>`;
    }
  } else {
    html += `<div style="margin-bottom: 12px;">● 请在左侧选择比赛，系统将自动生成五大玩法最佳量化投注建议...</div>`;
  }



  resultDom.innerHTML = html;

  // 3. 动态绑定五大玩法 Tab 切换交互事件
  const tabBtns = resultDom.querySelectorAll(".lottery-tab-btn");
  tabBtns.forEach(btn => {
    btn.onclick = () => {
      tabBtns.forEach(b => {
        b.classList.remove("active");
        b.style.background = "rgba(255,255,255,0.02)";
        b.style.borderColor = "var(--panel-border)";
        b.style.color = "var(--text-muted)";
      });
      btn.classList.add("active");
      btn.style.background = "rgba(0,255,136,0.1)";
      btn.style.borderColor = "var(--neon-green)";
      btn.style.color = "var(--neon-green)";
      
      const targetTab = btn.dataset.tab;
      resultDom.querySelectorAll(".lottery-play-card").forEach(card => {
        if (card.id === `play-card-${targetTab}`) {
          card.style.display = "block";
        } else {
          card.style.display = "none";
        }
      });
    };
  });

  // 需求 3：动态更新单场记录按钮状态并绑定保存事件
  const saveSingleBtn = document.getElementById("save-single-btn");
  if (saveSingleBtn) {
    if (recommendData && recommendData.single && recommendData.single.status !== "EXCLUDED") {
      saveSingleBtn.disabled = false;
      saveSingleBtn.style.opacity = "1";
      saveSingleBtn.style.cursor = "pointer";
      
      saveSingleBtn.onclick = async () => {
        const oldText = saveSingleBtn.innerText;
        saveSingleBtn.innerText = "记录中...";
        saveSingleBtn.disabled = true;
        
        let hedgeBetOutcome = "";
        let hedgeBetOdds = 0.0;
        let hedgeBetStake = 0.0;
        if (recommendData.single.hedgeBets && recommendData.single.hedgeBets.length > 0) {
          const hedge = recommendData.single.hedgeBets[0];
          hedgeBetOutcome = hedge.outcome;
          hedgeBetOdds = hedge.odds;
          hedgeBetStake = 20.0;
        }
        
        try {
          const saveRes = await fetch(`${API_BASE}/lottery/save-single`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
              matchId: currentMatchID,
              oddsH: parseFloat(document.getElementById("lottery-odds-h")?.value || 1.95),
              oddsD: parseFloat(document.getElementById("lottery-odds-d")?.value || 3.20),
              oddsA: parseFloat(document.getElementById("lottery-odds-a")?.value || 3.80),
              primaryBet: recommendData.single.primaryBet,
              primaryOdds: recommendData.single.primaryOdds,
              hedgeBet: hedgeBetOutcome,
              hedgeOdds: hedgeBetOdds,
              hedgeAmt: hedgeBetStake,
              reason: recommendData.single.reason
            })
          });
          const saveResult = await saveRes.json();
          if (saveResult.status === "success") {
            alert("💾 单场量化投注方案记录成功，已加入复盘库！");
            renderLotteryPanel(recommendData);
          } else {
            alert("记录失败: " + (saveResult.error || "未知错误"));
          }
        } catch (err) {
          alert("记录网络异常: " + err.message);
        } finally {
          saveSingleBtn.innerText = oldText;
          saveSingleBtn.disabled = false;
        }
      };
    } else {
      saveSingleBtn.disabled = true;
      saveSingleBtn.style.opacity = "0.5";
      saveSingleBtn.style.cursor = "not-allowed";
      saveSingleBtn.onclick = null;
    }
  }
  } catch (err) {
    console.error("renderLotteryPanel internal error:", err);
    resultDom.innerHTML = `<div style="color:#ff4a4a; font-size:11px; padding:10px;">⚠️ 建议渲染逻辑异常: ${err.message}</div>`;
  }
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
        headerTitle.innerHTML = `中国体彩量化投注建议 <span style="font-size:10px; color:#ffb700; font-weight:normal; border:1px solid #ffb700; padding:2px 6px; border-radius:4px; margin-left:6px; background:rgba(255,183,0,0.08);">● 官方参考赔率</span>`;
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
        <strong>[Elo实力变化]</strong>: ${translateTeamName(last.homeTeam || "主队")} ${last.homeEloDiff >= 0 ? `+${last.homeEloDiff.toFixed(1)}` : last.homeEloDiff.toFixed(1)} | ${translateTeamName(last.awayTeam || "客队")} ${last.awayEloDiff >= 0 ? `+${last.awayEloDiff.toFixed(1)}` : last.awayEloDiff.toFixed(1)}<br>
        <span style="color:var(--text-muted); display:block; margin-top:4px;"><strong>[大模型反思心得]</strong>: "${last.tacticsReview}"</span>
      `;

      // 渲染已完赛历史详情卡片列表 (倒序展示最新的已完赛复盘在最上)
      if (historyListDom) {
        historyListDom.innerHTML = history.slice().reverse().map(h => {
          const matchInfo = matchesMap[h.matchId] || {};
          const homeCn = translateTeamName(h.homeTeam || matchInfo.homeTeam || "主队");
          const awayCn = translateTeamName(h.awayTeam || matchInfo.awayTeam || "客队");
          const homeScore = h.homeScore !== undefined ? h.homeScore : (matchInfo.homeScore || 0);
          const awayScore = h.awayScore !== undefined ? h.awayScore : (matchInfo.awayScore || 0);
          const hEloDiff = h.homeEloDiff >= 0 ? `+${h.homeEloDiff.toFixed(1)}` : h.homeEloDiff.toFixed(1);
          const aEloDiff = h.awayEloDiff >= 0 ? `+${h.awayEloDiff.toFixed(1)}` : h.awayEloDiff.toFixed(1);
          
          return `
            <div style="background: rgba(255,255,255,0.02); border: 1px solid var(--panel-border); border-radius: 6px; padding: 8px; display: flex; flex-direction: column; gap: 4px; margin-bottom: 6px;">
              <div style="display: flex; justify-content: space-between; font-weight: 600; color: var(--text-white-adapt);">
                <span>${homeCn} ${homeScore} : ${awayScore} ${awayCn}</span>
                <span style="color: var(--neon-green); font-size: 10px;">Brier: ${h.brierScore.toFixed(3)}</span>
              </div>
              <div style="font-size: 10px; color: var(--text-muted);">
                Elo变化: ${homeCn} ${hEloDiff} | ${awayCn} ${aEloDiff}
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
        <label style="display: inline-flex; align-items: center; gap: 3px; cursor: pointer; color: var(--text-white-adapt);">
          <input type="radio" name="parlay-sub-opt" value="${opt.value}" ${checkedAttr} style="accent-color: var(--neon-green); cursor: pointer;"> ${opt.label}
        </label>
      `;
    });
  } else {
    for (let i = 2; i <= count; i++) {
      const checkedAttr = i === count ? "checked" : "";
      subDiv.innerHTML += `
        <label style="display: inline-flex; align-items: center; gap: 3px; cursor: pointer; color: var(--text-white-adapt);">
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

    const activeParlays = data.parlays || [];

    if (activeParlays.length === 0) {
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
        <div style="grid-column: 1 / -1; display:flex; flex-direction:column; align-items:center; justify-content:center; padding:25px 10px; text-align:center; background: var(--sub-panel-bg); border-radius: 12px; border: 1px dashed var(--panel-border); width: 100%;">
          <span style="font-size: 28px; margin-bottom: 8px;">🛡️</span>
          <h3 style="color: #ff4a4a; margin-bottom: 6px; font-size: 13px; font-weight:700;">智能量化风控防御系统已拦截</h3>
          <p style="font-size:11px; color: var(--text-muted); max-width: 480px; line-height: 1.5; margin-bottom: 12px; padding: 0 10px;">
            当前所勾选的比赛存在高危偏置属性，或玩法均未开售导致无有效投注组合，故无法生成过关推荐方案：
          </p>
          <div style="width: 100%; text-align: left; max-height: 140px; overflow-y: auto; padding: 0 10px; box-sizing: border-box;">
            ${exclHtml || '<div style="color:var(--text-muted); text-align:center; font-size:11px;">暂无排除明细</div>'}
          </div>
          <p style="font-size:11px; color: var(--neon-purple); margin-top: 12px; font-weight:600;">
            💡 操作提示: 请重新选择其他已开售且中低风险的未完赛场次，再次发起精算。
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

    // 寻找概率最高且正常开售的一套方案作为“主推”
    let maxProb = -1;
    let bestIndex = -1;
    activeParlays.forEach((p, idx) => {
      const isAvailable = p.comboOdds > 0 && p.winsCount > 0;
      if (isAvailable && p.comboProb > maxProb) {
        maxProb = p.comboProb;
        bestIndex = idx;
      }
    });

    activeParlays.forEach((p, idx) => {
      const isBest = idx === bestIndex;
      const isAvailable = p.comboOdds > 0 && p.winsCount > 0;

      let cardClass = "scheme-card";
      let badgeText = "📊 玩法方案";
      if (isBest) {
        cardClass = "scheme-card best-pick";
        badgeText = "🔥 胜率主推";
      } else if (!isAvailable) {
        cardClass = "scheme-card unavailable-pick";
        badgeText = "🚫 暂不可投";
      }

      // 计算环形图圆周 2 * PI * r = 2 * 3.14159 * 38 = 238.76
      const radius = 38;
      const circ = 2 * Math.PI * radius;
      const offset = circ - (p.comboProb * circ);

      if (!isAvailable) {
        let matchingDetail = "部分赛事该玩法未开售，场数不足。";
        if (data.excluded && data.excluded.length > 0) {
          const ex = data.excluded.find(e => e.matchId === p.parlayType || e.homeTeam === p.parlayType);
          if (ex) {
            matchingDetail = ex.awayTeam || ex.reason || "本组赛事中仅有部分场次开售该玩法，不足以组成所选过关方式。";
          }
        }

        schemesList.innerHTML += `
          <div class="${cardClass}" style="opacity: 0.65; filter: grayscale(80%);">
            <div class="scheme-header">
              <span style="font-size:13px; font-weight:800; color:var(--text-white-adapt);">${p.parlayType}</span>
              <span class="scheme-badge" style="background:#ff4a4a; color:white;">${badgeText}</span>
            </div>
            
            <div class="scheme-prob-ring">
              <svg class="prob-circle-svg">
                <circle class="prob-circle-bg" cx="45" cy="45" r="${radius}"></circle>
                <circle class="prob-circle-val" cx="45" cy="45" r="${radius}" 
                  style="stroke-dasharray: ${circ}; stroke-dashoffset: ${circ}; stroke: #666;"></circle>
              </svg>
              <span class="prob-text" style="color:var(--text-muted);">--</span>
            </div>

            <div class="scheme-meta">
              <div class="scheme-payout-row">
                <span style="color:var(--text-muted);">过关总赔率</span>
                <strong style="color:var(--text-muted);">--</strong>
              </div>
              <div class="scheme-payout-row">
                <span style="color:var(--text-muted);">总投注方案</span>
                <strong style="color:var(--text-muted);">--</strong>
              </div>
              <div class="scheme-payout-row">
                <span style="color:var(--text-muted);">建议配资比例</span>
                <strong style="color:var(--text-muted);">0.0%</strong>
              </div>
            </div>

            <div class="scheme-detail" style="border-top: 1px solid rgba(255,255,255,0.05); margin-top: 10px; padding-top: 8px; font-size: 10px; color: #ff9a9a; text-align: left; line-height: 1.4; max-height: 60px; overflow-y: auto;">
              ${matchingDetail}
            </div>
          </div>
        `;
        return;
      }

      // 提取本玩法下串关单场明细（借助之前在 Excluded 里面转存的信息）
      let detailDesc = "单场选择暂缺";
      if (data.excluded && data.excluded.length > 0) {
        const matchingDetail = data.excluded.find(e => e.homeTeam === p.parlayType);
        if (matchingDetail) {
          detailDesc = matchingDetail.awayTeam;
        }
      }



      const roiColor = p.totalEv >= 0 ? "var(--neon-green)" : "#ff4a4a";
      const roiSign = p.totalEv >= 0 ? "+" : "";

      schemesList.innerHTML += `
        <div class="${cardClass}">
          <div class="scheme-header">
            <span style="font-size:13px; font-weight:800; color:var(--text-white-adapt);">${p.parlayType}</span>
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
              <strong style="color:var(--text-white-adapt);">${p.winsCount || 1} 注 (${(p.cost || 2.0).toFixed(0)} 元)</strong>
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
              <strong style="color:var(--text-white-adapt);">${(p.kellyStake * 100).toFixed(1)}%</strong>
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

    // 绑定记录过关方案按钮事件
    document.getElementById("record-parlay-btn").onclick = async () => {
      const btn = document.getElementById("record-parlay-btn");
      const oldText = btn.innerText;
      btn.innerText = "记录中...";
      btn.disabled = true;
      try {
        const saveRes = await fetch(`${API_BASE}/lottery/save-parlay`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            matchIds: matchIds.join(","),
            parlayMode: parlayMode,
            parlayOptions: subOpts.join(","),
            parlays: data.parlays,
            excluded: data.excluded
          })
        });
        const saveResult = await saveRes.json();
        if (saveResult.status === "success") {
          alert(`💾 ${saveResult.saved} 套过关方案记录成功，已加入复盘库！`);
        } else {
          alert("记录失败: " + (saveResult.error || "未知错误"));
        }
      } catch (err) {
        alert("记录失败，网络异常: " + err.message);
      } finally {
        btn.innerText = oldText;
        btn.disabled = false;
      }
    };

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
      copyText += `* 数据基于去抽水 Shin 氏算法与 Dixon-Coles 泊松模型演算，投注有风险，量化仅供参考。`;
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

// 全自动量化交易复盘对账中心渲染逻辑
async function showSingleSaved() {
  const modal = document.getElementById("single-saved-modal");
  const listDom = document.getElementById("single-saved-list");
  const selectAllChk = document.getElementById("single-saved-select-all");
  if (selectAllChk) selectAllChk.checked = false; // 重置全选状态

  listDom.innerHTML = `<div style="text-align:center; padding:10px; color:#00bfff; font-size:11px;">加载已保存方案...</div>`;
  modal.style.display = "flex";
  try {
    const res = await fetch(`${API_BASE}/lottery/saved`);
    const history = await res.json();
    const list = history.filter(h => h.planType === "single");
    if (list.length === 0) {
      listDom.innerHTML = `<div style="text-align:center; padding:20px; color:var(--text-muted); font-size:11px;">📭 暂无已保存的单场方案记录</div>`;
      return;
    }
    listDom.innerHTML = list.map(h => {
      const home = translateTeamName(h.homeTeam);
      const away = translateTeamName(h.awayTeam);
      const isFT = h.status === "FT";
      
      let primaryBadge = "";
      let hedgeBadge = "";
      let statusBadge = "";
      
      if (h.isSettled === 1 && isFT) {
        statusBadge = `<span style="background:rgba(0, 255, 136, 0.12); color:var(--neon-green); padding:2px 6px; border-radius:4px; font-weight:bold; font-size:10px;">已完赛结算</span>`;
        primaryBadge = h.primaryHit ? `<span style="color:var(--neon-green);">主推: 🎯 ${h.primaryBet}(中)</span>` : `<span style="color:var(--text-muted);">主推: ❌ ${h.primaryBet}(失)</span>`;
        if (h.hedgeBet) {
          hedgeBadge = h.hedgeHit ? `<span style="color:#ffeb3b; margin-left:8px;">对冲: 🛡️ ${h.hedgeBet}(中)</span>` : `<span style="color:var(--text-muted); margin-left:8px;">对冲: ❌ ${h.hedgeBet}(失)</span>`;
        } else {
          hedgeBadge = `<span style="color:var(--text-muted); margin-left:8px;">无对冲</span>`;
        }
      } else {
        statusBadge = `<span style="background:rgba(0, 191, 255, 0.12); color:#00bfff; padding:2px 6px; border-radius:4px; font-weight:bold; font-size:10px;">待结算</span>`;
        primaryBadge = `<span style="color:var(--text-main);">主推: ${h.primaryBet}(待定)</span>`;
        if (h.hedgeBet) {
          hedgeBadge = `<span style="color:var(--text-main); margin-left:8px;">对冲: ${h.hedgeBet}(待定)</span>`;
        } else {
          hedgeBadge = `<span style="color:var(--text-muted); margin-left:8px;">无对冲</span>`;
        }
      }
      
      return `
        <div style="display:flex; align-items:center; background:rgba(255,255,255,0.02); border:1px solid rgba(255,255,255,0.05); border-radius:6px; padding:8px; gap:8px;">
          <input type="checkbox" class="saved-item-chk-single" data-id="${h.id}" style="accent-color:#00bfff; width:14px; height:14px; cursor:pointer; flex-shrink:0;">
          <div style="flex:1; font-size:11px; display:flex; flex-direction:column; gap:4px;">
            <div style="display:flex; justify-content:space-between; align-items:center;">
              <span><strong>${home} ${isFT ? `${h.homeScore}:${h.awayScore}` : 'vs'} ${away}</strong></span>
              <div style="display:flex; align-items:center; gap:8px;">
                ${statusBadge}
                <span class="delete-saved-item-btn" data-id="${h.id}" data-type="single" style="color:#ff4a4a; cursor:pointer; font-size:10px; font-weight:bold; padding:2px 4px; background:rgba(255,74,74,0.1); border-radius:4px;">🗑️ 删除</span>
              </div>
            </div>
            <div>${primaryBadge} ${hedgeBadge}</div>
            <div style="font-size:9px; color:var(--text-muted); text-align:right; border-top:1px solid rgba(255,255,255,0.02); padding-top:4px; margin-top:2px;">保存时间: ${h.createdAt}</div>
          </div>
        </div>
      `;
    }).join("");
    
    // 绑定单条删除事件
    listDom.querySelectorAll(".delete-saved-item-btn").forEach(btn => {
      btn.onclick = async (e) => {
        e.stopPropagation();
        const id = parseInt(btn.getAttribute("data-id"));
        if (await customConfirm("确定要删除这条保存记录吗？")) {
          await deleteSavedRecords([id], "single");
        }
      };
    });
  } catch (err) {
    listDom.innerHTML = `<div style="color:#ff4a4a; padding:10px; font-size:11px; text-align:center;">数据拉取失败: ${err.message}</div>`;
  }
}

async function showParlaySaved() {
  const modal = document.getElementById("parlay-saved-modal");
  const listDom = document.getElementById("parlay-saved-list");
  const selectAllChk = document.getElementById("parlay-saved-select-all");
  if (selectAllChk) selectAllChk.checked = false; // 重置全选状态

  listDom.innerHTML = `<div style="text-align:center; padding:10px; color:#00bfff; font-size:11px;">加载已保存方案...</div>`;
  modal.style.display = "flex";
  try {
    const res = await fetch(`${API_BASE}/lottery/saved`);
    const history = await res.json();
    const list = history.filter(h => h.planType === "parlay");
    if (list.length === 0) {
      listDom.innerHTML = `<div style="text-align:center; padding:20px; color:var(--text-muted); font-size:11px;">📭 暂无已保存的过关方案记录</div>`;
      return;
    }
    listDom.innerHTML = list.map(h => {
      let statusBadge = "";
      if (h.isSettled === 1) {
        statusBadge = h.primaryHit 
          ? `<span style="background:rgba(0, 255, 136, 0.12); color:var(--neon-green); padding:2px 6px; border-radius:4px; font-weight:bold; font-size:10px;">🎯 组合中奖</span>`
          : `<span style="background:rgba(255, 255, 255, 0.05); color:var(--text-muted); padding:2px 6px; border-radius:4px; font-weight:bold; font-size:10px;">❌ 组合失效</span>`;
      } else {
        statusBadge = `<span style="background:rgba(0, 191, 255, 0.12); color:#00bfff; padding:2px 6px; border-radius:4px; font-weight:bold; font-size:10px;">待结算</span>`;
      }

      const uniqueLegsMap = {};
      (h.tickets || []).forEach(tk => {
        (tk.legs || []).forEach(leg => {
          const key = `${leg.matchId}_${leg.option}`;
          uniqueLegsMap[key] = leg;
        });
      });
      const legsHtml = Object.values(uniqueLegsMap).map(leg => {
        const homeCn = translateTeamName(leg.homeTeam || "");
        const awayCn = translateTeamName(leg.awayTeam || "");
        const isFT = leg.status === "FT";
        let hitText = "待定";
        if (isFT) {
          hitText = leg.hit ? `<span style="color:var(--neon-green);">中</span>` : `<span style="color:var(--text-muted);">失</span>`;
        }
        return `
          <div style="color:var(--text-muted); padding:2px 0; border-bottom:1px dashed rgba(255,255,255,0.02);">
            • ${homeCn} ${isFT ? `${leg.homeScore}:${leg.awayScore}` : 'vs'} ${awayCn} | 预测: ${leg.option} (${hitText})
          </div>
        `;
      }).join("");

      return `
        <div style="display:flex; align-items:center; background:rgba(255,255,255,0.02); border:1px solid rgba(255,255,255,0.05); border-radius:6px; padding:8px; gap:8px;">
          <input type="checkbox" class="saved-item-chk-parlay" data-id="${h.id}" style="accent-color:#00bfff; width:14px; height:14px; cursor:pointer; flex-shrink:0;">
          <div style="flex:1; font-size:11px; display:flex; flex-direction:column; gap:4px;">
            <div style="display:flex; justify-content:space-between; align-items:center;">
              <span><strong>${h.homeTeam} (${h.awayTeam})</strong></span>
              <div style="display:flex; align-items:center; gap:8px;">
                ${statusBadge}
                <span class="delete-saved-item-btn" data-id="${h.id}" data-type="parlay" style="color:#ff4a4a; cursor:pointer; font-size:10px; font-weight:bold; padding:2px 4px; background:rgba(255,74,74,0.1); border-radius:4px;">🗑️ 删除</span>
              </div>
            </div>
            <div style="padding-left:6px; border-left:2px solid #00bfff; display:flex; flex-direction:column; gap:2px; margin-top:2px;">
              ${legsHtml}
            </div>
            <div style="font-size:9px; color:var(--text-muted); text-align:right; border-top:1px solid rgba(255,255,255,0.02); padding-top:4px; margin-top:2px;">保存时间: ${h.createdAt}</div>
          </div>
        </div>
      `;
    }).join("");
    
    // 绑定单条删除事件
    listDom.querySelectorAll(".delete-saved-item-btn").forEach(btn => {
      btn.onclick = async (e) => {
        e.stopPropagation();
        const id = parseInt(btn.getAttribute("data-id"));
        if (await customConfirm("确定要删除这条保存记录吗？")) {
          await deleteSavedRecords([id], "parlay");
        }
      };
    });
  } catch (err) {
    listDom.innerHTML = `<div style="color:#ff4a4a; padding:10px; font-size:11px; text-align:center;">数据拉取失败: ${err.message}</div>`;
  }
}

async function deleteSavedRecords(ids, type) {
  try {
    const res = await fetch(`${API_BASE}/lottery/delete`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ ids })
    });
    const data = await res.json();
    if (data.status === "ok") {
      if (type === "single") {
        await showSingleSaved();
      } else {
        await showParlaySaved();
      }
    } else {
      alert("删除失败: " + (data.error || "未知错误"));
    }
  } catch (err) {
    alert("删除出错: " + err.message);
  }
}

let singleChartInstance = null;
let parlayChartInstance = null;

async function showSingleHistory() {
  const modal = document.getElementById("single-history-modal");
  const listDom = document.getElementById("single-history-list");
  listDom.innerHTML = `<div style="text-align:center; padding:10px; color:var(--neon-green); font-size:11px;">加载历史数据...</div>`;
  modal.style.display = "flex";
  try {
    const res = await fetch(`${API_BASE}/lottery/history`);
    const data = await res.json();
    const history = (data.history || []).filter(h => h.planType === "single").sort((a, b) => a.id - b.id);
    const N = history.length;
    document.getElementById("single-stat-total").innerText = `${N} 场`;
    if (N === 0) {
      document.getElementById("single-stat-primary-rate").innerText = "0.0%";
      document.getElementById("single-stat-cover-rate").innerText = "0.0%";
      listDom.innerHTML = `<div style="text-align:center; padding:20px; color:var(--text-muted); font-size:11px;">暂无已结算方案</div>`;
      return;
    }
    const primaryHits = history.filter(h => h.primaryHit).length;
    const coverHits = history.filter(h => h.primaryHit || h.hedgeHit).length;
    document.getElementById("single-stat-primary-rate").innerText = `${((primaryHits / N) * 100).toFixed(1)}%`;
    document.getElementById("single-stat-cover-rate").innerText = `${((coverHits / N) * 100).toFixed(1)}%`;

    const chartDom = document.getElementById("single-history-chart");
    if (singleChartInstance) singleChartInstance.dispose();
    singleChartInstance = echarts.init(chartDom);
    const xData = history.map((_, idx) => `#${idx + 1}`);
    let runningPrimary = 0, runningCover = 0;
    const yPrimary = [], yCover = [];
    history.forEach((h, idx) => {
      if (h.primaryHit) runningPrimary++;
      if (h.primaryHit || h.hedgeHit) runningCover++;
      yPrimary.push(((runningPrimary / (idx + 1)) * 100).toFixed(1));
      yCover.push(((runningCover / (idx + 1)) * 100).toFixed(1));
    });
    singleChartInstance.setOption({
      tooltip: { trigger: 'axis', formatter: '{b}<br/>主推精度: {c0}%<br/>覆盖率: {c1}%' },
      grid: { left: '10%', right: '5%', top: '15%', bottom: '15%' },
      xAxis: { type: 'category', data: xData, axisLine: { lineStyle: { color: 'rgba(255,255,255,0.2)' } } },
      yAxis: { type: 'value', min: 0, max: 100, axisLabel: { formatter: '{value}%' }, splitLine: { lineStyle: { color: 'rgba(255,255,255,0.05)' } } },
      series: [
        { name: '主推命中率', type: 'line', data: yPrimary, itemStyle: { color: 'var(--neon-green)' }, smooth: true },
        { name: '整体覆盖率', type: 'line', data: yCover, itemStyle: { color: '#ffeb3b' }, smooth: true }
      ]
    });

    listDom.innerHTML = history.slice().reverse().map(h => {
      const home = translateTeamName(h.homeTeam);
      const away = translateTeamName(h.awayTeam);
      const primaryBadge = h.primaryHit ? `<span style="color:var(--neon-green);">主推: 🎯 ${h.primaryBet}(中)</span>` : `<span style="color:var(--text-muted);">主推: ❌ ${h.primaryBet}(失)</span>`;
      const hedgeBadge = h.hedgeBet ? (h.hedgeHit ? `<span style="color:#ffeb3b; margin-left:8px;">对冲: 🛡️ ${h.hedgeBet}(中)</span>` : `<span style="color:var(--text-muted); margin-left:8px;">对冲: ❌ ${h.hedgeBet}(失)</span>`) : `<span style="color:var(--text-muted); margin-left:8px;">无对冲</span>`;
      return `
        <div style="background:rgba(255,255,255,0.02); border:1px solid rgba(255,255,255,0.05); border-radius:6px; padding:6px; font-size:11px;">
          <div style="display:flex; justify-content:space-between; margin-bottom:4px;">
            <span><strong>${home} ${h.homeScore}:${h.awayScore} ${away}</strong></span>
          </div>
          <div>${primaryBadge} ${hedgeBadge}</div>
        </div>
      `;
    }).join("");
  } catch (err) {
    listDom.innerHTML = `<div style="color:#ff4a4a; padding:10px; font-size:11px; text-align:center;">数据拉取失败: ${err.message}</div>`;
  }
}

async function showParlayHistory() {
  const modal = document.getElementById("parlay-history-modal");
  const listDom = document.getElementById("parlay-history-list");
  listDom.innerHTML = `<div style="text-align:center; padding:10px; color:var(--neon-purple); font-size:11px;">加载历史数据...</div>`;
  modal.style.display = "flex";
  try {
    const res = await fetch(`${API_BASE}/lottery/history`);
    const data = await res.json();
    const history = (data.history || []).filter(h => h.planType === "parlay").sort((a, b) => a.id - b.id);
    const N = history.length;
    document.getElementById("parlay-stat-total").innerText = `${N} 组`;
    if (N === 0) {
      document.getElementById("parlay-stat-combo-rate").innerText = "0.0%";
      document.getElementById("parlay-stat-leg-rate").innerText = "0.0%";
      listDom.innerHTML = `<div style="text-align:center; padding:20px; color:var(--text-muted); font-size:11px;">暂无已结算方案</div>`;
      return;
    }
    const comboHits = history.filter(h => h.primaryHit).length;
    let totalLegs = 0, hitLegs = 0;
    history.forEach(h => {
      (h.tickets || []).forEach(tk => {
        (tk.legs || []).forEach(leg => {
          totalLegs++;
          if (leg.hit) hitLegs++;
        });
      });
    });
    const avgLegRate = totalLegs > 0 ? (hitLegs / totalLegs) * 100 : 0.0;
    document.getElementById("parlay-stat-combo-rate").innerText = `${((comboHits / N) * 100).toFixed(1)}%`;
    document.getElementById("parlay-stat-leg-rate").innerText = `${avgLegRate.toFixed(1)}%`;

    const chartDom = document.getElementById("parlay-history-chart");
    if (parlayChartInstance) parlayChartInstance.dispose();
    parlayChartInstance = echarts.init(chartDom);
    const xData = history.map((_, idx) => `#${idx + 1}`);
    let runningCombo = 0, runningLegsTotal = 0, runningLegsHit = 0;
    const yCombo = [], yLeg = [];
    history.forEach((h, idx) => {
      if (h.primaryHit) runningCombo++;
      (h.tickets || []).forEach(tk => {
        (tk.legs || []).forEach(leg => {
          runningLegsTotal++;
          if (leg.hit) runningLegsHit++;
        });
      });
      yCombo.push(((runningCombo / (idx + 1)) * 100).toFixed(1));
      yLeg.push(runningLegsTotal > 0 ? ((runningLegsHit / runningLegsTotal) * 100).toFixed(1) : 0.0);
    });

    parlayChartInstance.setOption({
      tooltip: { trigger: 'axis', formatter: '{b}<br/>组合中奖率: {c0}%<br/>Leg命中率: {c1}%' },
      grid: { left: '10%', right: '5%', top: '15%', bottom: '15%' },
      xAxis: { type: 'category', data: xData, axisLine: { lineStyle: { color: 'rgba(255,255,255,0.2)' } } },
      yAxis: { type: 'value', min: 0, max: 100, axisLabel: { formatter: '{value}%' }, splitLine: { lineStyle: { color: 'rgba(255,255,255,0.05)' } } },
      series: [
        { name: '组合整体命中率', type: 'line', data: yCombo, itemStyle: { color: '#ff0088' }, smooth: true },
        { name: 'Leg预测平均命中率', type: 'line', data: yLeg, itemStyle: { color: '#ff9800' }, smooth: true }
      ]
    });

    listDom.innerHTML = history.slice().reverse().map(h => {
      const comboStatus = h.primaryHit ? `<span style="color:var(--neon-green); font-weight:800;">🎯 组合中奖</span>` : `<span style="color:var(--text-muted);">❌ 组合失效</span>`;
      const uniqueLegsMap = {};
      (h.tickets || []).forEach(tk => {
        (tk.legs || []).forEach(leg => {
          const key = `${leg.matchId}_${leg.option}`;
          uniqueLegsMap[key] = leg;
        });
      });
      const legsHtml = Object.values(uniqueLegsMap).map(leg => {
        const homeCn = translateTeamName(leg.homeTeam || "");
        const awayCn = translateTeamName(leg.awayTeam || "");
        const hitText = leg.hit ? `<span style="color:var(--neon-green);">中</span>` : `<span style="color:var(--text-muted);">失</span>`;
        return `
          <div style="color:var(--text-muted); padding:2px 0; border-bottom:1px dashed rgba(255,255,255,0.02);">
            • ${homeCn} ${leg.homeScore}:${leg.awayScore} ${awayCn} | 预测: ${leg.option} (${hitText})
          </div>
        `;
      }).join("");

      return `
        <div style="background:rgba(255,255,255,0.02); border:1px solid rgba(255,255,255,0.05); border-radius:6px; padding:6px; font-size:11px; display:flex; flex-direction:column; gap:4px;">
          <div style="display:flex; justify-content:space-between; align-items:center;">
            <span><strong>${h.homeTeam} (${h.awayTeam})</strong></span>
            <span>${comboStatus}</span>
          </div>
          <div style="padding-left:6px; border-left:2px solid var(--neon-purple); display:flex; flex-direction:column; gap:2px;">
            ${legsHtml}
          </div>
        </div>
      `;
    }).join("");
  } catch (err) {
    listDom.innerHTML = `<div style="color:#ff4a4a; padding:10px; font-size:11px; text-align:center;">数据拉取失败: ${err.message}</div>`;
  }
}

// 绑定所有的复盘与关闭交互事件
window.addEventListener("DOMContentLoaded", () => {
  initThemeSwitcher();
  // 绑定历史记录查看按钮
  const historySingleBtn = document.getElementById("history-single-btn");
  if (historySingleBtn) {
    historySingleBtn.onclick = () => showSingleSaved();
  }

  const historyParlayBtn = document.getElementById("history-parlay-btn");
  if (historyParlayBtn) {
    historyParlayBtn.onclick = () => showParlaySaved();
  }

  // 绑定智能多场过关复盘按钮
  const settleParlayBtn = document.getElementById("settle-parlay-btn");
  if (settleParlayBtn) {
    settleParlayBtn.onclick = async () => {
      const oldText = settleParlayBtn.innerText;
      settleParlayBtn.innerText = "复盘中...";
      settleParlayBtn.disabled = true;
      try {
        const res = await fetch(`${API_BASE}/lottery/settle`, { method: "POST" });
        const resData = await res.json();
        if (resData.status === "success") {
          await showParlayHistory();
        } else {
          alert("复盘结算失败: " + (resData.error || "未知错误"));
        }
      } catch (err) {
        alert("复盘网络异常: " + err.message);
      } finally {
        settleParlayBtn.innerText = oldText;
        settleParlayBtn.disabled = false;
      }
    };
  }

  // 绑定单场复盘按钮
  const settleSingleBtn = document.getElementById("settle-single-btn");
  if (settleSingleBtn) {
    settleSingleBtn.onclick = async () => {
      const oldText = settleSingleBtn.innerText;
      settleSingleBtn.innerText = "复盘中...";
      settleSingleBtn.disabled = true;
      try {
        const res = await fetch(`${API_BASE}/lottery/settle`, { method: "POST" });
        const resData = await res.json();
        if (resData.status === "success") {
          await showSingleHistory();
        } else {
          alert("复盘结算失败: " + (resData.error || "未知错误"));
        }
      } catch (err) {
        alert("复盘网络异常: " + err.message);
      } finally {
        settleSingleBtn.innerText = oldText;
        settleSingleBtn.disabled = false;
      }
    };
  }

  // 单场历史弹窗关闭逻辑
  const closeSingleBtn = document.getElementById("close-single-history-btn");
  if (closeSingleBtn) {
    closeSingleBtn.onclick = () => {
      document.getElementById("single-history-modal").style.display = "none";
    };
  }
  const confirmSingleBtn = document.getElementById("confirm-single-history-btn");
  if (confirmSingleBtn) {
    confirmSingleBtn.onclick = () => {
      document.getElementById("single-history-modal").style.display = "none";
    };
  }

  // 过关历史弹窗关闭逻辑
  const closeParlayBtn = document.getElementById("close-parlay-history-btn");
  if (closeParlayBtn) {
    closeParlayBtn.onclick = () => {
      document.getElementById("parlay-history-modal").style.display = "none";
    };
  }
  const confirmParlayBtn = document.getElementById("confirm-parlay-history-btn");
  if (confirmParlayBtn) {
    confirmParlayBtn.onclick = () => {
      document.getElementById("parlay-history-modal").style.display = "none";
    };
  }

  // 点击背景遮罩关闭
  const singleModal = document.getElementById("single-history-modal");
  const parlayModal = document.getElementById("parlay-history-modal");
  const singleSavedModal = document.getElementById("single-saved-modal");
  const parlaySavedModal = document.getElementById("parlay-saved-modal");
  
  // 新增已保存弹窗关闭逻辑
  const closeSingleSaved = document.getElementById("close-single-saved-btn");
  if (closeSingleSaved) {
    closeSingleSaved.onclick = () => { singleSavedModal.style.display = "none"; };
  }
  const confirmSingleSaved = document.getElementById("confirm-single-saved-btn");
  if (confirmSingleSaved) {
    confirmSingleSaved.onclick = () => { singleSavedModal.style.display = "none"; };
  }
  const closeParlaySaved = document.getElementById("close-parlay-saved-btn");
  if (closeParlaySaved) {
    closeParlaySaved.onclick = () => { parlaySavedModal.style.display = "none"; };
  }
  const confirmParlaySaved = document.getElementById("confirm-parlay-saved-btn");
  if (confirmParlaySaved) {
    confirmParlaySaved.onclick = () => { parlaySavedModal.style.display = "none"; };
  }

  window.addEventListener("click", (event) => {
    if (event.target === singleModal) {
      singleModal.style.display = "none";
    }
    if (event.target === parlayModal) {
      parlayModal.style.display = "none";
    }
    if (event.target === singleSavedModal) {
      singleSavedModal.style.display = "none";
    }
    if (event.target === parlaySavedModal) {
      parlaySavedModal.style.display = "none";
    }
  });

  // 绑定全选与批量删除
  const singleSelectAll = document.getElementById("single-saved-select-all");
  if (singleSelectAll) {
    singleSelectAll.onchange = () => {
      const chks = document.querySelectorAll(".saved-item-chk-single");
      chks.forEach(chk => chk.checked = singleSelectAll.checked);
    };
  }

  const singleDeleteBatch = document.getElementById("single-saved-delete-batch-btn");
  if (singleDeleteBatch) {
    singleDeleteBatch.onclick = async () => {
      const chks = document.querySelectorAll(".saved-item-chk-single:checked");
      if (chks.length === 0) {
        alert("请先选择要删除的历史方案！");
        return;
      }
      const ids = Array.from(chks).map(chk => parseInt(chk.getAttribute("data-id")));
      if (await customConfirm(`确定要删除选中的 ${ids.length} 个方案记录吗？`)) {
        await deleteSavedRecords(ids, "single");
      }
    };
  }

  const parlaySelectAll = document.getElementById("parlay-saved-select-all");
  if (parlaySelectAll) {
    parlaySelectAll.onchange = () => {
      const chks = document.querySelectorAll(".saved-item-chk-parlay");
      chks.forEach(chk => chk.checked = parlaySelectAll.checked);
    };
  }

  const parlayDeleteBatch = document.getElementById("parlay-saved-delete-batch-btn");
  if (parlayDeleteBatch) {
    parlayDeleteBatch.onclick = async () => {
      const chks = document.querySelectorAll(".saved-item-chk-parlay:checked");
      if (chks.length === 0) {
        alert("请先选择要删除的历史方案！");
        return;
      }
      const ids = Array.from(chks).map(chk => parseInt(chk.getAttribute("data-id")));
      if (await customConfirm(`确定要删除选中的 ${ids.length} 个方案记录吗？`)) {
        await deleteSavedRecords(ids, "parlay");
      }
    };
  }

  // 保留全自动端到端（E2E）精算测试调试钩子
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

// ==================== 智能对话组件逻辑开始 ====================
let isAIChatOpen = false;

// 重置并初始化当前比赛的 AI 智能对话框
function resetAIChatWindow() {
  isAIChatOpen = false;
  const cot = document.getElementById("prediction-cot-panel");
  if (cot) cot.classList.remove("cot-collapsed");
  const predPanel = document.getElementById("prediction-panel");
  if (predPanel) predPanel.classList.remove("collapsed-layout");
  const collapsedTrigger = document.getElementById("ai-chat-collapsed-trigger");
  if (collapsedTrigger) collapsedTrigger.style.display = "flex";
  const containerDom = document.getElementById("ai-chat-container");
  if (containerDom) containerDom.style.display = "none";

  const historyDom = document.getElementById("ai-chat-history");
  if (historyDom) {
    const localHist = localStorage.getItem("ai_chat_history");
    if (localHist) {
      historyDom.innerHTML = localHist;
    } else {
      const matchInfo = matchesMap[currentMatchID];
      const homeCn = matchInfo ? translateTeamNameText(matchInfo.homeTeam) : "主队";
      const awayCn = matchInfo ? translateTeamNameText(matchInfo.awayTeam) : "客队";
      historyDom.innerHTML = `
        <div style="background: rgba(136,0,255,0.06); border-left: 2px solid var(--neon-purple); padding: 6px 10px; border-radius: 4px; color: var(--text-main); line-height: 1.4; align-self: flex-start; max-width: 85%;">
          你好！我是全能智能决策与通用精算助手。已为你锁定 <strong>${homeCn} vs ${awayCn}</strong> 场次。你可以向我追问关于这场比赛的量化精算、让球风控等细节，也可以向我咨询任何其他足球联赛、科学常识、全球天气或技术编程等通用问题。
        </div>
      `;
    }
  }
  const inputDom = document.getElementById("ai-chat-input");
  if (inputDom) {
    inputDom.value = "";
  }
}

// 绑定智能对话组件相关的交互事件
function initAIChat() {
  const toggleBtn = document.getElementById("ai-chat-toggle-btn");
  const closeBtn = document.getElementById("ai-chat-close-btn");
  const clearBtn = document.getElementById("ai-chat-clear-btn");
  const sendBtn = document.getElementById("ai-chat-send-btn");
  const inputDom = document.getElementById("ai-chat-input");
  const collapsedTrigger = document.getElementById("ai-chat-collapsed-trigger");
  const containerDom = document.getElementById("ai-chat-container");
  const historyDom = document.getElementById("ai-chat-history");

  if (toggleBtn) {
    toggleBtn.onclick = () => {
      isAIChatOpen = true;
      const cot = document.getElementById("prediction-cot-panel");
      if (cot) cot.classList.add("cot-collapsed");
      const predPanel = document.getElementById("prediction-panel");
      if (predPanel) predPanel.classList.add("collapsed-layout");
      if (collapsedTrigger) collapsedTrigger.style.display = "none";
      if (containerDom) containerDom.style.display = "flex";
      // 聚焦输入框，并防止页面自动向下滚动
      if (inputDom) inputDom.focus({ preventScroll: true });
      if (historyDom) historyDom.scrollTop = historyDom.scrollHeight;
    };
  }

  if (closeBtn) {
    closeBtn.onclick = () => {
      isAIChatOpen = false;
      const cot = document.getElementById("prediction-cot-panel");
      if (cot) cot.classList.remove("cot-collapsed");
      const predPanel = document.getElementById("prediction-panel");
      if (predPanel) predPanel.classList.remove("collapsed-layout");
      if (collapsedTrigger) collapsedTrigger.style.display = "flex";
      if (containerDom) containerDom.style.display = "none";
    };
  }

  if (clearBtn) {
    clearBtn.onclick = () => {
      if (confirm("确定要清空所有对话记录吗？")) {
        localStorage.removeItem("ai_chat_history");
        localStorage.removeItem("ai_chat_history_json");
        const matchInfo = matchesMap[currentMatchID];
        const homeCn = matchInfo ? translateTeamNameText(matchInfo.homeTeam) : "主队";
        const awayCn = matchInfo ? translateTeamNameText(matchInfo.awayTeam) : "客队";
        if (historyDom) {
          historyDom.innerHTML = `
            <div style="background: rgba(136,0,255,0.06); border-left: 2px solid var(--neon-purple); padding: 6px 10px; border-radius: 4px; color: var(--text-main); line-height: 1.4; align-self: flex-start; max-width: 85%;">
              你好！我是全能智能决策与通用精算助手。已为你锁定 <strong>${homeCn} vs ${awayCn}</strong> 场次。你可以向我追问关于这场比赛的量化精算、让球风控等细节，也可以向我咨询任何其他足球联赛、科学常识、全球天气或技术编程等通用问题。
            </div>
          `;
        }
      }
    };
  }

  if (sendBtn) {
    sendBtn.onclick = () => {
      handleSendChatMessage();
    };
  }

  if (inputDom) {
    inputDom.onkeydown = (e) => {
      if (e.key === "Enter") {
        handleSendChatMessage();
      }
    };
  }
}

// 处理发送消息
async function handleSendChatMessage() {
  const inputDom = document.getElementById("ai-chat-input");
  const message = inputDom ? inputDom.value.trim() : "";
  if (!message) return;

  appendChatBubble("user", message);
  if (inputDom) inputDom.value = "";

  // 根据用户消息类型显示不同的动感 Loading 提示
  const isLotteryQuery = message.includes("怎么买") || message.includes("推荐买") || message.includes("预算") || message.includes("串一") || message.includes("方案") || message.includes("串");
  const isWeatherQuery = message.includes("天气") || message.includes("气温") || message.includes("下雨") || message.includes("海拔") || message.includes("温度");
  
  let loadingText = "正在思考...";
  if (isLotteryQuery) {
    loadingText = "正在进行过关方案精算...";
  } else if (isWeatherQuery) {
    loadingText = "正在检索相关气象与环境数据...";
  } else {
    loadingText = "正在深度推理与组织回答中...";
  }
  const loadingId = appendChatBubble("ai-loading", loadingText);

  try {
    const checkedChks = document.querySelectorAll(".match-select-chk:checked");
    const checkedMatchIds = Array.from(checkedChks).map(chk => chk.getAttribute("data-match-id") || chk.value);

    // 获取并裁剪历史记录，限制只发送最近的 6 条消息（3 轮对话）
    let chatHistory = [];
    try {
      chatHistory = JSON.parse(localStorage.getItem("ai_chat_history_json") || "[]");
    } catch (e) {
      chatHistory = [];
    }
    if (chatHistory.length > 6) {
      chatHistory = chatHistory.slice(-6);
    }

    const res = await fetch(`${API_BASE}/chat`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        matchId: currentMatchID,
        message: message,
        predictions: currentPredictions,
        checkedMatchIds: checkedMatchIds,
        history: chatHistory
      })
    });
    
    // 移除 Loading 气泡
    removeChatBubble(loadingId);

    if (res.ok) {
      const data = await res.json();
      appendChatBubble("ai", data.reply || "未返回分析。");
    } else {
      appendChatBubble("ai", "服务器返回错误，请稍后再试。");
    }
  } catch (err) {
    removeChatBubble(loadingId);
    appendChatBubble("ai", "无法连接到AI服务器，错误: " + err.message);
  }
}

// 追加对话气泡
function appendChatBubble(sender, text) {
  const historyDom = document.getElementById("ai-chat-history");
  if (!historyDom) return "";

  const bubbleId = "chat-bubble-" + Date.now() + "-" + Math.random().toString(36).substr(2, 5);
  let bg = "rgba(0,255,136,0.04)";
  let border = "1px solid rgba(0,255,136,0.15)";
  let title = "🔍 您";
  let titleColor = "var(--neon-green)";

  if (sender === "ai") {
    bg = "rgba(136,0,255,0.04)";
    border = "1px solid rgba(136,0,255,0.15)";
    title = "🤖 AI 决策助手";
    titleColor = "var(--neon-purple)";
  } else if (sender === "ai-loading") {
    bg = "rgba(255,255,255,0.02)";
    border = "1px dashed rgba(255,255,255,0.1)";
    title = "🤖 AI 决策助手";
    titleColor = "var(--text-muted)";
  }

  let alignSelf = "flex-start";
  if (sender === "user") {
    alignSelf = "flex-end";
  }

  const html = `
    <div id="${bubbleId}" style="background: ${bg}; border: ${border}; padding: 6px 10px; border-radius: 6px; display: flex; flex-direction: column; gap: 2px; align-self: ${alignSelf}; max-width: 85%;">
      <span style="color: ${titleColor}; font-weight: 800; font-size: 10px;">${title}</span>
      <p style="color: var(--text-main); margin: 0; line-height: 1.4; word-break: break-word;">${text}</p>
    </div>
  `;
  historyDom.insertAdjacentHTML("beforeend", html);
  historyDom.scrollTop = historyDom.scrollHeight;
  if (sender === "user" || sender === "ai") {
    localStorage.setItem("ai_chat_history", historyDom.innerHTML);

    // 同步写入结构化历史以供后端进行多轮上下文理解
    let jsonHistory = [];
    try {
      jsonHistory = JSON.parse(localStorage.getItem("ai_chat_history_json") || "[]");
    } catch (e) {
      jsonHistory = [];
    }
    const role = (sender === "user") ? "user" : "assistant";
    jsonHistory.push({ role: role, content: text });
    localStorage.setItem("ai_chat_history_json", JSON.stringify(jsonHistory));
  }
  return bubbleId;
}

// 移除对话气泡 (主要是移除 Loading)
function removeChatBubble(bubbleId) {
  const bubble = document.getElementById(bubbleId);
  if (bubble) bubble.remove();
}
// ==================== 智能对话组件逻辑结束 ====================

// ==================== 全局多主题切换器控制逻辑 ====================
function initThemeSwitcher() {
  const switcher = document.getElementById("global-theme-switcher");
  if (!switcher) return;

  const buttons = switcher.querySelectorAll(".theme-switch-btn");
  
  // 1. 初始化激活状态高亮
  const savedTheme = localStorage.getItem("selected_theme") || "glass";
  buttons.forEach(btn => {
    if (btn.getAttribute("data-theme") === savedTheme) {
      btn.classList.add("active");
    } else {
      btn.classList.remove("active");
    }
  });

  // 2. 绑定点击事件分流
  buttons.forEach(btn => {
    btn.onclick = () => {
      const theme = btn.getAttribute("data-theme");
      
      // 更新高亮类名
      buttons.forEach(b => b.classList.remove("active"));
      btn.classList.add("active");

      // 切换根节点类名并写缓存
      document.documentElement.className = "theme-" + theme;
      localStorage.setItem("selected_theme", theme);

      // 3. 自动触发 ECharts 重新加载与网格对比度重绘
      if (typeof initSimulationChart === "function") {
        initSimulationChart();
      }
      if (typeof initBacktestChart === "function") {
        initBacktestChart();
      }

      console.log(`[Theme] 成功切换至主题: theme-${theme}`);
    };
  });
}

// ==================== 赛程积分表专有交互与计算 ====================
function initScheduleStandings() {
  const btn = document.getElementById("schedule-standings-btn");
  const modal = document.getElementById("schedule-standings-modal");
  const closeBtn = document.getElementById("close-schedule-standings-btn");
  if (!btn || !modal) return;
  btn.onclick = () => {
    modal.style.display = "flex";
    renderScheduleStandings();
  };
  closeBtn.onclick = () => { modal.style.display = "none"; };
  const tabBtns = modal.querySelectorAll(".modal-tab-btn");
  tabBtns.forEach(tabBtn => {
    tabBtn.onclick = () => {
      tabBtns.forEach(b => {
        b.classList.remove("active");
        b.style.color = "var(--text-muted)";
        b.style.borderBottomColor = "transparent";
      });
      tabBtn.classList.add("active");
      tabBtn.style.color = "var(--neon-green)";
      tabBtn.style.borderBottomColor = "var(--neon-green)";
      const targetTab = tabBtn.getAttribute("data-tab");
      modal.querySelectorAll(".tab-content").forEach(content => {
        content.style.display = "none";
      });
      document.getElementById("tab-" + targetTab).style.display = "block";
    };
  });
}

function renderScheduleStandings() {
  if (!allMatchesData || allMatchesData.length === 0) return;
  const groups = ["A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L"];
  const standings = {};
  groups.forEach(g => { standings[g] = {}; });
  allMatchesData.forEach(m => {
    if (m.group && groups.includes(m.group)) {
      const g = m.group;
      if (!standings[g][m.homeTeam]) {
        standings[g][m.homeTeam] = { name: m.homeTeam, played: 0, wins: 0, draws: 0, losses: 0, goalsFor: 0, goalsAgainst: 0, goalDiff: 0, points: 0 };
      }
      if (!standings[g][m.awayTeam]) {
        standings[g][m.awayTeam] = { name: m.awayTeam, played: 0, wins: 0, draws: 0, losses: 0, goalsFor: 0, goalsAgainst: 0, goalDiff: 0, points: 0 };
      }
      if (m.status === "FT") {
        const h = standings[g][m.homeTeam], a = standings[g][m.awayTeam];
        h.played++; a.played++;
        h.goalsFor += m.homeScore; h.goalsAgainst += m.awayScore;
        a.goalsFor += m.awayScore; a.goalsAgainst += m.homeScore;
        if (m.homeScore > m.awayScore) {
          h.wins++; a.losses++; h.points += 3;
        } else if (m.homeScore === m.awayScore) {
          h.draws++; a.draws++; h.points += 1; a.points += 1;
        } else {
          h.losses++; a.wins++; a.points += 3;
        }
      }
    }
  });
  const groupsContainer = document.getElementById("groups-container");
  groupsContainer.innerHTML = "";
  groups.forEach(g => {
    const teams = Object.values(standings[g]);
    if (teams.length === 0) {
      const teamSet = new Set();
      allMatchesData.forEach(m => { if (m.group === g) { teamSet.add(m.homeTeam); teamSet.add(m.awayTeam); } });
      teamSet.forEach(tName => { teams.push({ name: tName, played: 0, wins: 0, draws: 0, losses: 0, goalsFor: 0, goalsAgainst: 0, goalDiff: 0, points: 0 }); });
    }
    teams.forEach(t => { t.goalDiff = t.goalsFor - t.goalsAgainst; });
    teams.sort((x, y) => {
      if (y.points !== x.points) return y.points - x.points;
      if (y.goalDiff !== x.goalDiff) return y.goalDiff - x.goalDiff;
      return y.goalsFor - x.goalsFor;
    });
    const groupMatches = allMatchesData.filter(m => m.group === g);
    const card = document.createElement("div");
    card.className = "group-card";
    let tableRows = "";
    teams.forEach((t, idx) => {
      const isTop2 = idx < 2 ? "class='top-2'" : "";
      tableRows += `
        <tr ${isTop2}>
          <td>${idx + 1}</td>
          <td class="team-name" title="${translateTeamNameText(t.name)}">${translateTeamName(t.name)}</td>
          <td>${t.played}</td>
          <td>${t.wins}/${t.draws}/${t.losses}</td>
          <td>${t.goalsFor}/${t.goalsAgainst}</td>
          <td>${t.goalDiff >= 0 ? "+" + t.goalDiff : t.goalDiff}</td>
          <td style="font-weight: 800;">${t.points}</td>
        </tr>
      `;
    });
    let matchesRows = "";
    groupMatches.forEach(gm => {
      const date = new Date(gm.scheduledAt);
      const timeStr = date.toLocaleString('zh-CN', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' });
      const homeCn = translateTeamName(gm.homeTeam);
      const awayCn = translateTeamName(gm.awayTeam);
      let statusOrScore = gm.status === "FT" ? `<span class="match-score">${gm.homeScore} - ${gm.awayScore}</span>` : (gm.status === "Live" ? `<span class="match-score" style="color: red; animation: blink 1s infinite;">${gm.homeScore} - ${gm.awayScore} (直播)</span>` : `<span class="match-time">${timeStr}</span>`);
      matchesRows += `
        <div class="group-match-row">
          <span style="max-width: 200px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">${homeCn} vs ${awayCn}</span>
          ${statusOrScore}
        </div>
      `;
    });
    card.innerHTML = `
      <h4>Group ${g} (${g}组)</h4>
      <table class="group-table">
        <thead>
          <tr><th>#</th><th style="text-align: left;">球队</th><th>赛</th><th>胜/平/负</th><th>得/失</th><th>净</th><th>分</th></tr>
        </thead>
        <tbody>${tableRows}</tbody>
      </table>
      <div style="font-size: 11px; font-weight: 800; color: var(--text-muted); border-top: 1px solid rgba(255, 255, 255, 0.04); padding-top: 6px; margin-top: 4px;">📅 小组赛程对阵</div>
      <div class="group-matches-list">${matchesRows}</div>
    `;
    groupsContainer.appendChild(card);
  });
  renderBracketTree();
}

function renderBracketTree() {
  const bracketContainer = document.getElementById("bracket-container");
  bracketContainer.innerHTML = "";
  const columnsConfig = [
    { title: "1/16 决赛", matches: [73, 74, 75, 76, 77, 78, 79, 80] },
    { title: "1/8 决赛", matches: [89, 90, 91, 92] },
    { title: "1/4 决赛", matches: [97, 98] },
    { title: "半决赛", matches: [101] },
    { title: "决赛 & 季军赛", matches: [104, 103], isCenter: true },
    { title: "半决赛", matches: [102] },
    { title: "1/4 决赛", matches: [99, 100] },
    { title: "1/8 决赛", matches: [93, 94, 95, 96] },
    { title: "1/16 决赛", matches: [81, 82, 83, 84, 85, 86, 87, 88] }
  ];
  const knockoutPlaceholders = {
    "wc2026_m73": { home: "A组第二", away: "B组第二" },
    "wc2026_m74": { home: "E组第一", away: "A/B/C/D/F组第三" },
    "wc2026_m75": { home: "F组第一", away: "C组第二" },
    "wc2026_m76": { home: "C组第一", away: "F组第二" },
    "wc2026_m77": { home: "I组第一", away: "C/D/F/G/H组第三" },
    "wc2026_m78": { home: "E组第二", away: "I组第二" },
    "wc2026_m79": { home: "A组第一", away: "C/E/F/H/I组第三" },
    "wc2026_m80": { home: "L组第一", away: "E/H/I/J/K组第三" },
    "wc2026_m81": { home: "D组第一", away: "B/E/F/I/J组第三" },
    "wc2026_m82": { home: "G组第一", away: "A/E/H/I/J组第三" },
    "wc2026_m83": { home: "K组第二", away: "L组第二" },
    "wc2026_m84": { home: "H组第一", away: "J组第二" },
    "wc2026_m85": { home: "B组第一", away: "E/F/G/I/J组第三" },
    "wc2026_m86": { home: "J组第一", away: "H组第二" },
    "wc2026_m87": { home: "K组第一", away: "D/E/I/J/L组第三" },
    "wc2026_m88": { home: "D组第二", away: "G组第二" },
    "wc2026_m89": { home: "74场胜者", away: "77场胜者" },
    "wc2026_m90": { home: "73场胜者", away: "75场胜者" },
    "wc2026_m91": { home: "76场胜者", away: "78场胜者" },
    "wc2026_m92": { home: "79场胜者", away: "80场胜者" },
    "wc2026_m93": { home: "83场胜者", away: "84场胜者" },
    "wc2026_m94": { home: "81场胜者", away: "82场胜者" },
    "wc2026_m95": { home: "86场胜者", away: "88场胜者" },
    "wc2026_m96": { home: "85场胜者", away: "87场胜者" },
    "wc2026_m97": { home: "89场胜者", away: "90场胜者" },
    "wc2026_m98": { home: "93场胜者", away: "94场胜者" },
    "wc2026_m99": { home: "91场胜者", away: "92场胜者" },
    "wc2026_m100": { home: "95场胜者", away: "96场胜者" },
    "wc2026_m101": { home: "97场胜者", away: "98场胜者" },
    "wc2026_m102": { home: "99场胜者", away: "100场胜者" },
    "wc2026_m103": { home: "101场败者", away: "102场败者" },
    "wc2026_m104": { home: "101场胜者", away: "102场胜者" }
  };

  columnsConfig.forEach(cfg => {
    const colDom = document.createElement("div");
    colDom.className = cfg.isCenter ? "bracket-column center-column" : "bracket-column";
    
    if (cfg.isCenter) {
      const trophy = document.createElement("div");
      trophy.className = "trophy-container";
      trophy.innerHTML = `
        <img class="trophy-image" src="/static/WorldCup.webp" alt="大力神杯">
      `;
      colDom.appendChild(trophy);
    }

    const titleDom = document.createElement("div");
    titleDom.className = "bracket-round-title";
    titleDom.innerText = cfg.title;
    colDom.appendChild(titleDom);

    cfg.matches.forEach(mNum => {
      const mId = "wc2026_m" + mNum;
      const m = allMatchesData.find(x => x.id === mId);
      if (!m) return;

      const isFinal = mNum === 104;
      const is3rd = mNum === 103;

      const card = document.createElement("div");
      card.className = isFinal ? "bracket-match-card final-match" : "bracket-match-card";
      
      const homePl = knockoutPlaceholders[mId]?.home || "等待晋级";
      const awayPl = knockoutPlaceholders[mId]?.away || "等待晋级";

      const hasHome = m.homeTeam !== "0" && m.homeTeam !== "";
      const homeName = hasHome ? translateTeamName(m.homeTeam) : homePl;
      const homeClass = !hasHome ? "bracket-team-line placeholder" : (m.status === "FT" && m.homeScore > m.awayScore ? "bracket-team-line winner" : (m.status === "FT" && m.homeScore < m.awayScore ? "bracket-team-line loser" : "bracket-team-line"));

      const hasAway = m.awayTeam !== "0" && m.awayTeam !== "";
      const awayName = hasAway ? translateTeamName(m.awayTeam) : awayPl;
      const awayClass = !hasAway ? "bracket-team-line placeholder" : (m.status === "FT" && m.awayScore > m.homeScore ? "bracket-team-line winner" : (m.status === "FT" && m.awayScore < m.homeScore ? "bracket-team-line loser" : "bracket-team-line"));

      const scoreHtml = m.status === "FT" || m.status === "Live" 
        ? { home: m.homeScore, away: m.awayScore }
        : { home: "-", away: "-" };

      const timeLabel = new Date(m.scheduledAt).toLocaleString('zh-CN', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' });
      const statusLabel = m.status === "FT" ? "已完赛" : (m.status === "Live" ? "直播中" : timeLabel);
      const statusColor = m.status === "Live" ? "color: red;" : "";

      const matchLabel = isFinal ? "决赛" : (is3rd ? "季军赛" : `场次 ${mNum}`);

      card.innerHTML = `
        <div class="bracket-match-info">
          <span>${matchLabel}</span>
          <span style="${statusColor}">${statusLabel}</span>
        </div>
        <div class="${homeClass}">
          <span style="white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 75px;">${homeName}</span>
          <span class="bracket-team-score">${scoreHtml.home}</span>
        </div>
        <div class="${awayClass}">
          <span style="white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 75px;">${awayName}</span>
          <span class="bracket-team-score">${scoreHtml.away}</span>
        </div>
      `;
      colDom.appendChild(card);
    });

    bracketContainer.appendChild(colDom);
  });
}


