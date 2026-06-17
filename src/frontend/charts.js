// charts.js - 用于前端大屏的数据可视化图表渲染
let simulationChart = null;

// 初始化蒙特卡洛仿真图表
function initSimulationChart() {
  const chartDom = document.getElementById('simulation-chart');
  if (!chartDom) return;
  
  // 销毁旧实例以防内存泄漏
  if (simulationChart) {
    simulationChart.dispose();
  }
  
  const savedTheme = localStorage.getItem('selected_theme') || 'dark';
  const isLight = savedTheme === 'light';
  
  simulationChart = echarts.init(chartDom, isLight ? null : 'dark');
  
  const splitLineColor = isLight ? 'rgba(0, 0, 0, 0.08)' : 'rgba(255, 255, 255, 0.05)';
  const textColor = isLight ? '#334155' : '#8b9bb4';
  
  const option = {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' }
    },
    grid: {
      top: '10px',
      left: '3%',
      right: '4%',
      bottom: '3%',
      containLabel: true
    },
    xAxis: {
      type: 'value',
      axisLine: { show: false },
      axisTick: { show: false },
      splitLine: { lineStyle: { color: splitLineColor } },
      axisLabel: { color: textColor, fontSize: 10 }
    },
    yAxis: {
      type: 'category',
      inverse: true,
      data: ['巴西', '法国', '西班牙', '阿根廷', '德国', '英格兰', '葡萄牙', '荷兰'],
      axisLine: { show: false },
      axisTick: { show: false },
      axisLabel: { color: textColor, fontSize: 10 }
    },
    series: [
      {
        name: '夺冠概率 %',
        type: 'bar',
        data: [15.2, 12.8, 10.5, 9.8, 8.4, 7.9, 6.2, 5.8],
        itemStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 1, 0, [
            { offset: 0, color: '#8800ff' },
            { offset: 1, color: '#00ff88' }
          ]),
          borderRadius: 4
        },
        label: {
          show: true,
          position: 'right',
          formatter: '{c}%',
          color: isLight ? '#00b368' : '#00ff88',
          fontSize: 10,
          fontWeight: 'bold'
        }
      }
    ]
  };
  
  simulationChart.setOption(option);
  window.addEventListener('resize', () => simulationChart && simulationChart.resize());
}

// 动态更新蒙特卡洛仿真数据
function updateSimulationChart(results) {
  if (!simulationChart) return;
  
  // 只截取前 10 名显示，利用 yAxis.inverse 自动居于图表最上方
  const topResults = results.slice(0, 10);
  
  const yData = topResults.map(r => translateTeamNameText(r.teamName));
  const xData = topResults.map(r => parseFloat(r.winnerProb.toFixed(2)));
  
  simulationChart.setOption({
    yAxis: { data: yData },
    series: [{ data: xData }]
  });

  // 强力防缩水自适应：延迟触发 ECharts 重绘，确保在 DOM 排版彻底稳定后拉伸至真实物理宽度
  setTimeout(() => {
    if (simulationChart) {
      simulationChart.resize();
    }
  }, 200);
}

// 国家英文名到 FlagCDN 二位码的映射字典 (包含英伦三岛的特异性二位码)
const countryCodes = {
  "Brazil": "br",
  "Argentina": "ar",
  "France": "fr",
  "Germany": "de",
  "Spain": "es",
  "England": "gb-eng",
  "Italy": "it",
  "Netherlands": "nl",
  "Portugal": "pt",
  "Croatia": "hr",
  "Japan": "jp",
  "USA": "us",
  "United States": "us",
  "Mexico": "mx",
  "Ecuador": "ec",
  "Venezuela": "ve",
  "Jamaica": "jm",
  "Iran": "ir",
  "Wales": "gb-wls",
  "Saudi Arabia": "sa",
  "Poland": "pl",
  "Australia": "au",
  "Denmark": "dk",
  "Tunisia": "tn",
  "Costa Rica": "cr",
  "Belgium": "be",
  "Canada": "ca",
  "Morocco": "ma",
  "Serbia": "rs",
  "Switzerland": "ch",
  "Cameroon": "cm",
  "Ghana": "gh",
  "Uruguay": "uy",
  "South Korea": "kr",
  "Colombia": "co",
  "Algeria": "dz",
  "Chile": "cl",
  "Nigeria": "ng",
  "Scotland": "gb-sct",
  "Hungary": "hu",
  "Panama": "pa",
  "Bolivia": "bo",
  "Peru": "pe",
  "South Africa": "za",
  "Czech Republic": "cz",
  "Bosnia and Herzegovina": "ba",
  "Paraguay": "py",
  "Qatar": "qa",
  "Haiti": "ht",
  "Turkey": "tr",
  "Curaçao": "cw",
  "Ivory Coast": "ci",
  "Sweden": "se",
  "Egypt": "eg",
  "New Zealand": "nz",
  "Cape Verde": "cv",
  "Senegal": "sn",
  "Iraq": "iq",
  "Norway": "no",
  "Austria": "at",
  "Jordan": "jo",
  "Democratic Republic of the Congo": "cd",
  "Uzbekistan": "uz"
};

// 获取团队国旗的 HTML 标签，用于跨平台 (如 Windows) 的优雅降级渲染
function getTeamFlagHtml(enName) {
  const code = countryCodes[enName];
  if (!code) return "";
  return `<img src="https://flagcdn.com/w20/${code}.png" class="team-flag" alt="${enName}" style="width: 20px; height: auto; vertical-align: middle; margin-right: 4px; border-radius: 2px; box-shadow: 0 1px 3px rgba(0,0,0,0.2);">`;
}

// 翻译纯中文名称，防止在 ECharts 等不支持 HTML 标签的组件中溢出
function translateTeamNameText(enName) {
  const dict = {
    "Brazil": "巴西",
    "Argentina": "阿根廷",
    "France": "法国",
    "Germany": "德国",
    "Spain": "西班牙",
    "England": "英格兰",
    "Italy": "意大利",
    "Netherlands": "荷兰",
    "Portugal": "葡萄牙",
    "Croatia": "克罗地亚",
    "Japan": "日本",
    "USA": "美国",
    "United States": "美国",
    "Mexico": "墨西哥",
    "Ecuador": "厄瓜多尔",
    "Venezuela": "委内瑞拉",
    "Jamaica": "牙买加",
    "Iran": "伊朗",
    "Wales": "威尔士",
    "Saudi Arabia": "沙特阿拉伯",
    "Poland": "波兰",
    "Australia": "澳大利亚",
    "Denmark": "丹麦",
    "Tunisia": "突尼斯",
    "Costa Rica": "哥斯达黎加",
    "Belgium": "比利时",
    "Canada": "加拿大",
    "Morocco": "摩洛哥",
    "Serbia": "塞尔维亚",
    "Switzerland": "瑞士",
    "Cameroon": "喀麦隆",
    "Ghana": "加纳",
    "Uruguay": "乌拉圭",
    "South Korea": "韩国",
    "Colombia": "哥伦比亚",
    "Algeria": "阿尔及利亚",
    "Chile": "智利",
    "Nigeria": "尼日利亚",
    "Scotland": "苏格兰",
    "Hungary": "匈牙利",
    "Panama": "巴拿马",
    "Bolivia": "玻利维亚",
    "Peru": "秘鲁",
    "South Africa": "南非",
    "Czech Republic": "捷克",
    "Bosnia and Herzegovina": "波黑",
    "Paraguay": "巴拉圭",
    "Qatar": "卡塔尔",
    "Haiti": "海地",
    "Turkey": "土耳其",
    "Curaçao": "库拉索",
    "Ivory Coast": "科特迪瓦",
    "Sweden": "瑞典",
    "Egypt": "埃及",
    "New Zealand": "新西兰",
    "Cape Verde": "佛得角",
    "Senegal": "塞内加尔",
    "Iraq": "伊拉克",
    "Norway": "挪威",
    "Austria": "奥地利",
    "Jordan": "约旦",
    "Democratic Republic of the Congo": "刚果金",
    "Uzbekistan": "乌兹别克斯坦"
  };
  return dict[enName] || enName;
}

// 带国旗 HTML 标签的团队翻译函数，用于 innerHTML 渲染
function translateTeamName(enName) {
  const flagHtml = getTeamFlagHtml(enName);
  const cnName = translateTeamNameText(enName);
  if (!flagHtml) return cnName;
  return `${flagHtml}${cnName}`;
}


// 页面加载完毕后默认初始化
document.addEventListener('DOMContentLoaded', () => {
  initSimulationChart();
  initBacktestChart();
});

let backtestChart = null;

function initBacktestChart() {
  const chartDom = document.getElementById('backtest-chart');
  if (!chartDom) return;
  
  if (backtestChart) {
    backtestChart.dispose();
  }
  
  const savedTheme = localStorage.getItem('selected_theme') || 'dark';
  const isLight = savedTheme === 'light';
  
  backtestChart = echarts.init(chartDom, isLight ? null : 'dark');
  
  const splitLineColor = isLight ? 'rgba(0, 0, 0, 0.08)' : 'rgba(255, 255, 255, 0.05)';
  const textColor = isLight ? '#334155' : '#8b9bb4';
  
  const option = {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'axis',
      formatter: '{b}<br/>Brier Score: <strong>{c}</strong> (越接近0预测越准)'
    },
    grid: {
      top: '15px',
      left: '3%',
      right: '4%',
      bottom: '30px',
      containLabel: true
    },
    xAxis: {
      type: 'category',
      data: [],
      axisLine: { show: false },
      axisTick: { show: false },
      axisLabel: { color: textColor, fontSize: 10 }
    },
    yAxis: {
      type: 'value',
      min: 0,
      max: 2.0,
      axisLine: { show: false },
      splitLine: { lineStyle: { color: splitLineColor } },
      axisLabel: { color: textColor, fontSize: 10 }
    },
    series: [
      {
        name: 'Brier Score',
        type: 'line',
        smooth: true,
        data: [],
        lineStyle: {
          color: isLight ? '#7a00e6' : '#8800ff',
          width: 2
        },
        itemStyle: {
          color: isLight ? '#00b368' : '#00ff88'
        },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: isLight ? 'rgba(122,0,230,0.18)' : 'rgba(136,0,255,0.25)' },
            { offset: 1, color: isLight ? 'rgba(122,0,230,0)' : 'rgba(136,0,255,0)' }
          ])
        }
      }
    ]
  };
  
  backtestChart.setOption(option);
  window.addEventListener('resize', () => backtestChart && backtestChart.resize());
}

function updateBacktestChart(history) {
  if (!backtestChart) return;
  const xData = history.map((h, i) => `G${i+1}`);
  const yData = history.map(h => parseFloat(h.brierScore.toFixed(3)));
  
  backtestChart.setOption({
    xAxis: {
      type: 'category',
      data: xData
    },
    series: [
      {
        name: 'Brier Score',
        type: 'line',
        data: yData
      }
    ]
  });

  // 强力防缩水自适应：延迟触发 ECharts 重绘，确保在 DOM 排版彻底稳定后拉伸至真实物理宽度
  setTimeout(() => {
    if (backtestChart) {
      backtestChart.resize();
    }
  }, 200);
}
