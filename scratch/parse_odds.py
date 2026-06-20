import requests

url = "https://webapi.sporttery.cn/gateway/jc/football/getMatchCalculatorV1.qry?poolCode=had,hhad,crs,ttg,hafu&channel=c"
headers = {
    "Referer": "https://www.lottery.gov.cn/",
    "User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
}
res = requests.get(url, headers=headers).json()
m = res["value"]["matchInfoList"][0]["subMatchList"][0]
for k, v in m.items():
    if not isinstance(v, dict):
        print(f"{k}: {v}")
    else:
        print(f"{k}: (dict keys: {list(v.keys())})")
