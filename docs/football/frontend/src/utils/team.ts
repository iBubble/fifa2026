// team.ts - 球队名称翻译与缩写格式化工具
// 支持 100% 真实数据下英文名称到“缩写 (中文)”的完美映射与动态降级 fallback

const TEAM_TRANSLATIONS: Record<string, { abbr: string; cn: string }> = {
  // 英超 (EPL)
  'Manchester City': { abbr: 'MCI', cn: '曼城' },
  'Arsenal': { abbr: 'ARS', cn: '阿森纳' },
  'Liverpool': { abbr: 'LIV', cn: '利物浦' },
  'Aston Villa': { abbr: 'AST', cn: '阿斯顿维拉' },
  'Tottenham Hotspur': { abbr: 'TOT', cn: '热刺' },
  'Chelsea': { abbr: 'CHE', cn: '切尔西' },
  'Manchester United': { abbr: 'MUN', cn: '曼联' },
  'Newcastle United': { abbr: 'NEW', cn: '纽卡斯尔' },
  'West Ham United': { abbr: 'WHU', cn: '西汉姆联' },
  'West Ham': { abbr: 'WHU', cn: '西汉姆联' },
  'Brighton': { abbr: 'BHA', cn: '布莱顿' },
  'Wolverhampton Wanderers': { abbr: 'WOL', cn: '狼队' },
  'Wolves': { abbr: 'WOL', cn: '狼队' },
  'Crystal Palace': { abbr: 'CRY', cn: '水晶宫' },
  'Everton': { abbr: 'EVE', cn: '埃弗顿' },
  'Bournemouth': { abbr: 'BOU', cn: '伯恩茅斯' },
  'Fulham': { abbr: 'FUL', cn: '富勒姆' },
  'Nottingham Forest': { abbr: 'NFO', cn: '诺丁汉森林' },
  'Brentford': { abbr: 'BRE', cn: '布伦特福德' },
  'Leicester City': { abbr: 'LEI', cn: '莱斯特城' },
  'Leicester': { abbr: 'LEI', cn: '莱斯特城' },
  'Ipswich': { abbr: 'IPS', cn: '伊普斯维奇' },
  'Southampton': { abbr: 'SOU', cn: '南安普顿' },

  // 西甲 (La Liga)
  'Real Madrid': { abbr: 'RMA', cn: '皇家马德里' },
  'Barcelona': { abbr: 'BAR', cn: '巴塞罗那' },
  'Atletico Madrid': { abbr: 'ATM', cn: '马德里竞技' },
  'Sevilla': { abbr: 'SEV', cn: '塞维利亚' },
  'Real Sociedad': { abbr: 'RSO', cn: '皇家社会' },
  'Athletic Club': { abbr: 'ATH', cn: '毕尔巴鄂竞技' },
  'Athletic Bilbao': { abbr: 'ATH', cn: '毕尔巴鄂竞技' },
  'Villarreal': { abbr: 'VIL', cn: '比利亚雷亚尔' },
  'Valencia': { abbr: 'VAL', cn: '瓦伦西亚' },
  'Girona': { abbr: 'GIR', cn: '赫罗纳' },
  'Real Betis': { abbr: 'BET', cn: '皇家贝蒂斯' },

  // 意甲 (Serie A)
  'Juventus': { abbr: 'JUV', cn: '尤文图斯' },
  'AC Milan': { abbr: 'ACM', cn: 'AC米兰' },
  'Inter Milan': { abbr: 'INT', cn: '国际米兰' },
  'Inter': { abbr: 'INT', cn: '国际米兰' },
  'Roma': { abbr: 'ROM', cn: '罗马' },
  'Napoli': { abbr: 'NAP', cn: '那不勒斯' },
  'Lazio': { abbr: 'LAZ', cn: '拉齐奥' },
  'Atalanta': { abbr: 'ATA', cn: '亚特兰大' },
  'Fiorentina': { abbr: 'FIO', cn: '佛罗伦萨' },
  'Bologna': { abbr: 'BOL', cn: '博洛尼亚' },

  // 德甲 (Bundesliga)
  'Bayern Munich': { abbr: 'FCB', cn: '拜仁慕尼黑' },
  'Borussia Dortmund': { abbr: 'BVB', cn: '多特蒙德' },
  'Dortmund': { abbr: 'BVB', cn: '多特蒙德' },
  'Bayer Leverkusen': { abbr: 'LEV', cn: '勒沃库森' },
  'RB Leipzig': { abbr: 'RBL', cn: '莱比锡红牛' },
  'Eintracht Frankfurt': { abbr: 'SGE', cn: '法兰克福' },
  'VfB Stuttgart': { abbr: 'VFB', cn: '斯图加特' },
  'Werder Bremen': { abbr: 'SVW', cn: '云达不来梅' },

  // 法甲 (Ligue 1)
  'Paris Saint Germain': { abbr: 'PSG', cn: '巴黎圣日耳曼' },
  'Paris SG': { abbr: 'PSG', cn: '巴黎圣日耳曼' },
  'Monaco': { abbr: 'ASM', cn: '摩纳哥' },
  'Lyon': { abbr: 'OL', cn: '里昂' },
  'Lille': { abbr: 'LOSC', cn: '里尔' },
  'Nice': { abbr: 'OGCN', cn: '尼斯' },
  'Lens': { abbr: 'RCL', cn: '朗斯' },
  'Rennes': { abbr: 'SRFC', cn: '雷恩' },
  'Marseille': { abbr: 'OM', cn: '马赛' },

  // 美职联 (MLS)
  'Inter Miami CF': { abbr: 'MIA', cn: '迈阿密国际' },
  'Inter Miami': { abbr: 'MIA', cn: '迈阿密国际' },
  'Los Angeles FC': { abbr: 'LAFC', cn: '洛杉矶FC' },
  'LA Galaxy': { abbr: 'LAG', cn: '洛杉矶银河' },
  'Columbus Crew': { abbr: 'CLB', cn: '哥伦布机员' },
  'New York Red Bulls': { abbr: 'NYRB', cn: '纽约红牛' },
  'Orlando City SC': { abbr: 'ORL', cn: '奥兰多城' },
  'Portland Timbers': { abbr: 'POR', cn: '波特兰伐木者' },

  // 世界杯 (World Cup)
  'Argentina': { abbr: 'ARG', cn: '阿根廷' },
  'France': { abbr: 'FRA', cn: '法国' },
  'Brazil': { abbr: 'BRA', cn: '巴西' },
  'Germany': { abbr: 'GER', cn: '德国' },
  'Spain': { abbr: 'ESP', cn: '西班牙' },
  'Portugal': { abbr: 'POR', cn: '葡萄牙' },
  'England': { abbr: 'ENG', cn: '英格兰' },
  'Netherlands': { abbr: 'NED', cn: '荷兰' },
  'South Africa': { abbr: 'RSA', cn: '南非' },
  'Mexico': { abbr: 'MEX', cn: '墨西哥' }
}

/**
 * 格式化球队名称：将英文原名翻译为“缩写 (中文)”的格式
 * 例如: "Paris Saint Germain" -> "PSG (巴黎圣日耳曼)"
 */
export function formatTeamName(englishName: string): string {
  if (!englishName) return englishName
  
  // 精确匹配与去除前后空格后的匹配
  const match = TEAM_TRANSLATIONS[englishName] || TEAM_TRANSLATIONS[englishName.trim()]
  if (match) {
    return `${match.abbr} (${match.cn})`
  }

  // 模糊去噪匹配（处理例如 "Inter Milan" vs "Inter", "Marseille" vs "Olympique de Marseille" 等）
  for (const [key, value] of Object.entries(TEAM_TRANSLATIONS)) {
    if (englishName.toLowerCase().includes(key.toLowerCase()) || key.toLowerCase().includes(englishName.toLowerCase())) {
      return `${value.abbr} (${value.cn})`
    }
  }

  // 兜底动态降级算法：提取英文首字母作为缩写
  const cleanName = englishName.replace(/(FC|CF|SC|United|City|Club|de)$/i, '').trim()
  const words = cleanName.split(/[\s-]+/)
  let abbr = ''
  if (words.length >= 3) {
    abbr = (words[0][0] + words[1][0] + words[2][0]).toUpperCase()
  } else if (words.length === 2 && words[0] && words[1]) {
    abbr = (words[0].substring(0, 2) + words[1][0]).toUpperCase()
  } else if (words[0] && words[0].length >= 3) {
    abbr = words[0].substring(0, 3).toUpperCase()
  } else {
    abbr = (words[0] || 'UNK').toUpperCase()
  }
  
  return `${abbr} (${englishName})`
}
