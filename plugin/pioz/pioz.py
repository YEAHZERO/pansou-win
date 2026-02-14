#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Pioz Plugin - Next.js流式渲染版"""

import sys
import json
import re
from typing import List, Dict, Any
from urllib.parse import quote
import requests

try:
    from bs4 import BeautifulSoup
    HAS_BS4 = True
except ImportError:
    HAS_BS4 = False


class PiozPlugin:
    SITE_URL = "https://www.pioz.cn"
    
    LINK_PATTERNS = {
        'quark': r'https?://pan\.quark\.cn/s/[0-9a-zA-Z]{6,}',
        'baidu': r'https?://pan\.baidu\.com/s/[0-9a-zA-Z_\-]+',
        'aliyun': r'https?://(?:aliyundrive\.com|alipan\.com)/s/[0-9a-zA-Z]+',
        'uc': r'https?://drive\.uc\.cn/s/[0-9a-zA-Z]+',
        'xunlei': r'https?://pan\.xunlei\.com/s/[0-9a-zA-Z_\-]+',
        'magnet': r'magnet:\?xt=urn:btih:[0-9a-fA-F]{40}',
    }
    
    def __init__(self):
        self.session = requests.Session()
        self.session.headers.update({
            'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0.0.0',
            'Accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8',
            'Accept-Language': 'zh-CN,zh;q=0.9,en;q=0.8',
        })
    
    def search(self, keyword: str) -> List[Dict[str, Any]]:
        results = self._nextjs_search(keyword)
        if not results:
            results = self._html_search(keyword)
        return results[:20]
    
    def _nextjs_search(self, keyword: str) -> List[Dict]:
        try:
            url = f"{self.SITE_URL}/search?q={quote(keyword)}"
            resp = self.session.get(url, timeout=15)
            
            # Extract individual result objects from the HTML
            # Pattern: \"id\":123,\"title\":\"...\",\"download_url\":\"...\"
            results = []
            seen_ids = set()
            
            # Find all result objects
            pattern = r'\\?"id\\?":\s*(\d+),\\?"title\\?":\\?"([^"\\]+)\\?"[^}]*\\?"download_url\\?":\\?"([^"\\]+)\\?"'
            
            for match in re.finditer(pattern, resp.text):
                rid = match.group(1)
                title = match.group(2).replace('\\"', '"')
                download_url = match.group(3).replace('\\/', '/')
                
                if rid in seen_ids:
                    continue
                seen_ids.add(rid)
                
                link_type = self._detect_link_type(download_url) or 'quark'
                
                results.append({
                    'unique_id': f"pioz-{rid}",
                    'title': title,
                    'content': f"来源: pioz.cn/detail/{rid}",
                    'links': [{
                        'type': link_type,
                        'url': download_url,
                        'password': '',
                    }],
                    'channel': '',
                    'tags': [],
                    'images': [],
                    'resource_id': rid,
                })
            
            return results
        except Exception as e:
            print(f"[pioz] nextjs error: {e}", file=sys.stderr)
        return []
    
    def _html_search(self, keyword: str) -> List[Dict]:
        try:
            url = f"{self.SITE_URL}/search?q={quote(keyword)}"
            resp = self.session.get(url, timeout=15)
            
            results = []
            seen = set()
            
            if HAS_BS4:
                soup = BeautifulSoup(resp.text, 'html.parser')
                for a in soup.select('a[href*="/detail/"]'):
                    href = a.get('href', '')
                    match = re.search(r'/detail/(\d+)', href)
                    if not match:
                        continue
                    
                    rid = match.group(1)
                    if rid in seen:
                        continue
                    seen.add(rid)
                    
                    strings = list(a.stripped_strings)
                    title = strings[1] if len(strings) > 1 else (strings[0] if strings else f"Resource {rid}")
                    
                    results.append({
                        'unique_id': f"pioz-{rid}",
                        'title': title,
                        'content': f"来源: pioz.cn/detail/{rid}",
                        'links': [],
                        'channel': '',
                        'tags': [],
                        'images': [],
                        'resource_id': rid,
                    })
            else:
                for match in re.finditer(r'<a href="/detail/(\d+)"[^>]*>(.*?)</a>', resp.text, re.DOTALL):
                    rid, content = match.groups()
                    if rid in seen:
                        continue
                    seen.add(rid)
                    
                    text = re.sub(r'<[^>]+>', ' ', content)
                    text = ' '.join(text.split()).strip()
                    
                    results.append({
                        'unique_id': f"pioz-{rid}",
                        'title': text[:50],
                        'content': f"来源: pioz.cn/detail/{rid}",
                        'links': [],
                        'channel': '',
                        'tags': [],
                        'images': [],
                        'resource_id': rid,
                    })
            
            return results
        except Exception as e:
            print(f"[pioz] html error: {e}", file=sys.stderr)
        return []
    
    def _detect_link_type(self, url: str) -> str:
        for link_type, pattern in self.LINK_PATTERNS.items():
            if re.search(pattern, url):
                return link_type
        return 'other'


def main():
    keyword = sys.argv[1] if len(sys.argv) > 1 else ""
    plugin = PiozPlugin()
    results = plugin.search(keyword)
    print(json.dumps(results, ensure_ascii=False))


if __name__ == '__main__':
    main()
