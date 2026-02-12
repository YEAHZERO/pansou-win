import requests
import json

# 测试关键词
keywords = ["星辰未眠", "太奶奶"]

for keyword in keywords:
    print(f"测试关键词: {keyword}")
    print("------------------------")
    
    try:
        # 构建请求数据
        data = {
            "keyword": keyword,
            "page": 1
        }
        
        # 发送 POST 请求
        response = requests.post(
            "http://localhost:8889/api/search",
            headers={"Content-Type": "application/json"},
            data=json.dumps(data, ensure_ascii=False)
        )
        
        # 打印响应状态码
        print(f"响应状态码: {response.status_code}")
        
        # 尝试解析 JSON 响应
        try:
            result = response.json()
            print("搜索结果:")
            print(json.dumps(result, ensure_ascii=False, indent=2))
        except json.JSONDecodeError:
            print("响应内容:")
            print(response.text)
            
    except Exception as e:
        print(f"错误: {str(e)}")
    
    print()
