PROFILE=$(shell aws configure list-profiles | grep personal || echo "default")
ACCOUNT=$(aws sts get-caller-identity --profile "$PROFILE" | jq -r '.Account')

snapshot:
	ACCOUNT=$(ACCOUNT) PROFILE=$(PROFILE) goreleaser build --snapshot --rm-dist

release:
	ACCOUNT=$(ACCOUNT) PROFILE=$(PROFILE) goreleaser release --rm-dist