"""Scan active tracked/unignored text without ever printing matched values.

Gitleaks adds its maintained rules; this narrow rule catches low-entropy libpq
passwords missed by generic entropy checks. History is scanned separately.
"""
from pathlib import Path
import re
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[1]
RULES = {
    'libpq-password': re.compile(r'(?i)\bpassword\s*=\s*["\']?([^\s"\'<>]+)'),
    'postgres-url-password': re.compile(r'postgres(?:ql)?://[^\s/:]+:([^@\s]+)@'),
    'private-key': re.compile(r'-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----'),
}
PLACEHOLDERS = {'CHANGE_ME', 'REDACTED', 'PLACEHOLDER', '${DB_PASSWORD}'}


def main():
    paths = subprocess.check_output(['git', 'ls-files', '-z', '--cached', '--others', '--exclude-standard'], cwd=ROOT).decode().split('\0')
    findings = 0
    for name in sorted(set(paths) - {''}):
        path = ROOT / name
        if not path.is_file() or path.stat().st_size > 2_000_000:
            continue
        raw = path.read_bytes()
        if b'\0' in raw:
            continue
        for line_no, line in enumerate(raw.decode('utf-8', errors='replace').splitlines(), 1):
            for rule, pattern in RULES.items():
                for match in pattern.finditer(line):
                    value = match.group(1) if match.lastindex else match.group()
                    if value in PLACEHOLDERS:
                        continue
                    print(f'{name}:{line_no}: {rule}: [REDACTED]')
                    findings += 1
    print(f'Active source scan: {findings} finding(s); values redacted.')
    return bool(findings)


if __name__ == '__main__':
    sys.exit(main())
