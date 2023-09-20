PROFILE=$(shell aws configure list-profiles | grep personal || echo "default")
ACCOUNT=$(shell aws sts get-caller-identity --profile $(PROFILE) | jq -r '.Account')
GITHUB_TOKEN=$(shell aws ssm get-parameter --profile $(PROFILE) --name /github_token --with-decryption | jq -r '.Parameter.Value')

snapshot:
	ACCOUNT=$(ACCOUNT) PROFILE=$(PROFILE) goreleaser build --snapshot --clean

release:
	aws ecr get-login-password  --profile $(PROFILE) --region ca-central-1 | docker login --username AWS --password-stdin $(ACCOUNT).dkr.ecr.ca-central-1.amazonaws.com
	GITHUB_TOKEN=$(GITHUB_TOKEN) ACCOUNT=$(ACCOUNT) PROFILE=$(PROFILE) goreleaser release --clean

pre-release:
	aws ecr get-login-password  --profile $(PROFILE) --region ca-central-1 | docker login --username AWS --password-stdin $(ACCOUNT).dkr.ecr.ca-central-1.amazonaws.com
	GITHUB_TOKEN=$(GITHUB_TOKEN) ACCOUNT=$(ACCOUNT) PROFILE=$(PROFILE) goreleaser release --clean --skip-publish --auto-snapshot