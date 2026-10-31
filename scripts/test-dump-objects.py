#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""DUMP-001 proof: SQLite in-process, MySQL 8.4 / MariaDB 12.3 / PostgreSQL 18.6 live.

Each system gets its own disposable dbfix container, one after another. The Go live
tests build a hostile catalog, dump it, restore it with the project splitter and
compare catalogs. This script then restores the same dumps with the engine's own
command line client (mysql/mariadb/psql) into a fresh database and compares the
catalogs read by that client with independent queries (the DELIMITER proof).
"""
import argparse
import os
from pathlib import Path
import subprocess
import tempfile

from dbfix import Fixtures, ROOT, PASSWORD

CATALOG_MYSQL = [
    "SELECT 'view',TABLE_NAME,VIEW_DEFINITION,CHECK_OPTION,SECURITY_TYPE FROM information_schema.VIEWS WHERE TABLE_SCHEMA='{db}' ORDER BY 2",
    "SELECT 'routine',ROUTINE_TYPE,ROUTINE_NAME,SHA2(ROUTINE_DEFINITION,256),IS_DETERMINISTIC,SQL_DATA_ACCESS,SECURITY_TYPE,SQL_MODE,ROUTINE_COMMENT FROM information_schema.ROUTINES WHERE ROUTINE_SCHEMA='{db}' ORDER BY 2,3",
    "SELECT 'trigger',TRIGGER_NAME,EVENT_MANIPULATION,EVENT_OBJECT_TABLE,ACTION_ORDER,SHA2(ACTION_STATEMENT,256),ACTION_TIMING,SQL_MODE FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA='{db}' ORDER BY 2",
    "SELECT 'event',EVENT_NAME,SHA2(EVENT_DEFINITION,256),EVENT_TYPE,EXECUTE_AT,INTERVAL_VALUE,INTERVAL_FIELD,STATUS,ON_COMPLETION,SQL_MODE,TIME_ZONE,STARTS FROM information_schema.EVENTS WHERE EVENT_SCHEMA='{db}' ORDER BY 2",
]
CATALOG_PG = [
    "SELECT 'view',n.nspname,c.relname,c.relkind,md5(pg_get_viewdef(c.oid,true)),COALESCE(array_to_string(c.reloptions,','),'') FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE c.relkind IN ('v','m') AND n.nspname IN ('public','app') ORDER BY 2,3",
    "SELECT 'routine',n.nspname,p.proname,pg_get_function_identity_arguments(p.oid),p.prokind,md5(pg_get_functiondef(p.oid)) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname IN ('public','app') ORDER BY 2,3,4",
    "SELECT 'trigger',t.tgname,c.relname,t.tgenabled,md5(pg_get_triggerdef(t.oid,true)) FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid WHERE NOT t.tgisinternal ORDER BY 2,3",
    "SELECT 'index',indexname,indexdef FROM pg_indexes WHERE schemaname='app' ORDER BY 2",
]


def docker(container, *args, stdin=None):
    return subprocess.run(['docker', 'exec', '-i', container, *args], input=stdin, capture_output=True, check=True)


def client(name, container, database, sql_bytes):
    """Run SQL through the engine's own command line client; returns stdout."""
    if name == 'postgres':
        cmd = ['env', 'PGPASSWORD=' + PASSWORD, 'psql', '-h', '127.0.0.1', '-U', 'postgres', '-v', 'ON_ERROR_STOP=1', '-At', '-d', database]
    else:
        binary = 'mysql' if name == 'mysql' else 'mariadb'
        cmd = [binary, '-h127.0.0.1', '-uroot', '-p' + PASSWORD, '--default-character-set=utf8mb4', '-N', '-B', '--skip-ssl' if name == 'maria' else '--ssl-mode=DISABLED']
        if database:
            cmd.append(database)
    return docker(container, *cmd, stdin=sql_bytes).stdout.decode()


def catalog(name, container, database):
    queries = CATALOG_PG if name == 'postgres' else [q.format(db=database) for q in CATALOG_MYSQL]
    return '\n'.join(client(name, container, database, (q + ';\n').encode()) for q in queries)


def create_db(name, container, database):
    if name == 'postgres':
        client(name, container, 'probe', f'DROP DATABASE IF EXISTS {database};CREATE DATABASE {database};'.encode())
    else:
        client(name, container, '', f'DROP DATABASE IF EXISTS {database};CREATE DATABASE {database};'.encode())


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--systems', default='mysql,maria,postgres')
    args = parser.parse_args()
    env = os.environ.copy()
    env['GOPROXY'] = 'off'
    env['GOSUMDB'] = 'off'
    for key in list(env):
        if key.startswith('DBM_TEST_'):
            del env[key]
    subprocess.run(['go', 'test', '-race', '-count=1', './internal/core', './internal/database', './internal/httpapi', '-run', 'Object|Export'], cwd=ROOT, env=env, check=True)
    for system in args.systems.split(','):
        with tempfile.TemporaryDirectory(prefix='dbmanager-dumpobj-') as scratch, Fixtures('dump-objects', systems=(system,)) as fx:
            live = fx.environment(env)
            live['DBM_DUMP_OBJECTS_DIR'] = scratch
            container = fx.prefix + '-' + system
            subprocess.run(['go', 'test', '-race', '-count=1', '-v', './internal/database', '-run', 'TestLiveExportObjects/' + system + '$|TestLiveExportObjectsFailures/' + system + '$|TestLiveExportObjectsCatalogDump/' + system + '$'], cwd=ROOT, env=live, check=True)
            dump = Path(scratch) / (system + '-objects.sql')
            # Independent client restore: the source database is dropped and recreated under
            # the same name (view bodies are schema qualified), then restored with the
            # engine's own command line client and compared with independent queries.
            src = catalog(system, container, 'objsrc')
            assert src.strip(), 'empty source catalog'
            create_db(system, container, 'objsrc')
            assert not catalog(system, container, 'objsrc').strip(), 'recreated database is not empty'
            client(system, container, 'objsrc', dump.read_bytes())
            assert catalog(system, container, 'objsrc') == src, 'client restore catalog differs for ' + system
            # DROP variant on top of the restored database, then compare once more.
            client(system, container, 'objsrc', (Path(scratch) / (system + '-objects-drop.sql')).read_bytes())
            assert catalog(system, container, 'objsrc') == src, 'drop variant differs for ' + system
            dst = src
            print(system + ': client restore catalog equal (' + str(len(src.splitlines())) + ' catalog lines)')
        assert not fx.leftovers(), fx.leftovers()
    print('DUMP-001 live proof passed')


if __name__ == '__main__':
    main()
