#!/usr/bin/env bash
# Concurrencia real entre procesos sobre el mismo deck: dos prepares con la
# misma huella y un launch compiten a la vez; SQLite serializa, exactamente uno
# de los prepares gana, el duplicado se deniega y no se pierde ninguna fila.
# Después, tres prepares distintos en paralelo deben commitear los tres.
# Compila con -race: cualquier carrera interna del binario aparece en stderr.
set -euo pipefail

here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
repo=$(cd "$here/../.." && pwd)
work=${WORK:-$(mktemp -d "${TMPDIR:-/tmp}/wp04-concurrency.XXXXXX")}
mkdir -p "$work"
bin=${GO_BIN:-$work/orchestrator-runner-race}
if [ -z "${GO_BIN:-}" ]; then
  (cd "$repo/runner-go" && go build -race -o "$bin" ./cmd/orchestrator-runner)
fi
session=ses_concurrency
rounds=${ROUNDS:-3}

fail() { echo "FAIL: $*" >&2; exit 1; }
aif() { # dir, json -> stdout; caller redirects
  local dir=$1
  ( cd "$dir" && OPENCODE_SESSION_ID="$session" "$bin" aif ) <<<"$2"
}
rc() { cat "$1"; }
race_free() { ! grep -q "DATA RACE" "$1" 2>/dev/null; }

same_a='{"aif":"0.2","operation":"deck_prepare","input":{"agent":"general","task":"tk-race","goal":"mismo encargo en carrera","scope":["a"],"checks":["go test ./..."],"criteria":"serializa"}}'
seed_b='{"aif":"0.2","operation":"deck_prepare","input":{"agent":"verify","task":"tk-seed","goal":"encargo sembrado","scope":["b"],"checks":["go vet ./..."],"criteria":"sembrado"}}'

for round in $(seq 1 "$rounds"); do
  dir="$work/round-$round"
  mkdir -p "$dir"

  aif "$dir" "$seed_b" >"$dir/seed.out" 2>"$dir/seed.err" || fail "ronda $round: no se pudo sembrar el encargo B"
  b_id=$(python3 - "$dir/seed.out" <<'PY'
import json, sys
print(json.load(open(sys.argv[1]))["outcome"]["id"])
PY
)
  launch_b="{\"aif\":\"0.2\",\"operation\":\"deck_launch\",\"input\":{\"id\":\"$b_id\"}}"

  set +e
  ( cd "$dir" && OPENCODE_SESSION_ID="$session" "$bin" aif <<<"$same_a" >"$dir/prep1.out" 2>"$dir/prep1.err"; echo $? >"$dir/prep1.rc" ) &
  p1=$!
  ( cd "$dir" && OPENCODE_SESSION_ID="$session" "$bin" aif <<<"$same_a" >"$dir/prep2.out" 2>"$dir/prep2.err"; echo $? >"$dir/prep2.rc" ) &
  p2=$!
  ( cd "$dir" && OPENCODE_SESSION_ID="$session" "$bin" aif <<<"$launch_b" >"$dir/launch.out" 2>"$dir/launch.err"; echo $? >"$dir/launch.rc" ) &
  p3=$!
  wait "$p1" "$p2" "$p3"
  set -e

  r1=$(rc "$dir/prep1.rc"); r2=$(rc "$dir/prep2.rc"); r3=$(rc "$dir/launch.rc")
  wins=0
  { [ "$r1" = 0 ] && wins=$((wins + 1)); } || true
  { [ "$r2" = 0 ] && wins=$((wins + 1)); } || true
  [ "$wins" = 1 ] || fail "ronda $round: prepares con misma huella exitosos=$wins (rc $r1/$r2); no serializó"
  [ "$r3" = 0 ] || fail "ronda $round: el launch paralelo falló (rc $r3)"
  ( [ "$r1" = 4 ] || [ "$r2" = 4 ] ) || fail "ronda $round: el duplicado debía denegarse con 4 (rc $r1/$r2)"
  race_free "$dir/prep1.err" && race_free "$dir/prep2.err" && race_free "$dir/launch.err" \
    || fail "ronda $round: el detector de carreras encontró DATA RACE"

  aif "$dir" '{"aif":"0.2","operation":"deck_list","input":{"state":"all"}}' >"$dir/list.out"
  python3 - "$dir/list.out" <<'PY'
import json, sys
d = json.load(open(sys.argv[1]))["outcome"]["delegations"]
assert len(d) == 2, "filas tras la carrera: %d (se perdió una escritura)" % len(d)
states = sorted(row["state"] for row in d)
assert states == ["launched", "prepared"], states
PY

  # Tres encargos distintos a la vez: ninguno se pierde.
  set +e
  for i in 1 2 3; do
    inp="{\"aif\":\"0.2\",\"operation\":\"deck_prepare\",\"input\":{\"agent\":\"general\",\"task\":\"tk-dist-$i\",\"goal\":\"encargo distinto $i\",\"scope\":[\"d$i\"],\"checks\":[\"go build\"],\"criteria\":\"distinto\"}}"
    ( cd "$dir" && OPENCODE_SESSION_ID="$session" "$bin" aif <<<"$inp" >"$dir/dist$i.out" 2>"$dir/dist$i.err"; echo $? >"$dir/dist$i.rc" ) &
    eval "dp$i=$!"
  done
  wait "$dp1" "$dp2" "$dp3"
  set -e
  for i in 1 2 3; do
    [ "$(rc "$dir/dist$i.rc")" = 0 ] || fail "ronda $round: prepare distinto $i falló"
    race_free "$dir/dist$i.err" || fail "ronda $round: DATA RACE en prepare distinto"
  done
  aif "$dir" '{"aif":"0.2","operation":"deck_list","input":{"state":"all"}}' >"$dir/list2.out"
  python3 - "$dir/list2.out" <<'PY'
import json, sys
d = json.load(open(sys.argv[1]))["outcome"]["delegations"]
assert len(d) == 5, "filas tras los prepares distintos: %d (se perdió una)" % len(d)
PY

  echo "ok ronda $round: prepares serializados, duplicado denegado, launch y 3 prepares distintos sin pérdida"
done

echo
echo "PASS: concurrencia real sin pérdida en $rounds ronda(s); work=$work"
