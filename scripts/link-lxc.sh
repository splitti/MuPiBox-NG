#!/bin/sh
# Link the already mirrored LXC project to the verified remote branch.
# Existing files must match the branch. Nothing is overwritten on a mismatch.
set -eu
cd "$(dirname "$0")/.."
target=$(pwd -P)
if [ -e .git ]; then
 echo 'This directory already has Git metadata. Inspect git status; no changes made.'
 exit 0
fi
command -v python3 >/dev/null
parent=$(dirname "$target")
staging=$(mktemp -d "$parent/.mupibox-git-link.XXXXXX")
trap 'rm -rf -- "$staging"' EXIT HUP INT TERM
git clone --single-branch --branch rebuild/go-foundation https://github.com/splitti/MuPiBox-NG.git "$staging/checkout"
python3 - "$target" "$staging/checkout" <<'PY'
import os,pathlib,shutil,stat,subprocess,sys
root,checkout=map(pathlib.Path,sys.argv[1:])
paths=subprocess.check_output(['git','-C',str(checkout),'ls-files','-z']).decode().split('\0')
files=[pathlib.Path(p) for p in paths if p]
for rel in files:
 dest=root/rel
 # Reject symlinks and path conflicts before copying any source file.
 for parent in [dest,*dest.parents]:
  if parent==root: break
  if parent.is_symlink(): raise SystemExit(f'Abort: symlink conflict: {parent}')
  if parent!=dest and parent.exists() and not parent.is_dir(): raise SystemExit(f'Abort: directory conflict: {parent}')
 if dest.exists() and (not dest.is_file() or dest.read_bytes()!=(checkout/rel).read_bytes()):
  raise SystemExit(f'Abort: local file differs: {rel}. Preserve/reconcile it first.')
for rel in files:
 dest=root/rel
 if not dest.exists():
  dest.parent.mkdir(parents=True,exist_ok=True)
  shutil.copy2(checkout/rel,dest)
 os.chmod(dest,stat.S_IMODE((checkout/rel).stat().st_mode))
# The verified index matches all tracked content. Test file/music/config stay untouched.
os.rename(checkout/'.git',root/'.git')
PY
git status --short --branch
printf '\nLinked checkout: %s\n' "$target"
