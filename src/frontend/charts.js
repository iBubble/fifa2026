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
  
  simulationChart = echarts.init(chartDom, 'dark');
  
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
      splitLine: { lineStyle: { color: 'rgba(255,255,255,0.05)' } }
    },
    yAxis: {
      type: 'category',
      data: ['巴西', '法国', '西班牙', '阿根廷', '德国', '英格兰', '葡萄牙', '荷兰'],
      axisLine: { show: false },
      axisTick: { show: false }
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
          color: '#00ff88'
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
  
  // 只截取前 8 名显示，以保证图表美观
  const topResults = results.slice(0, 8).reverse();
  
  const yData = topResults.map(r => translateTeamName(r.teamName));
  const xData = topResults.map(r => parseFloat(r.winnerProb.toFixed(2)));
  
  simulationChart.setOption({
    yAxis: { data: yData },
    series: [{ data: xData }]
  });
}

// 简易的中英国家队名称互译，提升中文体验
function translateTeamName(enName) {
  const dict = {
    "Brazil": "🇧🇷 巴西",
    "Argentina": "🇦🇷 阿根廷",
    "France": "🇫🇷 法国",
    "Germany": "🇩🇪 德国",
    "Spain": "🇪🇸 西班牙",
    "England": "🏴󠁧󠁢󠁥󠁮󠁧󠁿 英格兰",
    "Italy": "🇮🇹 意大利",
    "Netherlands": "🇳🇱 荷兰",
    "Portugal": "🇵🇹 葡萄牙",
    "Croatia": "🇭🇷 克罗地亚",
    "Japan": "🇯🇵 日本",
    "USA": "🇺🇸 美国",
    "Mexico": "🇲🇽 墨西哥",
    "Ecuador": "🇪🇨 厄瓜多尔",
    "Venezuela": "🇻🇪 委内瑞拉",
    "Jamaica": "🇯🇲 牙买加",
    "Iran": "🇮🇷 伊朗",
    "Wales": "🏴󠁧󠁢󠁷󠁬󠁳󠁿 威尔士",
    "Saudi Arabia": "🇸🇦 沙特阿拉伯",
    "Poland": "🇵🇱 波兰",
    "Australia": "🇦🇺 澳大利亚",
    "Denmark": "🇩🇰 丹麦",
    "Tunisia": "🇹🇳 突尼斯",
    "Costa Rica": "🇨🇷 哥斯达黎加",
    "Belgium": "🇧🇪 比利时",
    "Canada": "🇨🇦 加拿大",
    "Morocco": "🇲🇦 摩洛哥",
    "Serbia": "🇷🇸 塞尔维亚",
    "Switzerland": "🇨🇭 瑞士",
    "Cameroon": "🇨🇲 喀麦隆",
    "Ghana": "🇬🇭 加纳",
    "Uruguay": "🇺🇾 乌拉圭",
    "South Korea": "🇰🇷 韩国",
    "Colombia": "🇨🇴 哥伦比亚",
    "Algeria": "🇩🇿 阿尔及利亚",
    "Chile": "🇨🇱 智利",
    "Nigeria": "🇳🇬 尼日利亚",
    "Scotland": "🏴󠁧󠁢󠁳󠁣󠁴󠁿 苏格兰",
    "Hungary": "🇭🇺 匈牙利",
    "Panama": "🇵🇦 巴拿马",
    "Bolivia": "🇧🇴 玻利维亚",
    "Peru": "🇵🇪 秘鲁",
    "South Africa": "🇿🇦 南非",
    "Czech Republic": "🇨🇿 捷克",
    "Bosnia and Herzegovina": "🇧🇦 波黑",
    "Paraguay": "🇵🇾 巴拉圭",
    "Qatar": "🇶🇦 卡塔尔",
    "Haiti": "🇭🇹 海地",
    "Turkey": "🇹🇷 土耳其"
  };
  return dict[enName] || enName;
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
  
  backtestChart = echarts.init(chartDom, 'dark');
  
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
      bottom: '3%',
      containLabel: true
    },
    xAxis: {
      type: 'category',
      data: [],
      axisLine: { show: false },
      axisTick: { show: false }
    },
    yAxis: {
      type: 'value',
      min: 0,
      max: 2.0,
      axisLine: { show: false },
      splitLine: { lineStyle: { color: 'rgba(255,255,255,0.05)' } }
    },
    series: [
      {
        name: 'Brier Score',
        type: 'line',
        smooth: true,
        data: [],
        lineStyle: {
          color: '#8800ff',
          width: 2
        },
        itemStyle: {
          color: '#00ff88'
        },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: 'rgba(136,0,255,0.25)' },
            { offset: 1, color: 'rgba(136,0,255,0)' }
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
    xAxis: { data: xData },
    series: [{ data: yData }]
  });
}
