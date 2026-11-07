"""Disposable synthetic backup/restore drill. Requires local PG client tools.

Uses PGHOST/PGPORT/PGUSER/PGPASSFILE (never prints connection settings).
Creates unique _test databases; never reads or drops an existing database.
API_BINARY points to a freshly built executable. No production data is used.
"""
from pathlib import Path
from tempfile import TemporaryDirectory
import json
import os
import subprocess
import urllib.request
import time
import uuid
import socket

ROOT = Path(__file__).resolve().parents[1]


def command(args, env=None):
    result = subprocess.run(args, env=env, capture_output=True)
    if result.returncode:
        # pg tools may echo connection details. Keep stderr out of logs.
        raise RuntimeError(f'{Path(args[0]).stem} failed; inspect locally with restricted access')
    return result.stdout.decode().strip()


def sql(db, query):
    return command(['psql', '-X', '-v', 'ON_ERROR_STOP=1', '-At', '-d', db, '-c', query])


def main():
    if os.environ.get('PGHOST') not in ('127.0.0.1', 'localhost', '::1'):
        raise RuntimeError('Restore drill requires explicit loopback PGHOST')
    binary = str(Path(os.environ['API_BINARY']).resolve())
    suffix = uuid.uuid4().hex[:12]
    source, target = f'drill_source_{suffix}_test', f'drill_restored_{suffix}_test'
    created = []
    try:
        with TemporaryDirectory() as temp:
            for db in (source, target):
                command(['createdb', db]); created.append(db)
            # A legacy schema proves the restored data also accepts migrations.
            sql(source, "CREATE TABLE guestbooks(id bigserial PRIMARY KEY,name text,message text,created_at timestamptz); INSERT INTO guestbooks(name,message,created_at) VALUES ('synthetic','<b>restore fixture</b>','2020-01-01T00:00:00Z'),('synthetic two','restore fixture two','2020-01-02T00:00:00Z')")
            dump = str(Path(temp) / 'synthetic.dump')
            command(['pg_dump', '-Fc', '--no-owner', '--no-acl', '-f', dump, source])
            command(['pg_restore', '--exit-on-error', '--no-owner', '--no-acl', '-d', target, dump])
            if sql(source, 'SELECT row_to_json(g)::text FROM guestbooks g ORDER BY id') != sql(target, 'SELECT row_to_json(g)::text FROM guestbooks g ORDER BY id'):
                raise RuntimeError('Restored synthetic rows differ')
            if sql(target, 'SELECT count(*) FROM guestbooks') != '2':
                raise RuntimeError('Restored row count differs')
            env = {k: v for k, v in os.environ.items() if not k.startswith('PG')}
            if 'PGPASSFILE' in os.environ:
                env['PGPASSFILE'] = os.environ['PGPASSFILE']
            host = os.environ['PGHOST']
            if host == '::1': host = '[::1]'
            # The disposable database is accessed with a test role; credentials stay in pgpass.
            env.update(APP_ENV='development', DATABASE_URL=f"postgres://{os.environ['PGUSER']}@{host}:{os.environ.get('PGPORT','5432')}/{target}?sslmode=disable", STATIC_DIR=str(ROOT / 'dev/dist'), MODERATOR_ID='synthetic-drill')
            command([binary, 'migrate'], env)
            command([binary, 'migrate'], env)
            if sql(target, "SELECT count(*) FROM guestbooks WHERE status='pending'") != '2':
                raise RuntimeError('Restored legacy moderation default differs')
            command([binary, 'moderate', 'approve', '1'], env)
            with socket.socket() as reservation:
                reservation.bind(('127.0.0.1', 0)); port = reservation.getsockname()[1]
            env['LISTEN_ADDR'] = f'127.0.0.1:{port}'
            with open(os.devnull, 'wb') as quiet:
                server = subprocess.Popen([binary], env=env, stdout=quiet, stderr=quiet)
                try:
                    ready = False
                    for _ in range(50):
                        try:
                            with urllib.request.urlopen(f'http://127.0.0.1:{port}/api/ready', timeout=1) as response:
                                ready = response.status == 200
                            if ready: break
                        except OSError:
                            time.sleep(0.1)
                    if not ready: raise RuntimeError('Restored application did not become ready')
                    with urllib.request.urlopen(f'http://127.0.0.1:{port}/api/guestbook', timeout=3) as response:
                        page = json.load(response)
                    if len(page['items']) != 1 or page['items'][0]['id'] != 1:
                        raise RuntimeError('Restored public moderation query differs')
                finally:
                    server.terminate(); server.wait(timeout=15)
            print('Restore drill PASS: 2 identical synthetic rows; legacy migration repeatable; 2 pending; approve/readiness/public GET verified.')
    finally:
        # Only names successfully created during this invocation are ever removed.
        for db in reversed(created):
            command(['dropdb', db])


if __name__ == '__main__':
    main()
