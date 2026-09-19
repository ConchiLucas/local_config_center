#!/usr/bin/env python3
"""Local-only demo server. Python 3.10+; standard library only.
Catalog reads use SQLite mode=ro. Test-generation adapters are deliberately absent.
No credentials, SQL input, external requests, or configuration write APIs.
"""
from __future__ import annotations
import argparse
import json
import mimetypes
import re
import sqlite3
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import unquote, urlsplit

ROOT = Path(__file__).resolve().parent
WEB = ROOT / 'web'
DB = ROOT / 'database' / 'demo.sqlite3'
MAX_BODY = 64 * 1024


def read_catalog() -> dict:
    con = sqlite3.connect(DB.as_uri() + '?mode=ro', uri=True)
    try:
        con.execute('PRAGMA query_only=ON')
        groups = []
        for gid, meta in con.execute('SELECT id,metadata_json FROM config_groups ORDER BY sort_order,id'):
            g = json.loads(meta)
            g['pages'] = []
            for pid, pmeta in con.execute('SELECT id,metadata_json FROM config_pages WHERE group_id=? ORDER BY sort_order,id', (gid,)):
                p = json.loads(pmeta)
                p['configs'] = [json.loads(r[0]) for r in con.execute('SELECT public_json FROM config_entries WHERE group_id=? AND page_id=? ORDER BY sort_order,id', (gid, pid))]
                g['pages'].append(p)
            groups.append(g)
        return {'schemaVersion': 1, 'demo': True, 'source': 'sqlite-demo', 'groups': groups,
                'notice': 'Only catalog reads are real. All media/test outputs are local fixtures, not provider results.'}
    finally:
        con.close()


class Handler(BaseHTTPRequestHandler):
    server_version = 'ReadonlyConfigDemo/1.0'

    def end_headers(self) -> None:
        self.send_header('X-Content-Type-Options', 'nosniff')
        self.send_header('Referrer-Policy', 'no-referrer')
        self.send_header('X-Frame-Options', 'DENY')
        super().end_headers()

    def send_json(self, code: int, value: dict, head: bool = False) -> None:
        data = json.dumps(value, ensure_ascii=False).encode('utf-8')
        self.send_response(code)
        self.send_header('Content-Type', 'application/json; charset=utf-8')
        self.send_header('Content-Length', str(len(data)))
        self.send_header('Cache-Control', 'no-store')
        self.end_headers()
        if not head:
            self.wfile.write(data)

    def do_HEAD(self) -> None:
        self.read_request(head=True)

    def do_GET(self) -> None:
        self.read_request(head=False)

    def read_request(self, head: bool) -> None:
        path = unquote(urlsplit(self.path).path)
        try:
            if path == '/api/catalog':
                return self.send_json(200, read_catalog(), head)
            if path.startswith('/api/configs/'):
                cid = path.split('/')[-1]
                for g in read_catalog()['groups']:
                    for p in g['pages']:
                        for c in p['configs']:
                            if c['id'] == cid:
                                return self.send_json(200, {'demo': True, 'config': c}, head)
                return self.send_json(404, {'error': 'CONFIG_NOT_FOUND'}, head)
            if path.startswith('/api/'):
                return self.send_json(404, {'error': 'ENDPOINT_NOT_IMPLEMENTED_IN_DEMO'}, head)
            target = (WEB / (path.lstrip('/') or 'index.html')).resolve()
            if not target.is_relative_to(WEB.resolve()) or not target.is_file():
                return self.send_json(404, {'error': 'NOT_FOUND'}, head)
            # Only serve static web files, never the SQLite database or source docs.
            size = target.stat().st_size
            start, end, code = 0, size - 1, 200
            range_header = self.headers.get('Range')
            if range_header:
                match = re.fullmatch(r'bytes=(\d*)-(\d*)', range_header.strip())
                if not match or not any(match.groups()):
                    return self.bad_range(size, head)
                left, right = match.groups()
                if left:
                    start = int(left)
                    end = min(int(right) if right else size - 1, size - 1)
                else:
                    suffix = int(right)
                    if suffix <= 0:
                        return self.bad_range(size, head)
                    start = max(0, size - suffix)
                if start >= size or start > end:
                    return self.bad_range(size, head)
                code = 206
            length = max(0, end - start + 1)
            content_type = mimetypes.guess_type(target.name)[0] or 'application/octet-stream'
            if target.suffix == '.js': content_type = 'application/javascript; charset=utf-8'
            elif target.suffix in ('.html', '.css'): content_type += '; charset=utf-8'
            self.send_response(code)
            self.send_header('Content-Type', content_type)
            self.send_header('Content-Length', str(length))
            self.send_header('Accept-Ranges', 'bytes')
            self.send_header('Cache-Control', 'no-cache')
            if code == 206:
                self.send_header('Content-Range', f'bytes {start}-{end}/{size}')
            self.end_headers()
            if not head:
                with target.open('rb') as f:
                    f.seek(start)
                    remaining = length
                    while remaining:
                        data = f.read(min(64 * 1024, remaining))
                        if not data: break
                        self.wfile.write(data)
                        remaining -= len(data)
        except (BrokenPipeError, ConnectionResetError):
            pass
        except (sqlite3.Error, OSError, ValueError) as exc:
            print(f'Demo read error: {exc}', file=sys.stderr)
            self.send_json(500, {'error': 'DEMO_READ_ERROR', 'message': 'Check the bundled SQLite demo database.'}, head)

    def bad_range(self, size: int, head: bool) -> None:
        self.send_response(416)
        self.send_header('Content-Range', f'bytes */{size}')
        self.send_header('Content-Length', '0')
        self.end_headers()

    def do_POST(self) -> None:
        # Never pretend to run a provider request. Production adapters must be added.
        self.close_connection = True
        if urlsplit(self.path).path == '/api/test-runs':
            self.send_json(501, {'error': 'ADAPTER_NOT_IMPLEMENTED', 'demo': True,
                'message': 'This kit only implements read-only catalog APIs. UI tests use labelled local fixtures.'})
        else:
            self.send_json(405, {'error': 'READ_ONLY_CONFIGURATION'})

    def do_PUT(self) -> None:
        self.close_connection = True
        self.send_json(405, {'error': 'READ_ONLY_CONFIGURATION'})

    do_PATCH = do_PUT
    do_DELETE = do_PUT


def main() -> None:
    parser = argparse.ArgumentParser(description='Read-only configuration UI prototype')
    parser.add_argument('--port', type=int, default=8765)
    args = parser.parse_args()
    if not DB.exists():
        raise SystemExit('Missing database/demo.sqlite3. Extract the whole ZIP before running.')
    if not 1 <= args.port <= 65535:
        raise SystemExit('Port must be between 1 and 65535.')
    server = ThreadingHTTPServer(('127.0.0.1', args.port), Handler)
    print(f'\nOpen http://127.0.0.1:{args.port}\nSQLite catalog: read-only. All tests: explicitly labelled local fixtures.\nCtrl+C to stop.\n', flush=True)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        server.server_close()

if __name__ == '__main__':
    main()
