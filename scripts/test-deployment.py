#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Exercise Compose and an isolated Swarm daemon without changing the host Swarm."""
import argparse
import json
import os
from pathlib import Path
import secrets
import socket
import subprocess
import tempfile
import time
import urllib.error
import urllib.request

ROOT = Path(__file__).resolve().parent.parent


def run(args, **kwargs):
    return subprocess.run(args, check=True, text=True, capture_output=True, **kwargs).stdout


def wait_until(check, description):
    for _ in range(60):
        try:
            if check():
                return
        except (subprocess.CalledProcessError, urllib.error.URLError, OSError, ValueError):
            pass
        time.sleep(1)
    raise RuntimeError('Timed out: ' + description)


def verify_http(base, token):
    assert json.load(urllib.request.urlopen(base + '/healthz', timeout=3))['status'] == 'ok'
    try:
        urllib.request.urlopen(base + '/api/v1/capabilities', timeout=3)
    except urllib.error.HTTPError as error:
        assert error.code == 401
    else:
        raise AssertionError('Unauthenticated API access')
    request = urllib.request.Request(base + '/api/v1/capabilities', headers={'Authorization': 'Bearer ' + token})
    assert json.load(urllib.request.urlopen(request, timeout=3))['contract_version'] == '0.1.0'


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--image', default='db-manager:deployment-test')
    parser.add_argument('--skip-build', action='store_true')
    parser.add_argument('--skip-swarm', action='store_true')
    args = parser.parse_args()
    suffix = secrets.token_hex(8)
    project = 'dbmanager-test-' + suffix
    daemon = project + '-swarm'
    token = secrets.token_urlsafe(48)
    if not args.skip_build:
        run(['docker', 'build', '-t', args.image, '.'], cwd=ROOT)
    secret_root = ROOT / 'secrets'
    secret_root.mkdir(mode=0o700, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix='deployment-', dir=secret_root) as temp:
        directory = Path(temp)
        token_path = directory / 'api_token'
        token_path.write_text(token + '\n')
        token_path.chmod(0o444)
        with socket.socket() as listener:
            listener.bind(('127.0.0.1', 0))
            port = listener.getsockname()[1]
        env = dict(os.environ, DBM_IMAGE=args.image, DBM_PORT=str(port), DBM_TOKEN_PATH=str(token_path))
        compose = ['docker', 'compose', '-f', str(ROOT / 'compose.yaml'), '-p', project]
        try:
            run(compose + ['up', '-d', '--no-build'], env=env)
            wait_until(lambda: json.load(urllib.request.urlopen(f'http://127.0.0.1:{port}/healthz', timeout=2))['status'] == 'ok', 'Compose health')
            verify_http(f'http://127.0.0.1:{port}', token)
            container = run(compose + ['ps', '-q', 'app'], env=env).strip()
            wait_until(lambda: run(['docker', 'inspect', '--format', '{{.State.Health.Status}}', container]).strip() == 'healthy', 'Compose Docker healthcheck')
            assert run(['docker', 'inspect', '--format', '{{.Config.User}}', container]).strip() == '10001:10001'
            assert run(['docker', 'inspect', '--format', '{{.HostConfig.ReadonlyRootfs}}', container]).strip() == 'true'
            print('Compose: HTTP/auth, Docker healthcheck, non-root and read-only root passed.', flush=True)
        finally:
            run(compose + ['down', '--volumes', '--remove-orphans'], env=env)
        if args.skip_swarm:
            return
        daemon_created = False
        try:
            run(['docker', 'run', '-d', '--privileged', '--name', daemon, '-e', 'DOCKER_TLS_CERTDIR=', 'docker:29.5.2-dind'])
            daemon_created = True
            inner = ['docker', 'exec', daemon, 'docker']
            wait_until(lambda: bool(run(inner + ['info', '--format', '{{.ServerVersion}}']).strip()), 'isolated Docker daemon')
            run(inner + ['swarm', 'init', '--advertise-addr', 'eth0'])
            node = run(inner + ['info', '--format', '{{.Swarm.NodeID}}']).strip()
            run(inner + ['node', 'update', '--label-add', 'dbmanager.data=true', node])
            image_tar = directory / 'image.tar'
            run(['docker', 'save', '-o', str(image_tar), args.image])
            with image_tar.open('rb') as source:
                subprocess.run(['docker', 'exec', '-i', daemon, 'docker', 'load'], stdin=source, check=True, capture_output=True)
            run(['docker', 'exec', '-i', daemon, 'docker', 'secret', 'create', 'dbmanager_api_token_v1', '-'], input=token)
            stack = (ROOT / 'deploy/swarm.yaml').read_text().replace('${DBM_IMAGE:-db-manager:local}', args.image).replace('${DBM_TOKEN_SECRET:-dbmanager_api_token_v1}', 'dbmanager_api_token_v1')
            run(['docker', 'exec', '-i', daemon, 'docker', 'stack', 'deploy', '--resolve-image', 'never', '--compose-file', '-', 'dbmanager'], input=stack)
            wait_until(lambda: run(inner + ['service', 'ls', '--format', '{{.Replicas}}']).strip() == '1/1', 'Swarm service convergence')
            health = ['docker', 'exec', daemon, 'wget', '-qO-', 'http://127.0.0.1:8080/healthz']
            wait_until(lambda: json.loads(run(health))['status'] == 'ok', 'Swarm routing mesh HTTP health')
            # Feed the header through stdin, keeping the token out of process arguments.
            command = 'read -r token; wget -qO- --header="Authorization: Bearer $token" http://127.0.0.1:8080/api/v1/capabilities'
            result = run(['docker', 'exec', '-i', daemon, 'sh', '-c', command], input=token + '\n')
            assert json.loads(result)['contract_version'] == '0.1.0'
            containers = run(inner + ['ps', '--filter', 'label=com.docker.swarm.service.name=dbmanager_app', '--format', '{{.ID}}']).strip().splitlines()
            assert len(containers) == 1
            wait_until(lambda: run(inner + ['inspect', '--format', '{{.State.Health.Status}}', containers[0]]).strip() == 'healthy', 'Swarm task healthcheck')
            print('Swarm: isolated manager, secret, labeled placement, routing mesh, auth and healthcheck passed.', flush=True)
        finally:
            # Only this uniquely named test daemon and its anonymous volumes are removed.
            if daemon_created:
                subprocess.run(['docker', 'rm', '-fv', daemon], capture_output=True, check=False)
    print('Task-owned containers, networks, volumes and temporary files removed.', flush=True)


if __name__ == '__main__':
    main()
