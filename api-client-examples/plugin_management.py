#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
插件管理工具 (Python版本)
用于管理插件优先级和查看统计信息
"""

import requests
import json
from typing import Dict, List, Optional
from datetime import datetime

BASE_URL = "http://localhost:8888/api"


class PluginManager:
    """插件管理器"""
    
    def __init__(self, base_url: str = BASE_URL):
        self.base_url = base_url
        self.session = requests.Session()
    
    def get_all_plugins(self) -> List[Dict]:
        """获取所有插件信息"""
        response = self.session.get(f"{self.base_url}/plugins")
        response.raise_for_status()
        return response.json()["data"]
    
    def get_plugin_stats(self) -> Dict:
        """获取所有插件统计"""
        response = self.session.get(f"{self.base_url}/plugins/stats")
        response.raise_for_status()
        return response.json()["data"]
    
    def get_plugin_detail(self, plugin_name: str) -> Dict:
        """获取单个插件详细信息"""
        response = self.session.get(f"{self.base_url}/plugins/stats/{plugin_name}")
        response.raise_for_status()
        return response.json()["data"]
    
    def set_plugin_priority(self, plugin_name: str, priority: int) -> Dict:
        """设置插件优先级"""
        data = {
            "plugin_name": plugin_name,
            "priority": priority
        }
        response = self.session.post(
            f"{self.base_url}/plugins/priority",
            json=data
        )
        response.raise_for_status()
        return response.json()
    
    def set_batch_priority(self, priorities: Dict[str, int]) -> Dict:
        """批量设置插件优先级"""
        data = {"priorities": priorities}
        response = self.session.post(
            f"{self.base_url}/plugins/priority/batch",
            json=data
        )
        response.raise_for_status()
        return response.json()
    
    def reset_plugin_priority(self, plugin_name: str) -> Dict:
        """重置插件优先级"""
        response = self.session.delete(
            f"{self.base_url}/plugins/priority/{plugin_name}"
        )
        response.raise_for_status()
        return response.json()
    
    def export_stats(self, format: str = "json", output_file: str = None) -> None:
        """导出统计数据"""
        response = self.session.get(
            f"{self.base_url}/plugins/stats/export",
            params={"format": format}
        )
        response.raise_for_status()
        
        if output_file:
            with open(output_file, 'w', encoding='utf-8') as f:
                if format == "json":
                    json.dump(response.json(), f, indent=2, ensure_ascii=False)
                else:
                    f.write(response.text)
            print(f"✓ 已导出到: {output_file}")
        else:
            print(response.text if format == "csv" else json.dumps(response.json(), indent=2, ensure_ascii=False))


def print_plugins_table(plugins: List[Dict]) -> None:
    """打印插件信息表格"""
    print("\n" + "=" * 100)
    print(f"{'插件名称':<15} {'默认优先级':<10} {'自定义优先级':<12} {'当前优先级':<10} {'搜索次数':<10} {'平均结果':<10} {'平均响应(ms)':<15}")
    print("-" * 100)
    
    for plugin in plugins:
        custom_priority = plugin.get('custom_priority', 0)
        custom_str = str(custom_priority) if custom_priority > 0 else "-"
        
        print(f"{plugin['name']:<15} {plugin['default_priority']:<10} {custom_str:<12} "
              f"{plugin['current_priority']:<10} {plugin['total_searches']:<10} "
              f"{plugin['avg_results']:<10.1f} {plugin['avg_response_time']:<15.1f}")
    
    print("=" * 100)


def print_stats_table(stats: Dict) -> None:
    """打印统计信息表格"""
    print("\n" + "=" * 120)
    print(f"{'插件名称':<15} {'搜索次数':<10} {'成功次数':<10} {'总结果数':<10} "
          f"{'平均结果':<10} {'平均响应(ms)':<15} {'自定义优先级':<12}")
    print("-" * 120)
    
    # 按平均结果数排序
    sorted_stats = sorted(
        stats.items(),
        key=lambda x: x[1].get('avg_results', 0),
        reverse=True
    )
    
    for name, stat in sorted_stats:
        custom_priority = stat.get('custom_priority', 0)
        custom_str = str(custom_priority) if custom_priority > 0 else "-"
        
        print(f"{stat['plugin_name']:<15} {stat['total_searches']:<10} "
              f"{stat['success_searches']:<10} {stat['total_results']:<10} "
              f"{stat['avg_results']:<10.1f} {stat['avg_response_time']:<15.1f} "
              f"{custom_str:<12}")
    
    print("=" * 120)


def print_plugin_detail(stat: Dict) -> None:
    """打印插件详细信息"""
    print("\n" + "=" * 60)
    print(f"插件名称: {stat['plugin_name']}")
    print(f"总搜索次数: {stat['total_searches']}")
    print(f"成功次数: {stat['success_searches']}")
    print(f"失败次数: {stat['failed_searches']}")
    print(f"总结果数: {stat['total_results']}")
    print(f"平均结果数: {stat['avg_results']:.2f}")
    print(f"平均响应时间: {stat['avg_response_time']:.2f}ms")
    print(f"最后搜索时间: {stat.get('last_search_time', 'N/A')}")
    
    custom_priority = stat.get('custom_priority', 0)
    priority_str = str(custom_priority) if custom_priority > 0 else "默认"
    print(f"自定义优先级: {priority_str}")
    print("=" * 60)


def calculate_recommended_priorities(stats: Dict) -> Dict[str, int]:
    """计算推荐的优先级"""
    scores = []
    
    for name, stat in stats.items():
        avg_results = stat.get('avg_results', 0)
        avg_time = stat.get('avg_response_time', 1)
        total_searches = stat.get('total_searches', 0)
        success_searches = stat.get('success_searches', 0)
        
        # 计算成功率
        success_rate = (success_searches / total_searches * 100) if total_searches > 0 else 0
        
        # 计算综合得分：平均结果数 * 成功率 / 响应时间
        score = (avg_results * success_rate) / avg_time if avg_time > 0 else 0
        
        scores.append({
            'name': name,
            'score': score,
            'avg_results': avg_results,
            'success_rate': success_rate,
            'avg_time': avg_time
        })
    
    # 排序
    scores.sort(key=lambda x: x['score'], reverse=True)
    
    # 分配优先级
    priorities = {}
    tier1_count = min(5, len(scores))
    tier2_count = min(10, len(scores) - tier1_count)
    
    for i, item in enumerate(scores):
        if i < tier1_count:
            priorities[item['name']] = 1
            item['recommended_priority'] = "1 (第一梯队)"
        elif i < (tier1_count + tier2_count):
            priorities[item['name']] = 2
            item['recommended_priority'] = "2 (第二梯队)"
        else:
            priorities[item['name']] = 3
            item['recommended_priority'] = "3 (第三梯队)"
    
    return priorities, scores


def print_recommended_priorities(scores: List[Dict]) -> None:
    """打印推荐的优先级"""
    print("\n" + "=" * 100)
    print(f"{'插件名称':<15} {'综合得分':<12} {'平均结果':<10} {'成功率(%)':<12} "
          f"{'响应时间(ms)':<15} {'推荐优先级':<15}")
    print("-" * 100)
    
    for item in scores:
        print(f"{item['name']:<15} {item['score']:<12.2f} {item['avg_results']:<10.1f} "
              f"{item['success_rate']:<12.1f} {item['avg_time']:<15.1f} "
              f"{item['recommended_priority']:<15}")
    
    print("=" * 100)
    print("\n提示: 使用选项9应用推荐设置")


def show_menu():
    """显示菜单"""
    print("\n" + "=" * 50)
    print("           插件管理工具")
    print("=" * 50)
    print("1. 查看所有插件信息")
    print("2. 查看插件统计信息")
    print("3. 查看单个插件详情")
    print("4. 设置插件优先级")
    print("5. 批量设置优先级")
    print("6. 重置插件优先级")
    print("7. 导出统计数据")
    print("8. 查看推荐优先级")
    print("9. 应用推荐优先级")
    print("0. 退出")
    print("=" * 50)


def main():
    """主程序"""
    manager = PluginManager()
    
    while True:
        show_menu()
        choice = input("请选择操作: ").strip()
        
        try:
            if choice == "1":
                plugins = manager.get_all_plugins()
                print_plugins_table(plugins)
            
            elif choice == "2":
                stats = manager.get_plugin_stats()
                print_stats_table(stats)
            
            elif choice == "3":
                name = input("请输入插件名称: ").strip()
                stat = manager.get_plugin_detail(name)
                print_plugin_detail(stat)
            
            elif choice == "4":
                name = input("请输入插件名称: ").strip()
                priority = int(input("请输入优先级 (1-10): ").strip())
                result = manager.set_plugin_priority(name, priority)
                print(f"✓ {result['message']}")
            
            elif choice == "5":
                print("示例: pioz=1,gying=1,hdr4k=2")
                input_str = input("请输入优先级设置: ").strip()
                priorities = {}
                for item in input_str.split(','):
                    parts = item.split('=')
                    if len(parts) == 2:
                        priorities[parts[0].strip()] = int(parts[1].strip())
                
                result = manager.set_batch_priority(priorities)
                print(f"✓ 成功: {result['data']['success_count']} 个")
                if result['data']['failed_count'] > 0:
                    print(f"✗ 失败: {result['data']['failed_count']} 个")
                    for failed in result['data']['failed_plugins']:
                        print(f"  - {failed}")
            
            elif choice == "6":
                name = input("请输入插件名称: ").strip()
                result = manager.reset_plugin_priority(name)
                print(f"✓ {result['message']}")
            
            elif choice == "7":
                format_type = input("请输入格式 (json/csv): ").strip()
                output_file = input("请输入输出文件名: ").strip()
                manager.export_stats(format_type, output_file)
            
            elif choice == "8":
                stats = manager.get_plugin_stats()
                priorities, scores = calculate_recommended_priorities(stats)
                print_recommended_priorities(scores)
            
            elif choice == "9":
                stats = manager.get_plugin_stats()
                priorities, scores = calculate_recommended_priorities(stats)
                print_recommended_priorities(scores)
                
                confirm = input("\n确认应用推荐设置? (y/n): ").strip().lower()
                if confirm == 'y':
                    result = manager.set_batch_priority(priorities)
                    print(f"✓ 成功: {result['data']['success_count']} 个")
                    if result['data']['failed_count'] > 0:
                        print(f"✗ 失败: {result['data']['failed_count']} 个")
                else:
                    print("已取消")
            
            elif choice == "0":
                print("再见!")
                break
            
            else:
                print("无效选择")
        
        except requests.exceptions.RequestException as e:
            print(f"✗ 请求失败: {e}")
        except Exception as e:
            print(f"✗ 错误: {e}")
        
        input("\n按回车继续...")


if __name__ == "__main__":
    main()
