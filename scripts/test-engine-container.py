#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Verify real SQLite SQL, read-only root, UID and restart persistence in Compose."""
import json
import os
from pathlib import Path
import secrets
import socket
import subprocess
import tempfile
import time
import urllib.request

ROOT=Path(__file__).resolve().parent.parent

def main():
    subprocess.run(['docker','ps','--format','{{.Names}} {{.Status}}'],check=True)
    suffix=secrets.token_hex(6)
    project='dbmanager-engine-container-'+suffix
    image='dbmanager-engine:'+suffix
    secret_root=ROOT/'secrets'
    secret_root.mkdir(exist_ok=True,mode=0o700)
    token=secrets.token_urlsafe(48)
    with tempfile.TemporaryDirectory(dir=secret_root,prefix=project) as temporary:
        path=Path(temporary)/'token';path.write_text(token);path.chmod(0o444)
        with socket.socket() as listener:
            listener.bind(('127.0.0.1',0));port=listener.getsockname()[1]
        environment=dict(os.environ,DBM_IMAGE=image,DBM_PORT=str(port),DBM_TOKEN_PATH=str(path))
        compose=['docker','compose','-f',str(ROOT/'compose.yaml'),'-p',project]
        def run(args):return subprocess.check_output(args,cwd=ROOT,env=environment,text=True).strip()
        def request(route,body=None):
            data=None if body is None else json.dumps(body).encode()
            req=urllib.request.Request(f'http://127.0.0.1:{port}'+route,data=data,headers={'Authorization':'Bearer '+token,'Content-Type':'application/json'})
            with urllib.request.urlopen(req,timeout=10) as response:return json.load(response)
        def healthy():
            for _ in range(60):
                try:
                    if request('/healthz')['status']=='ok':return
                except (OSError,ValueError):pass
                time.sleep(.5)
            raise RuntimeError('Engine container startup timeout')
        def query(sql):return request('/api/v1/query',{'connection':{'driver':'sqlite','database':'fixture'},'sql':sql})
        try:
            subprocess.run(['docker','build','-t',image,'.'],cwd=ROOT,check=True,stdout=subprocess.DEVNULL)
            run(compose+['up','-d','--no-build']);healthy()
            container=run(compose+['ps','-q','app'])
            assert run(['docker','inspect','--format','{{.Config.User}}',container])=='10001:10001'
            assert run(['docker','inspect','--format','{{.HostConfig.ReadonlyRootfs}}',container])=='true'
            assert set(request('/api/v1/capabilities')['drivers'])=={'sqlite','mysql','postgres'}
            query('CREATE TABLE saved(value TEXT)')
            query("INSERT INTO saved VALUES('Neustart ✓')")
            assert query('SELECT value FROM saved')['results'][0]['rows'][0][0]=={'kind':'text','value':'Neustart ✓'}
            run(compose+['restart','app']);healthy()
            assert query('SELECT value FROM saved')['results'][0]['rows'][0][0]['value']=='Neustart ✓'
            print('Engine container: real SQLite SQL, UID 10001, read-only root and restart persistence passed.')
        finally:
            subprocess.run(compose+['down','--volumes','--remove-orphans'],cwd=ROOT,env=environment,check=True,stdout=subprocess.DEVNULL)
            subprocess.run(['docker','image','rm',image],check=True,stdout=subprocess.DEVNULL)

if __name__=='__main__':main()
