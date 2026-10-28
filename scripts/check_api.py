#!/usr/bin/env python3
"""Exercise the real HTTP server, restart recovery, and MongoDB CLI inspection."""
import argparse
import json
import os
from pathlib import Path
import signal
import subprocess
import tempfile
import time
import urllib.error
import urllib.request

ROOT = Path(__file__).resolve().parents[1]
parser = argparse.ArgumentParser()
parser.add_argument('--backend', choices=['bbolt', 'mongodb'], default='bbolt')
parser.add_argument('--port', type=int, default=18080)
parser.add_argument('--mongo-container', default='pj2-nsf-mongo')
args = parser.parse_args()
base = f'http://127.0.0.1:{args.port}/sbi'
binary = str(os.environ.get('NSF_BINARY', ROOT / 'build/nsf'))


def show(message):
    print(message, flush=True)
    time.sleep(float(os.environ.get("NSF_DEMO_PAUSE", "0")))


def request(method, route, body=None, expected=200):
    show(f'$ {method} {route}' + (f' {json.dumps(body)}' if body is not None else ''))
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(base + route, data=data, method=method,
                                 headers={'Content-Type': 'application/json'})
    try:
        with urllib.request.urlopen(req, timeout=10) as response:
            status, raw = response.status, response.read()
    except urllib.error.HTTPError as error:
        status, raw = error.code, error.read()
    result = json.loads(raw) if raw else None
    visible = result
    if route == '/nnsf-nfm' and method == 'POST' and status == 200:
        assert result['service_token'] and result['service_uuid']
        visible = {'service_token_present': True, 'service_uuid': result['service_uuid']}
    show(f'HTTP {status} {json.dumps(visible)}')
    assert status == expected, (status, result)
    return result


with tempfile.TemporaryDirectory(prefix='pj2-api-') as directory:
    config = ROOT / 'config-mongodb.yaml'
    if args.backend == 'bbolt':
        # The supplied bbolt backend uses a fixed path. Refuse to touch existing data.
        if Path('/tmp/nsf.db').exists():
            raise SystemExit('Existing /tmp/nsf.db found; move it aside before this isolated test.')
        config = Path(directory) / 'config.yaml'
        config.write_text(f'nsf:\n  sbi_address: "127.0.0.1"\n  sbi_port: {args.port}\n'
                          '  db_type: "bbolt"\nlogger:\n  level: "error"\n')
    elif args.port != 18080:
        raise SystemExit('config-mongodb.yaml uses port 18080')
    env = os.environ.copy()
    if args.backend == 'mongodb':
        env.setdefault('NSF_MONGO_URI', 'mongodb://127.0.0.1:27019')
        env.setdefault('NSF_MONGO_DATABASE', f'pj2_nsf_demo_{os.getpid()}')
    show('PJ2 HTTP integration / DB interface demonstration')
    show(f'backend={args.backend}, port={args.port}')
    show('$ cat ' + str(config))
    show(config.read_text().rstrip())
    proc = None
    server_log = open(Path(directory) / 'server.log', 'w+')

    def stop():
        global proc
        if proc is not None:
            proc.send_signal(signal.SIGTERM)
            try:
                proc.wait(timeout=10)
            except subprocess.TimeoutExpired:
                proc.kill()
                proc.wait()
            proc = None

    def start():
        global proc
        show('$ ' + binary + ' -c ' + str(config))
        proc = subprocess.Popen([binary, '-c', str(config)],
                                cwd=ROOT, env=env, stdout=server_log, stderr=subprocess.STDOUT)
        for _ in range(100):
            if proc.poll() is not None:
                server_log.seek(0)
                raise RuntimeError(server_log.read())
            try:
                with urllib.request.urlopen(base + '/heartbeat', timeout=1) as response:
                    if response.status == 204:
                        return
            except (urllib.error.URLError, TimeoutError):
                time.sleep(0.1)
        raise RuntimeError('NSF startup timeout')

    try:
        start()
        request('GET', '/heartbeat', expected=204)
        request('GET', '/nnsf-disc', {'service': 'absent'}, expected=404)
        request('POST', '/nnsf-nfm', {'nf_name': 'PJ2-NF', 'sbi_address': '127.0.0.2',
                                    'sbi_port': 8081, 'services': ['pj2-a', 'pj2-b']})
        if args.backend == 'mongodb':
            database = env['NSF_MONGO_DATABASE']
            query = f'db.getSiblingDB({json.dumps(database)}).services.find().forEach(d => printjson(d))'
            cmd = ['docker', 'exec', args.mongo_container, 'mongosh', '--quiet', '--eval', query]
            show('$ docker exec ' + args.mongo_container + ' mongosh --quiet --eval ' + repr(query))
            output = subprocess.check_output(cmd, text=True)
            show(output.rstrip())
            assert 'PJ2-NF' in output and 'pj2-a' in output
            show('PASS: registration is stored in MongoDB and visible through mongosh')
        result = request('GET', '/nnsf-disc/group', {'services': ['pj2-b', 'pj2-a']})
        assert [item['service'] for item in result['services']] == ['pj2-b', 'pj2-a']
        show('$ stop NSF and restart with the same database')
        stop()
        start()
        result = request('GET', '/nnsf-disc', {'service': 'pj2-a'})
        assert result == {'service': 'pj2-a', 'nf_name': 'PJ2-NF',
                          'sbi_address': '127.0.0.2', 'sbi_port': 8081}
        request('DELETE', '/nnsf-nfm', {'nf_name': 'PJ2-NF', 'services': ['pj2-a']}, expected=204)
        request('GET', '/nnsf-disc', {'service': 'pj2-a'}, expected=404)
        request('GET', '/nnsf-disc', {'service': 'pj2-b'})
        request('DELETE', '/nnsf-nfm', {'nf_name': 'PJ2-NF', 'services': ['pj2-b']}, expected=204)
        stop()
        start()
        request('GET', '/nnsf-disc', {'service': 'pj2-b'}, expected=404)
        show('PASS: heartbeat, registration, group discovery, restart recovery, partial deletion, durable deletion')
    finally:
        stop()
        server_log.close()
        if args.backend == 'bbolt':
            Path('/tmp/nsf.db').unlink(missing_ok=True)
