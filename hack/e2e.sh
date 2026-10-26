#!/usr/bin/env bash
# The end-to-end tier (docs/adr/0003): a kind cluster of its own with cert-manager, haproxy-ingress
# and Dash, driven by Playwright through both Ingress objects over TLS. make e2e, e2e-up, e2e-test
# and e2e-down call it with the pins of the Makefile in the environment.
#
#   hack/e2e.sh up | suite [playwright args] | down | run [playwright args]
#
# Kubeconfig safety: every call names KIND_KUBECONFIG, a file of its own; nothing reads or writes
# ~/.kube/config. What needs kubectl runs it inside the kind node, against the node's own admin
# configuration, so no kubectl of this machine is used either.
set -euo pipefail

: "${KIND:?}" "${HELM:?}" "${KIND_CLUSTER:?}" "${KIND_KUBECONFIG:?}" "${KIND_NODE_IMAGE:?}" "${E2E_PORT:?}"
: "${BACKEND_IMG:?}" "${FRONTEND_IMG:?}"
: "${HAPROXY_INGRESS_CHART_VERSION:?}" "${HAPROXY_INGRESS_CHART_REPO:?}"
: "${CERT_MANAGER_CHART_VERSION:?}" "${CERT_MANAGER_CHART_REPO:?}"

root=$(cd "$(dirname "$0")/.." && pwd)
node="${KIND_CLUSTER}-control-plane"
# The Makefile's HELM is the pinned binary preceded by its HELM_*_HOME assignments; env runs it so.
read -ra helm_cmd <<<"$HELM"
helm_k=(env "${helm_cmd[@]}" --kubeconfig "$KIND_KUBECONFIG")

log() { printf '==> %s\n' "$*"; }

# kubectl inside the kind node: its own binary, its own admin configuration.
node_kubectl() { docker exec -i "$node" kubectl --kubeconfig /etc/kubernetes/admin.conf "$@"; }

cluster_exists() { "$KIND" get clusters 2>/dev/null | grep -qx "$KIND_CLUSTER"; }

up() {
	mkdir -p "$(dirname "$KIND_KUBECONFIG")"
	if cluster_exists; then
		log "kind cluster $KIND_CLUSTER exists"
		"$KIND" export kubeconfig --name "$KIND_CLUSTER" --kubeconfig "$KIND_KUBECONFIG"
	else
		log "creating kind cluster $KIND_CLUSTER (host port $E2E_PORT)"
		sed "s/E2E_PORT/$E2E_PORT/" "$root/hack/kind/cluster.yaml" |
			"$KIND" create cluster --name "$KIND_CLUSTER" --image "$KIND_NODE_IMAGE" \
				--kubeconfig "$KIND_KUBECONFIG" --wait 120s --config -
	fi

	log "loading $BACKEND_IMG and $FRONTEND_IMG into the cluster"
	"$KIND" load docker-image --name "$KIND_CLUSTER" "$BACKEND_IMG" "$FRONTEND_IMG"

	log "installing cert-manager $CERT_MANAGER_CHART_VERSION"
	"${helm_k[@]}" upgrade --install cert-manager cert-manager --repo "$CERT_MANAGER_CHART_REPO" \
		--version "$CERT_MANAGER_CHART_VERSION" --namespace cert-manager --create-namespace \
		--set crds.enabled=true --wait --timeout 5m
	node_kubectl apply -f - <"$root/hack/kind/selfsigned-issuer.yaml"

	log "installing haproxy-ingress $HAPROXY_INGRESS_CHART_VERSION"
	"${helm_k[@]}" upgrade --install haproxy-ingress haproxy-ingress --repo "$HAPROXY_INGRESS_CHART_REPO" \
		--version "$HAPROXY_INGRESS_CHART_VERSION" --namespace ingress-controller --create-namespace \
		-f "$root/hack/kind/haproxy-ingress-values.yaml" --wait --timeout 5m

	log "installing Dash"
	"${helm_k[@]}" upgrade --install dash "$root/deploy/helm/dash" --namespace dash --create-namespace \
		-f "$root/hack/kind/dash-values.yaml" \
		--set "backend.image.repository=${BACKEND_IMG%:*}" --set "backend.image.tag=${BACKEND_IMG##*:}" \
		--set "frontend.image.repository=${FRONTEND_IMG%:*}" --set "frontend.image.tag=${FRONTEND_IMG##*:}" \
		--wait --timeout 5m

	log "waiting for cert-manager to issue dash-tls"
	node_kubectl -n dash wait certificate/dash-tls --for=condition=Ready --timeout=120s

	log "waiting for both Ingress objects to answer on https://localhost:$E2E_PORT"
	local i answered=""
	for i in $(seq 1 60); do
		if curl -ksf -o /dev/null "https://localhost:$E2E_PORT/healthz" &&
			curl -ksf -o /dev/null "https://localhost:$E2E_PORT/api/v1/version"; then
			log "Dash answers through both Ingress objects"
			answered=yes
			break
		fi
		sleep 2
	done
	if [ -z "$answered" ]; then
		echo "Dash did not answer through the Ingress objects within two minutes" >&2
		diagnose
		return 1
	fi

	# The apps come after Dash, so that the seed's remote icon host, Dash's own frontend, answers
	# when the discovery reads the Ingress that names it.
	log "seeding the apps of hack/kind/seed.yaml"
	sed "s#FRONTEND_IMAGE#$FRONTEND_IMG#" "$root/hack/kind/seed.yaml" | node_kubectl apply -f -
	node_kubectl -n e2e-apps rollout status deploy/web deploy/jellyfin deploy/alerts-ok --timeout=120s
}

# The SHA-256 fingerprint of the certificate in dash-tls, which the suite compares with the one
# the controller serves.
secret_fingerprint() {
	node_kubectl -n dash get secret dash-tls -o 'jsonpath={.data.tls\.crt}' |
		base64 -d | openssl x509 -noout -fingerprint -sha256 | sed 's/^.*=//'
}

suite() {
	cluster_exists || { echo "no cluster $KIND_CLUSTER: run make e2e-up first" >&2; return 1; }
	local fingerprint
	fingerprint=$(secret_fingerprint)
	log "running the suite against https://localhost:$E2E_PORT (dash-tls $fingerprint)"
	(cd "$root/frontend" && E2E_BASE_URL="https://localhost:$E2E_PORT" E2E_TLS_FINGERPRINT="$fingerprint" E2E_NODE="$node" \
		npx playwright test -c e2e/playwright.config.ts "$@")
}

diagnose() {
	cluster_exists || return 0
	node_kubectl get pods -A -o wide || true
	node_kubectl -n dash get ingress,certificate,secret || true
	node_kubectl -n dash logs deploy/dash-backend --tail=50 || true
}

down() {
	if cluster_exists; then
		log "deleting kind cluster $KIND_CLUSTER"
		"$KIND" delete cluster --name "$KIND_CLUSTER" --kubeconfig "$KIND_KUBECONFIG"
	fi
	rm -f "$KIND_KUBECONFIG"
}

run() {
	trap down EXIT
	up
	if ! suite "$@"; then
		diagnose
		return 1
	fi
}

cmd=${1:-}
shift || true
case "$cmd" in
up) up ;;
suite) suite "$@" ;;
down) down ;;
run) run "$@" ;;
*)
	echo "usage: $0 up | suite [playwright args] | down | run [playwright args]" >&2
	exit 2
	;;
esac
