"""Gitleaks active files + all reachable Git history; only safe metadata is emitted.

The history exception file contains exact known fingerprints, not secret values,
author names or emails. Exceptions acknowledge exposure; they do not prove rotation.
"""
from pathlib import Path
from tempfile import TemporaryDirectory
import json
import os
import shutil
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[1]


def scan(mode, source, report):
    binary = os.environ.get('GITLEAKS_BIN', 'gitleaks')
    args = [binary, mode, str(source), '--config', str(ROOT / '.gitleaks.toml'), '--redact=100', '--no-banner', '--log-level', 'error', '--report-format', 'json', '--report-path', str(report)]
    if mode == 'git': args += ['--log-opts=--all']
    result = subprocess.run(args, cwd=ROOT, capture_output=True)
    if result.returncode not in (0, 1) or not report.is_file():
        raise RuntimeError('Gitleaks did not complete; output suppressed to protect sensitive values')
    findings = json.loads(report.read_text(encoding='utf-8'))
    report.unlink()  # Raw report may include Git author metadata even with secret redaction.
    return findings


def main():
    with TemporaryDirectory() as temp:
        temp = Path(temp)
        source = temp / 'source'; source.mkdir()
        names = subprocess.check_output(['git', 'ls-files', '-z', '--cached', '--others', '--exclude-standard'], cwd=ROOT).decode().split('\0')
        for name in set(names) - {''}:
            path = ROOT / name
            if not path.is_file() or path.stat().st_size > 2_000_000: continue
            if b'\0' in path.read_bytes(): continue
            target = source / name; target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(path, target)
        active = scan('dir', source, temp / 'active.json')
        history = scan('git', ROOT, temp / 'history.json')
        known = {item['Fingerprint'] for item in json.loads((ROOT / 'security/known-history.json').read_text())}
        new = [item for item in history if item['Fingerprint'] not in known]
        for item in active + new:
            print(f"{item['File']}:{item['StartLine']}: {item['RuleID']}: [REDACTED]")
        print(f'Gitleaks active: {len(active)}; historical: {len(history)}; known exposed: {len(history)-len(new)}; new historical: {len(new)}. Rotation remains a separate operator task.')
        return bool(active or new)


if __name__ == '__main__':
    sys.exit(main())
