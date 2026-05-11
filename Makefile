.PHONY: help deploy configmap

NAMESPACE := observability

help:
	@echo "Targets:"
	@echo "  make deploy     - Apply all manifests and regenerate ConfigMap from collector-config.yaml"
	@echo "  make configmap  - Regenerate ConfigMap only (without redeploying)"

configmap:
	kubectl create configmap custom-collector-config \
		--from-file=collector-config.yaml=collector/collector-config.yaml \
		--namespace=$(NAMESPACE) \
		--dry-run=client -o yaml | kubectl apply -f -

deploy: configmap
	kubectl apply -f deploy/namespace.yaml
	kubectl apply -f deploy/deployment.yaml
	kubectl apply -f deploy/service.yaml
	kubectl rollout status deployment/custom-collector -n $(NAMESPACE)
