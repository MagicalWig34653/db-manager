#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Exercise embedded UI, API and SQLite persistence on Compose and isolated Swarm."""
import argparse
import json
import os
from pathlib import Path
import secrets
import socket
import subprocess
import sys
import tempfile

from web_test_support import ROOT, api, redact, register_secret, run, verify_http, verify_image, verify_persistence, wait_until


def free_port():
    with socket.socket() as listener:
        listener.bind(('127.0.0.1', 0))
        return listener.getsockname()[1]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--image', help='Use this test image tag instead of a unique task tag')
    parser.add_argument('--skip-build', action='store_true')
    parser.add_argument('--skip-swarm', action='store_true')
    args = parser.parse_args()
    project = 'dbmanager-f5b-deploy-' + secrets.token_hex(6)
    daemon = project + '-swarm'
    image = args.image or project + ':test'
    if args.skip_build and not args.image:
        parser.error('--skip-build requires --image')
    token = register_secret(secrets.token_urlsafe(48))
    image_created = False
    secret_root = ROOT / 'secrets'
    secret_root.mkdir(mode=0o700, exist_ok=True)
    try:
        if not args.skip_build:
            run(['docker', 'build', '-t', image, '.'], cwd=ROOT)
            image_created = True
        verify_image(image)
        with tempfile.TemporaryDirectory(prefix=project + '-', dir=secret_root) as temp:
            directory = Path(temp)
            token_path = directory / 'api_token'
            token_path.write_text(token + '\n')
            token_path.chmod(0o444)
            port = free_port()
            base = f'http://127.0.0.1:{port}'
            env = dict(os.environ, DBM_IMAGE=image, DBM_PORT=str(port), DBM_TOKEN_PATH=str(token_path))
            compose = ['docker', 'compose', '-f', str(ROOT / 'compose.yaml'), '-p', project]
            try:
                run(compose + ['up', '-d', '--no-build'], env=env)
                wait_until(lambda: api(base, '/healthz')['status'] == 'ok', 'Compose health')
                verify_http(base, token)
                container = run(compose + ['ps', '-q', 'app'], env=env).strip()
                wait_until(lambda: run(['docker', 'inspect', '--format', '{{.State.Health.Status}}', container]) == 'healthy', 'Compose Docker healthcheck')
                assert run(['docker', 'inspect', '--format', '{{.Config.User}}', container]) == '10001:10001'
                assert run(['docker', 'inspect', '--format', '{{.HostConfig.ReadonlyRootfs}}', container]) == 'true'
                verify_persistence(base, token, lambda: run(compose + ['restart', 'app'], env=env))
                verify_http(base, token)
                print('Compose: original embedded assets/notices, CSP, API/auth, healthcheck, non-root, read-only root and confirmed row CRUD/conflict/default/NULL/precision/binary and SQLite restart persistence passed.', flush=True)
            finally:
                run(compose + ['down', '--volumes', '--remove-orphans'], env=env)
            if args.skip_swarm:
                return
            daemon_created = False
            try:
                # Only this disposable nested daemon is privileged; the host Swarm is untouched.
                run(['docker', 'run', '-d', '--privileged', '--name', daemon, '-p', '127.0.0.1::8080', '-e', 'DOCKER_TLS_CERTDIR=', 'docker:29.5.2-dind'])
                daemon_created = True
                inner = ['docker', 'exec', daemon, 'docker']
                wait_until(lambda: bool(run(inner + ['info', '--format', '{{.ServerVersion}}'])), 'isolated Docker daemon')
                run(inner + ['swarm', 'init', '--advertise-addr', 'eth0'])
                node = run(inner + ['info', '--format', '{{.Swarm.NodeID}}'])
                run(inner + ['node', 'update', '--label-add', 'dbmanager.data=true', node])
                image_tar = directory / 'image.tar'
                run(['docker', 'save', '-o', str(image_tar), image])
                with image_tar.open('rb') as source:
                    result = subprocess.run(['docker', 'exec', '-i', daemon, 'docker', 'load'], stdin=source, capture_output=True)
                    if result.returncode:
                        raise RuntimeError('Isolated Docker image import failed')
                run(['docker', 'exec', '-i', daemon, 'docker', 'secret', 'create', 'dbmanager_api_token_v1', '-'], input=token)
                stack = (ROOT / 'deploy/swarm.yaml').read_text().replace('${DBM_IMAGE:-ghcr.io/magicalwig34653/db-manager:latest}', image).replace('${DBM_TOKEN_SECRET:-dbmanager_api_token_v1}', 'dbmanager_api_token_v1')
                run(['docker', 'exec', '-i', daemon, 'docker', 'stack', 'deploy', '--resolve-image', 'never', '--compose-file', '-', 'dbmanager'], input=stack)
                wait_until(lambda: run(inner + ['service', 'ls', '--format', '{{.Replicas}}']) == '1/1', 'Swarm service convergence')
                published = json.loads(run(['docker', 'inspect', '--format', '{{json .NetworkSettings.Ports}}', daemon]))['8080/tcp'][0]['HostPort']
                base = 'http://127.0.0.1:' + published
                wait_until(lambda: api(base, '/healthz')['status'] == 'ok', 'Swarm routing mesh HTTP health')
                verify_http(base, token)

                def task():
                    return run(inner + ['ps', '--filter', 'label=com.docker.swarm.service.name=dbmanager_app', '--format', '{{.ID}}'])

                original = task()
                assert original and '\n' not in original
                wait_until(lambda: run(inner + ['inspect', '--format', '{{.State.Health.Status}}', original]) == 'healthy', 'Swarm task healthcheck')

                def restart():
                    run(inner + ['service', 'update', '--force', '--detach', 'dbmanager_app'])
                    wait_until(lambda: bool(task()) and task() != original and '\n' not in task(), 'Swarm replacement task')
                    wait_until(lambda: run(inner + ['service', 'inspect', '--format', '{{.UpdateStatus.State}}', 'dbmanager_app']) == 'completed', 'Swarm update convergence')

                verify_persistence(base, token, restart)
                verify_http(base, token)
                print('Swarm: isolated manager, secret, placement, routing mesh, original UI/CSP/notices, API/auth and confirmed row CRUD/conflict/default/NULL/precision/binary and SQLite persistence across task replacement passed.', flush=True)
            finally:
                if daemon_created:
                    run(['docker', 'rm', '-fv', daemon])
    finally:
        # An explicitly named caller image remains owned by its caller.
        if image_created and args.image is None:
            run(['docker', 'image', 'rm', image])
    print('F5b deployment: task-owned containers, nested daemon, volumes, image and temporary credentials removed.', flush=True)


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        print(redact(error), file=sys.stderr)
        sys.exit(1)
