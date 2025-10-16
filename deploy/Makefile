ifneq (,$(wildcard ./.env))
	include .env
	export
else
  $(error You haven't setup your .env file. Please refer to the readme)
endif
VERSION=$(shell awk -F'"' '/"version":/ {print $$4}' ../version.json)
LAMBDA_GOOS=linux
LAMBDA_GOARCH=arm64
LAMBDA_GOCC?=go
LAMBDA_GOFLAGS=-tags=lambda.norpc -ldflags="-s -w -X github.com/storacha/piri/pkg/build.version=$(VERSION)"
LAMBDA_CGO_ENABLED=0
LAMBDAS=build/aggregatesubmitter/bootstrap build/getclaim/bootstrap build/getroot/bootstrap build/pieceaccepter/bootstrap build/pieceaggregator/bootstrap build/postroot/bootstrap build/putblob/bootstrap

.PHONY: clean-lambda

clean-lambda:
	rm -rf build

.PHONY: clean-terraform

clean-terraform:
	tofu -chdir=app destroy

.PHONY: clean

clean: clean-terraform clean-lambda

lambdas: $(LAMBDAS)

.PHONY: $(LAMBDAS)

$(LAMBDAS): build/%/bootstrap:
	GOOS=$(LAMBDA_GOOS) GOARCH=$(LAMBDA_GOARCH) CGO_ENABLED=$(LAMBDA_CGO_ENABLED) $(LAMBDA_GOCC) build $(LAMBDA_GOFLAGS) -o $@ ../cmd/lambda/$*

app/.terraform:
	TF_WORKSPACE= tofu -chdir=app init

.tfworkspace: app/.terraform
	TF_WORKSPACE= tofu -chdir=app workspace new $(TF_WORKSPACE)
	touch .tfworkspace

.PHONY: init

init: app/.terraform .tfworkspace

.PHONY: validate

validate: app/.terraform .tfworkspace
	tofu -chdir=app validate

.PHONY: plan

plan: app/.terraform .tfworkspace $(LAMBDAS)
	tofu -chdir=app plan

.PHONY: apply

apply: app/.terraform .tfworkspace $(LAMBDAS)
	tofu -chdir=app apply

shared:
