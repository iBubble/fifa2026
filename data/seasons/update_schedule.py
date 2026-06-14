import json
import re
import os
import sqlite3
from datetime import datetime

# 1. 从 live_sync.go 提取 teamDictionary
go_file_path = "/Users/gemini/Projects/Own/FIFA2026/src/internal/service/prediction/live_sync.go"
team_dict = {}

with open(go_file_path, "r", encoding="utf-8") as f:
    go_content = f.read()

# 匹配 "中文": "英文" 或者是 "英文": "英文"
matches = re.findall(r'"([^"]+)":\s*"([^"]+)"', go_content)
for k, v in matches:
    team_dict[k] = v

print(f"从 live_sync.go 提取了 {len(team_dict)} 个队名映射。")

# 2. 读取 72 场小组赛数据
group_file_path = "/Users/gemini/.gemini/antigravity-ide/brain/b2aac6c6-ba09-4e66-bccf-7ee3fb6c635c/browser/scratchpad_qvqj4mu6.md"
with open(group_file_path, "r", encoding="utf-8") as f:
    group_games = json.load(f)

# 3. 读取 32 场淘汰赛数据
js_file_path = "/Users/gemini/.gemini/antigravity-ide/brain/b2aac6c6-ba09-4e66-bccf-7ee3fb6c635c/scratch/worldcup2026_schedule_tabs.37359b71.js"
with open(js_file_path, "r", encoding="utf-8") as f:
    js_content = f.read()

# 正则匹配淘汰赛: {matchId:"M74",date:"06/30/2026",time:"04:30",home:"1E",away:"3ABCDF"}
knockout_raw = re.findall(r'\{matchId:"([^"]+)",date:"([^"]+)",time:"([^"]+)",home:"([^"]+)",away:"([^"]+)"\}', js_content)
print(f"正则匹配到 {len(knockout_raw)} 场淘汰赛。")

# 4. 读取待更新的赛季 JSON
season_json_path = "/Users/gemini/Projects/Own/FIFA2026/data/seasons/fifa_2026.json"
with open(season_json_path, "r", encoding="utf-8") as f:
    season_data = json.load(f)

matches_list = season_data.get("matches", [])
print(f"原赛季 JSON 中共有 {len(matches_list)} 场比赛。")

# 记录更新数量
group_updated = 0
knockout_updated = 0

# 5. 更新小组赛
for game in group_games:
    team1 = game["team1"]
    team2 = game["team2"]
    date_str = game["date"] # 格式: "2026.06.12 03:00"
    
    # 转换为英文
    t1_en = team_dict.get(team1)
    t2_en = team_dict.get(team2)
    
    if not t1_en or not t2_en:
        print(f"⚠️ 无法找到中英文映射: {team1} ({t1_en}) vs {team2} ({t2_en})")
        continue
        
    # 解析时间
    dt = datetime.strptime(date_str, "%Y.%m.%d %H:%M")
    new_scheduled_at = dt.strftime("%Y-%m-%dT%H:%M:00+08:00")
    
    # 查找匹配的比赛
    matched = False
    for m in matches_list:
        if m["id"].startswith("wc2026_m") and int(m["id"].split("_m")[1]) <= 72:
            # 比较主客队
            if (m["homeTeam"] == t1_en and m["awayTeam"] == t2_en) or (m["homeTeam"] == t2_en and m["awayTeam"] == t1_en):
                m["scheduledAt"] = new_scheduled_at
                group_updated += 1
                matched = True
                break
    if not matched:
        print(f"⚠️ 未能在赛季 JSON 中找到匹配的小组赛: {team1} ({t1_en}) vs {team2} ({t2_en})")

# 6. 更新淘汰赛
for matchId, date, time_str, home, away in knockout_raw:
    dt_str = f"{date} {time_str}"
    dt = datetime.strptime(dt_str, "%m/%d/%Y %H:%M")
    new_scheduled_at = dt.strftime("%Y-%m-%dT%H:%M:00+08:00")
    
    match_key = f"wc2026_{matchId.lower()}"
    matched = False
    for m in matches_list:
        if m["id"] == match_key:
            m["scheduledAt"] = new_scheduled_at
            knockout_updated += 1
            matched = True
            break
    if not matched:
        print(f"⚠️ 未能在赛季 JSON 中找到匹配的淘汰赛 ID: {match_key}")

print(f"JSON 成功更新小组赛 {group_updated} 场，淘汰赛 {knockout_updated} 场。")

# 7. 写回赛季 JSON
with open(season_json_path, "w", encoding="utf-8") as f:
    json.dump(season_data, f, ensure_ascii=False, indent=2)
print("已成功覆盖写入 fifa_2026.json 文件！")

# 8. 强行更新 SQLite 数据库中的 matches 表时间（保证已完赛 FT 比赛的时间也同步校准）
db_path = "/Users/gemini/Projects/Own/FIFA2026/data/db/fifa2026.db"
if os.path.exists(db_path):
    print("正在直接更新 SQLite 数据库时间字段...")
    conn = sqlite3.connect(db_path)
    cursor = conn.cursor()
    
    db_updated = 0
    for m in matches_list:
        m_id = m["id"]
        sched_at = m["scheduledAt"] # 格式如 "2026-06-12T03:00:00+08:00"
        
        # 转换为 Go 写入 SQLite 的标准格式
        dt = datetime.strptime(sched_at[:19], "%Y-%m-%dT%H:%M:%S")
        db_time = dt.strftime("%Y-%m-%d %H:%M:%S") + " +0800 +0800"
        
        cursor.execute("UPDATE matches SET scheduled_at = ? WHERE id = ?", (db_time, m_id))
        if cursor.rowcount > 0:
            db_updated += 1
            
    conn.commit()
    conn.close()
    print(f"SQLite 数据库成功更新了 {db_updated} 场比赛的时间。")
else:
    print("⚠️ 数据库文件不存在，将在服务器冷启动时自动导入。")
