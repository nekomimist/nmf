#!/usr/bin/env bash
# Real SMB tests against a disposable, unprivileged Samba container.
set -euo pipefail

fixture_dir=$(mktemp -d)
fixture_name="nmf-smb-test-$$-$RANDOM"
cleanup() {
    docker unpause "$fixture_name" >/dev/null 2>&1 || true
    docker rm -f "$fixture_name" >/dev/null 2>&1 || true
    docker image rm "$fixture_name" >/dev/null 2>&1 || true
    rm -rf -- "$fixture_dir"
}
trap cleanup EXIT

cat > "$fixture_dir/Dockerfile" <<'EOF'
FROM ubuntu:22.04
RUN apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends samba && rm -rf /var/lib/apt/lists/*
RUN mkdir -p /srv/nmf-test && chmod 0777 /srv/nmf-test
COPY smb.conf /etc/samba/smb.conf
CMD ["smbd", "--foreground", "--no-process-group"]
EOF
cat > "$fixture_dir/smb.conf" <<'EOF'
[global]
server role = standalone server
map to guest = Bad User
smb ports = 445
server min protocol = SMB2
[nmf-test]
path = /srv/nmf-test
read only = no
guest ok = yes
force user = root
EOF

docker build -q -t "$fixture_name" "$fixture_dir"
docker run -d --name "$fixture_name" "$fixture_name"
fixture_ip=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$fixture_name")
if [[ -z "$fixture_ip" ]]; then
    echo "The Docker bridge must be reachable from the Linux test host" >&2
    exit 1
fi
ready=false
for ((attempt = 0; attempt < 50; attempt++)); do
    if (echo > "/dev/tcp/$fixture_ip/445") 2>/dev/null; then ready=true; break; fi
    sleep 0.1
done
if [[ "$ready" != true ]]; then docker logs "$fixture_name"; exit 1; fi
docker exec "$fixture_name" smbd --version
export NMF_SMB_TEST_DIR="smb://guest@$fixture_ip/nmf-test"

# Run these sequentially: the read-cancellation test pauses the server.
go test -race -tags migrated_fynedo -count=1 -timeout=30s -v ./internal/jobs -run '^(TestSMBCopyRoundtrip|TestSMBReplacementIntegration)$'
NMF_SMB_TEST_CONTAINER="$fixture_name" go test -race -tags migrated_fynedo -count=1 -timeout=30s -v ./internal/fileinfo -run '^TestSMBReadCancellationWithPausedServer$'
