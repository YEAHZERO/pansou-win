#!/usr/bin/env python3
"""
PanSou API Python 客户端
支持官方服务和本地服务
"""

import requests
import json
import sys
import argparse
from typing import Dict, List, Optional, Any

class PanSouAPI:
    def __init__(self, base_url: str, username: Optional[str] = None, password: Optional[str] = None):
        """
        初始化 PanSou API 客户端
        
        Args:
            base_url: API 基础地址
            username: 用户名（如果需要认证）
            password: 密码（如果需要认证）
        """
        self.base_url = base_url.rstrip('/')
        self.session = requests.Session()
        self.session.headers.update({
            'Content-Type': 'application/json',
            'User-Agent': 'PanSou-Python-Client/1.0.0'
        })
        self.token = None
        
        if username and password:
            self.login(username, password)
    
    def login(self, username: str, password: str) -> Dict[str, Any]:
        """登录获取 token"""
        url = f"{self.base_url}/api/auth/login"
        data = {"username": username, "password": password}
        
        try:
            response = self.session.post(url, json=data)
            response.raise_for_status()
            
            result = response.json()
            self.token = result['token']
            self.session.headers.update({
                'Authorization': f'Bearer {self.token}'
            })
            
            print(f"✅ 登录成功，用户: {result['username']}")
            return result
        except requests.exceptions.RequestException as e:
            raise Exception(f"登录失败: {e}")
    
    def health_check(self) -> Dict[str, Any]:
        """健康检查"""
        url = f"{self.base_url}/api/health"
        try:
            response = self.session.get(url)
            response.raise_for_status()
            return response.json()
        except requests.exceptions.RequestException as e:
            raise Exception(f"健康检查失败: {e}")
    
    def search(self, 
               keyword: str, 
               res: str = "merge",
               src: str = "all",
               cloud_types: Optional[List[str]] = None,
               plugins: Optional[List[str]] = None,
               refresh: bool = False,
               ext: Optional[Dict[str, Any]] = None) -> Dict[str, Any]:
        """
        搜索网盘资源
        
        Args:
            keyword: 搜索关键词
            res: 返回格式 (merge/all/results)
            src: 数据源 (all/tg/plugin)
            cloud_types: 网盘类型列表
            plugins: 插件列表
            refresh: 是否强制刷新
            ext: 扩展参数
        """
        url = f"{self.base_url}/api/search"
        data = {
            "kw": keyword,
            "res": res,
            "src": src,
            "refresh": refresh
        }
        
        if cloud_types:
            data["cloud_types"] = cloud_types
        if plugins:
            data["plugins"] = plugins
        if ext:
            data["ext"] = ext
        
        try:
            response = self.session.post(url, json=data)
            response.raise_for_status()
            return response.json()
        except requests.exceptions.RequestException as e:
            if hasattr(e, 'response') and e.response is not None:
                if e.response.status_code == 401:
                    raise Exception("认证失败，请检查用户名密码或 token 是否过期")
                elif e.response.status_code == 400:
                    raise Exception(f"请求参数错误: {e.response.text}")
            raise Exception(f"搜索失败: {e}")

def format_results(results: Dict[str, Any], limit: int = 10) -> None:
    """格式化并打印搜索结果"""
    if 'merged_by_type' not in results:
        print("❌ 未找到相关资源")
        return
    
    merged_results = results['merged_by_type']
    if not merged_results:
        print("❌ 未找到相关资源")
        return
    
    total_links = sum(len(links) for links in merged_results.values())
    print(f"🎉 找到 {total_links} 个资源链接")
    print("=" * 60)
    
    # 网盘类型名称映射
    cloud_names = {
        'baidu': '百度网盘',
        'aliyun': '阿里云盘', 
        'quark': '夸克网盘',
        'tianyi': '天翼云盘',
        'uc': 'UC网盘',
        'mobile': '移动云盘',
        '115': '115网盘',
        'pikpak': 'PikPak',
        'xunlei': '迅雷网盘',
        '123': '123网盘',
        'magnet': '磁力链接',
        'ed2k': '电驴链接',
        'others': '其他网盘'
    }
    
    for cloud_type, links in merged_results.items():
        cloud_name = cloud_names.get(cloud_type, cloud_type)
        print(f"\n📁 {cloud_name} ({len(links)} 个链接)")
        print("-" * 40)
        
        for i, link in enumerate(links[:limit], 1):
            print(f"{i:2d}. {link['note']}")
            print(f"    🔗 {link['url']}")
            if link.get('password'):
                print(f"    🔑 提取码: {link['password']}")
            if link.get('source'):
                print(f"    📍 来源: {link['source']}")
            if link.get('datetime'):
                print(f"    📅 时间: {link['datetime']}")
            print()
        
        if len(links) > limit:
            print(f"    ... 还有 {len(links) - limit} 个链接")
            print()

def main():
    parser = argparse.ArgumentParser(
        description='PanSou 网盘搜索 Python 客户端',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
使用示例:
  %(prog)s "速度与激情"
  %(prog)s "Python教程" --url http://localhost:8888
  %(prog)s "电影" --username admin --password 123456
  %(prog)s "资源" --cloud-types baidu,aliyun --limit 5
  %(prog)s "软件" --plugins labi,zhizhen --src plugin
        """
    )
    
    parser.add_argument('keyword', help='搜索关键词')
    parser.add_argument('--url', default='https://so.252035.xyz', 
                       help='API 服务地址 (默认: %(default)s)')
    parser.add_argument('--username', help='用户名（如果需要认证）')
    parser.add_argument('--password', help='密码（如果需要认证）')
    parser.add_argument('--cloud-types', help='网盘类型，逗号分隔 (如: baidu,aliyun,quark)')
    parser.add_argument('--plugins', help='插件列表，逗号分隔 (如: labi,zhizhen)')
    parser.add_argument('--src', choices=['all', 'tg', 'plugin'], default='all',
                       help='数据源类型 (默认: %(default)s)')
    parser.add_argument('--limit', type=int, default=10, 
                       help='每种网盘显示的链接数量 (默认: %(default)s)')
    parser.add_argument('--refresh', action='store_true', 
                       help='强制刷新缓存')
    parser.add_argument('--health', action='store_true', 
                       help='仅执行健康检查')
    parser.add_argument('--verbose', '-v', action='store_true', 
                       help='显示详细信息')
    
    args = parser.parse_args()
    
    try:
        # 创建 API 客户端
        if args.verbose:
            print(f"🔗 连接到: {args.url}")
        
        api = PanSouAPI(args.url, args.username, args.password)
        
        # 健康检查
        if args.health:
            health = api.health_check()
            print("🏥 服务健康状态:")
            print(f"  状态: {health.get('status', '未知')}")
            print(f"  认证: {'启用' if health.get('auth_enabled') else '禁用'}")
            print(f"  插件: {'启用' if health.get('plugins_enabled') else '禁用'}")
            print(f"  插件数量: {health.get('plugin_count', 0)}")
            print(f"  频道数量: {health.get('channels_count', 0)}")
            return
        
        # 准备搜索参数
        search_options = {
            'src': args.src,
            'refresh': args.refresh
        }
        
        if args.cloud_types:
            search_options['cloud_types'] = [t.strip() for t in args.cloud_types.split(',')]
        
        if args.plugins:
            search_options['plugins'] = [p.strip() for p in args.plugins.split(',')]
        
        # 执行搜索
        if args.verbose:
            print(f"🔍 搜索关键词: {args.keyword}")
            print(f"📊 搜索参数: {search_options}")
            print()
        
        results = api.search(args.keyword, **search_options)
        
        # 显示结果
        format_results(results, args.limit)
        
    except KeyboardInterrupt:
        print("\n❌ 用户取消操作")
        sys.exit(1)
    except Exception as e:
        print(f"❌ 错误: {e}", file=sys.stderr)
        sys.exit(1)

if __name__ == '__main__':
    main()