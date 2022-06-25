PROFILE=$(shell aws configure list-profiles | grep personal || echo "default")
ACCOUNT=$(shell aws sts get-caller-identity --profile $(PROFILE) | jq -r '.Account')
GITHUB_TOKEN=$(shell aws ssm get-parameter --profile $(PROFILE) --name /github_token --with-decryption | jq -r '.Parameter.Value')

snapshot:
	ACCOUNT=$(ACCOUNT) PROFILE=$(PROFILE) goreleaser build --snapshot --rm-dist

release:
	GITHUB_TOKEN=$(GITHUB_TOKEN) ACCOUNT=$(ACCOUNT) PROFILE=$(PROFILE) goreleaser release --rm-dist