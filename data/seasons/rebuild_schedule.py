import json
import re
import os
import sqlite3
from datetime import datetime

# 1. 队名中文到标准英文映射字典
c2e = {
    "墨西哥": "Mexico", "南非": "South Africa", "韩国": "South Korea", "捷克": "Czech Republic",
    "加拿大": "Canada", "波黑": "Bosnia and Herzegovina", "卡塔尔": "Qatar", "瑞士": "Switzerland",
    "巴西": "Brazil", "摩洛哥": "Morocco", "海地": "Haiti", "苏格兰": "Scotland",
    "美国": "United States", "巴拉圭": "Paraguay", "澳大利亚": "Australia", "土耳其": "Turkey",
    "德国": "Germany", "库拉索": "Curaçao", "科特迪瓦": "Ivory Coast", "厄瓜多尔": "Ecuador",
    "荷兰": "Netherlands", "日本": "Japan", "瑞典": "Sweden", "突尼斯": "Tunisia",
    "比利时": "Belgium", "埃及": "Egypt", "伊朗": "Iran", "新西兰": "New Zealand",
    "西班牙": "Spain", "佛得角": "Cape Verde", "沙特阿拉伯": "Saudi Arabia", "乌拉圭": "Uruguay",
    "法国": "France", "塞内加尔": "Senegal", "伊拉克": "Iraq", "挪威": "Norway",
    "阿根廷": "Argentina", "阿尔及利亚": "Algeria", "奥地利": "Austria", "约旦": "Jordan",
    "葡萄牙": "Portugal", "刚果金": "Democratic Republic of the Congo", "乌兹别克斯坦": "Uzbekistan", "哥伦比亚": "Colombia",
    "英格兰": "England", "克罗地亚": "Croatia", "加纳": "Ghana", "巴拿马": "Panama"
}

# 2. 比赛城市到场馆映射
city2venue = {
    "墨西哥城": "Estadio Azteca",
    "瓜达拉哈拉": "Estadio Akron",
    "多伦多": "BMO Field",
    "洛杉矶": "SoFi Stadium",
    "波士顿": "Gillette Stadium",
    "温哥华": "BC Place",
    "新泽西纽约": "MetLife Stadium",
    "纽约": "MetLife Stadium",
    "旧金山": "Levi's Stadium",
    "费城": "Lincoln Financial Field",
    "休斯顿": "NRG Stadium",
    "达拉斯": "AT&T Stadium",
    "蒙特雷": "Estadio BBVA",
    "迈阿密": "Hard Rock Stadium",
    "亚特兰大": "Mercedes-Benz Stadium",
    "堪萨斯": "GEHA Field at Arrowhead Stadium",
    "西雅图": "Lumen Field"
}

# 3. 淘汰赛轮次中文到英文组名映射
stage_map = {
    "1/16决赛": "R32",
    "1/8决赛": "R16",
    "1/4决赛": "QF",
    "半决赛": "SF",
    "季军赛": "3RD",
    "决赛": "FINAL"
}

def parse_time(date_str, time_str):
    # 解析 "2026年6月12日" + "03:00"
    m = re.match(r'(\d+)年(\d+)月(\d+)日', date_str)
    if not m:
        raise ValueError(f"日期解析错误: {date_str}")
    year, month, day = m.groups()
    dt_str = f"{year}-{int(month):02d}-{int(day):02d} {time_str}:00"
    dt = datetime.strptime(dt_str, "%Y-%m-%d %H:%M:%S")
    return dt

def rebuild():
    md_path = "/Users/gemini/Projects/Own/FIFA2026/docs/FIFA2026赛程表.md"
    with open(md_path, "r", encoding="utf-8") as f:
        lines = f.readlines()

    new_matches = []
    current_stage = "" # 用于向下传播淘汰赛轮次

    for line in lines:
        line_str = line.strip()
        if not line_str or line_str.startswith("-") or "场序" in line_str or "注" in line_str:
            continue

        tokens = line_str.split()
        if len(tokens) < 6:
            continue

        if not tokens[0].isdigit():
            continue

        idx = int(tokens[0])
        
        if idx <= 72:
            if len(tokens) < 8:
                continue
            group_cn = tokens[1]
            date_str = tokens[2]
            time_str = tokens[3]
            teamA_cn = tokens[4]
            score = tokens[5]
            teamB_cn = tokens[6]
            city = tokens[7]

            group_en = group_cn.replace("组", "")
            teamA_en = c2e.get(teamA_cn)
            teamB_en = c2e.get(teamB_cn)
            
            if not teamA_en or not teamB_en:
                print(f"⚠️ 场序 {idx} 翻译缺失: {teamA_cn} -> {teamA_en}, {teamB_cn} -> {teamB_en}")
                continue

            dt = parse_time(date_str, time_str)
            scheduled_at = dt.strftime("%Y-%m-%dT%H:%M:00+08:00")
            venue = city2venue.get(city, "")

            status = "NS"
            if idx <= 4:
                status = "FT"

            new_matches.append({
                "id": f"wc2026_m{idx}",
                "homeTeam": teamA_en,
                "awayTeam": teamB_en,
                "group": group_en,
                "scheduledAt": scheduled_at,
                "status": status,
                "venue": venue,
                "db_time": dt.strftime("%Y-%m-%d %H:%M:%S") + " +0800 +0800"
            })
        else:
            if "决赛" in tokens[1] or "半决赛" in tokens[1] or "季军赛" in tokens[1]:
                current_stage = stage_map.get(tokens[1], "R32")
                offset = 1
            else:
                offset = 0

            date_str = tokens[1 + offset]
            time_str = tokens[2 + offset]
            teamA = tokens[3 + offset]
            score = tokens[4 + offset]
            teamB = tokens[5 + offset]
            city = tokens[6 + offset]

            dt = parse_time(date_str, time_str)
            scheduled_at = dt.strftime("%Y-%m-%dT%H:%M:00+08:00")
            venue = city2venue.get(city, "")

            new_matches.append({
                "id": f"wc2026_m{idx}",
                "homeTeam": "0",
                "awayTeam": "0",
                "group": current_stage,
                "scheduledAt": scheduled_at,
                "status": "NS",
                "venue": venue,
                "db_time": dt.strftime("%Y-%m-%d %H:%M:%S") + " +0800 +0800"
            })

    print(f"解析成功，共提取 {len(new_matches)} 场比赛！")
    
    # 4. 写入 json 文件
    json_path = "/Users/gemini/Projects/Own/FIFA2026/data/seasons/fifa_2026.json"
    with open(json_path, "r", encoding="utf-8") as f:
        season_data = json.load(f)
        
    # 去除 db_time 仅保留需要的字段写回 json
    json_matches = []
    for m in new_matches:
        json_matches.append({
            "id": m["id"],
            "homeTeam": m["homeTeam"],
            "awayTeam": m["awayTeam"],
            "group": m["group"],
            "scheduledAt": m["scheduledAt"],
            "status": m["status"],
            "venue": m["venue"]
        })
    season_data["matches"] = json_matches
    
    with open(json_path, "w", encoding="utf-8") as f:
        json.dump(season_data, f, ensure_ascii=False, indent=2)
    print("已成功覆盖写入 fifa_2026.json！")

    # 5. 直接向 SQLite 重新填充这 100 场赛事数据（避开锁竞争）
    db_path = "/Users/gemini/Projects/Own/FIFA2026/data/db/fifa2026.db"
    conn = sqlite3.connect(db_path)
    cursor = conn.cursor()
    
    # 清空 matches 中除前4场外的赛程以确保干净重载
    cursor.execute("DELETE FROM matches WHERE id LIKE 'wc2026_%' AND id NOT IN ('wc2026_m1', 'wc2026_m2', 'wc2026_m3', 'wc2026_m4')")
    
    inserted = 0
    for m in new_matches:
        if m["id"] in ('wc2026_m1', 'wc2026_m2', 'wc2026_m3', 'wc2026_m4'):
            # 对于前4场已赛完的比赛，更新其静态字段以纠正组别
            cursor.execute("""
                UPDATE matches 
                SET match_group = ?, home_team = ?, away_team = ?, venue = ?, scheduled_at = ?
                WHERE id = ?
            """, (m["group"], m["homeTeam"], m["awayTeam"], m["venue"], m["db_time"], m["id"]))
            continue
        cursor.execute("""
            INSERT INTO matches (id, tournament_id, home_team, away_team, match_group, scheduled_at, status, home_score, away_score, venue)
            VALUES (?, 'fifa_2026', ?, ?, ?, ?, 'NS', 0, 0, ?)
        """, (m["id"], m["homeTeam"], m["awayTeam"], m["group"], m["db_time"], m["venue"]))
        inserted += 1
        
    conn.commit()
    conn.close()
    print(f"已直接将 {inserted} 场赛程直接装载入 SQLite 数据库！")

if __name__ == "__main__":
    rebuild()
