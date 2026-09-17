#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd); cd "$root"
tmp=$(mktemp "${TMPDIR:-/tmp}/motion-compose.XXXXXX"); trap 'rm -f "$tmp"' EXIT HUP INT TERM
POSTGRES_PASSWORD=test LOBEHUB_SSO_SHARED_SECRET=test docker compose -f deploy/docker-compose.build.yml config --format json >"$tmp"
python3 - "$tmp" deploy/intel-check-motion/smoke-pass.html <<'PY'
import json,re,sys
c=json.load(open(sys.argv[1]));h=open(sys.argv[2]).read();s=c['services'];a=s['sub2api'];m=s['intel-check-motion'];n='intel-check-motion-network'
assert set(s)=={'sub2api','postgres','redis','intel-check-motion'}
assert a['environment']['INTEL_CHECK_MOTION_ENABLED']=='true' and a['environment']['INTEL_CHECK_MOTION_EVALUATOR_URL']=='http://intel-check-motion:8081' and a['environment']['INTEL_CHECK_MOTION_TIMEOUT_SECONDS']=='8'
assert a['depends_on']['intel-check-motion']['condition']=='service_healthy' and set(a['networks'])=={'sub2api-network',n}
assert set(m['networks'])=={n} and c['networks'][n]['internal'] is True and 'ports' not in m and 'volumes' not in m and 'depends_on' not in m
assert m['read_only'] is True and m['init'] is True and m['user']=='pwuser' and m['security_opt']==['no-new-privileges:true'] and m['cap_drop']==['ALL'] and m['pids_limit']==256 and int(m['mem_limit'])==1073741824 and int(m['shm_size'])==134217728 and 'noexec' in m['tmpfs'][0] and 'nosuid' in m['tmpfs'][0]
parts={'wheel-rear','wheel-front','crank-center','pedal-left','pedal-right','foot-left','foot-right','leg-left','leg-right'}
assert set(re.findall(r'data-intel-part="([^"]+)"',h.split('<script>',1)[0]))==parts and 'requestAnimationFrame(animate)' in h and not re.search(r'(src|href)=["\']https?://',h)
PY
printf 'docker compose build motion test passed\n'
