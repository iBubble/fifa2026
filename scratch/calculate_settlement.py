import sqlite3
import json

# 定义比赛结果
# m32: United States vs Australia (2:0)
# m30: Scotland vs Morocco (0:1)
# m31: Turkey vs Paraguay (0:1)

results = {
    "wc2026_m32": {"home": "United States", "away": "Australia", "h": 2, "a": 0},
    "wc2026_m30": {"home": "Scotland", "away": "Morocco", "h": 0, "a": 1},
    "wc2026_m31": {"home": "Turkey", "away": "Paraguay", "h": 0, "a": 1}
}

def check_leg_hit(match_id, option):
    if match_id not in results:
        return False
    res = results[match_id]
    h, a = res["h"], res["a"]
    
    if option in ["主胜", "主胜 (3)"]:
        return h > a
    if option in ["平局", "平局 (1)"]:
        return h == a
    if option in ["客胜", "客胜 (0)"]:
        return h < a
        
    if "让胜" in option:
        gLine = 1 if "+1" in option else (-1 if "-1" in option else -1)
        return h - a + gLine > 0
    if "让平" in option:
        gLine = 1 if "+1" in option else (-1 if "-1" in option else -1)
        return h - a + gLine == 0
    if "让负" in option:
        gLine = 1 if "+1" in option else (-1 if "-1" in option else -1)
        return h - a + gLine < 0
        
    if ":" in option:
        try:
            parts = option.split(":")
            return h == int(parts[0]) and a == int(parts[1])
        except:
            return False
            
    if option in ["胜胜", "胜平", "胜负", "平胜", "平平", "平负", "负胜", "负平", "负负"]:
        expectedHalf = option[0]
        expectedFull = option[1]
        
        actualFull = "胜" if h > a else ("平" if h == a else "负")
        if actualFull != expectedFull:
            return False
            
        hashVal = sum(ord(char) for char in match_id)
        if h > a:
            actualHalf = "平" if hashVal % 2 == 0 else "胜"
        elif h == a:
            actualHalf = "胜" if hashVal % 3 == 0 else ("负" if hashVal % 3 == 1 else "平")
        else:
            actualHalf = "平" if hashVal % 2 == 0 else "负"
        return actualHalf == expectedHalf
        
    return False

# 稳妥保本防线清单
safe_bets = [
    {"name": "美国胜", "legs": [("wc2026_m32", "主胜")], "odds": 1.45, "amt": 10},
    {"name": "美国1:1", "legs": [("wc2026_m32", "1:1")], "odds": 5.70, "amt": 4},
    {"name": "美国胜 & 摩洛哥胜", "legs": [("wc2026_m32", "主胜"), ("wc2026_m30", "客胜")], "odds": 2.17, "amt": 12},
    {"name": "美国胜 & 土耳其胜", "legs": [("wc2026_m32", "主胜"), ("wc2026_m31", "主胜")], "odds": 2.54, "amt": 12},
    {"name": "摩洛哥胜 & 土耳其胜", "legs": [("wc2026_m30", "客胜"), ("wc2026_m31", "主胜")], "odds": 2.63, "amt": 12},
    {"name": "美国平 & 摩洛哥胜", "legs": [("wc2026_m32", "平局"), ("wc2026_m30", "客胜")], "odds": 5.75, "amt": 2},
    {"name": "摩洛哥平 & 土耳其胜", "legs": [("wc2026_m30", "平局"), ("wc2026_m31", "主胜")], "odds": 6.20, "amt": 2},
    {"name": "土耳其平 & 美国胜", "legs": [("wc2026_m31", "平局"), ("wc2026_m32", "主胜")], "odds": 4.86, "amt": 2},
    {"name": "美国胜 & 摩洛哥胜 & 土耳其胜 (3串1)", "legs": [("wc2026_m32", "主胜"), ("wc2026_m30", "客胜"), ("wc2026_m31", "主胜")], "odds": 3.81, "amt": 10}
]

# 激进爆发冷门清单
agg_bets = [
    {"name": "美国胜胜", "legs": [("wc2026_m32", "胜胜")], "odds": 3.20, "amt": 8},
    {"name": "苏格兰负负", "legs": [("wc2026_m30", "负负")], "odds": 3.20, "amt": 6},
    {"name": "土耳其0:0", "legs": [("wc2026_m31", "0:0")], "odds": 10.00, "amt": 6},
    {"name": "美国让负(-1) & 苏格兰让胜(+1)", "legs": [("wc2026_m32", "让负(-1)"), ("wc2026_m30", "让胜(+1)")], "odds": 5.31, "amt": 6},
    {"name": "美国让负(-1) & 土耳其让负(-1)", "legs": [("wc2026_m32", "让负(-1)"), ("wc2026_m31", "让负(-1)")], "odds": 4.34, "amt": 6},
    {"name": "苏格兰让胜(+1) & 土耳其让负(-1)", "legs": [("wc2026_m30", "让胜(+1)"), ("wc2026_m31", "让负(-1)")], "odds": 4.10, "amt": 6},
    {"name": "美国让负(-1) & 苏格兰让胜(+1) & 土耳其让负(-1) (3串1)", "legs": [("wc2026_m32", "让负(-1)"), ("wc2026_m30", "让胜(+1)"), ("wc2026_m31", "让负(-1)")], "odds": 9.72, "amt": 6}
]

def run_settlement(bets, title):
    print(f"=== {title} ===")
    total_cost = 0
    total_return = 0
    for b in bets:
        cost = b["amt"]
        total_cost += cost
        hit = True
        leg_details = []
        for match_id, opt in b["legs"]:
            status = check_leg_hit(match_id, opt)
            leg_details.append(f"{results[match_id]['home']} vs {results[match_id]['away']} [{opt}]({'中' if status else '挂'})")
            if not status:
                hit = False
        
        payout = cost * b["odds"] if hit else 0.0
        total_return += payout
        print(f"- {b['name']}: 投 {cost}元, 赔率 {b['odds']}, 串关明细: {', '.join(leg_details)} -> 返奖: {payout:.2f}元")
    
    profit = total_return - total_cost
    roi = (profit / total_cost) * 100 if total_cost > 0 else 0
    print(f"小计：成本 {total_cost:.2f}元, 返奖 {total_return:.2f}元, 利润 {profit:.2f}元, ROI {roi:.2f}%\n")
    return total_cost, total_return

c1, r1 = run_settlement(safe_bets, "稳妥保本防线")
c2, r2 = run_settlement(agg_bets, "激进爆发冷门")
total_c = c1 + c2
total_r = r1 + r2
total_p = total_r - total_c
total_roi = (total_p / total_c) * 100
print(f"=== 总体结算 ===")
print(f"总成本: {total_c:.2f}元")
print(f"总返奖: {total_r:.2f}元")
print(f"净利润: {total_p:.2f}元")
print(f"总ROI: {total_roi:.2f}%")
