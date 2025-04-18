PROFILE=$(shell aws configure list-profiles | grep default || echo "default")
ACCOUNT=$(shell aws sts get-caller-identity --profile $(PROFILE) | jq -r '.Account')
GITHUB_TOKEN=$(shell aws ssm get-parameter --profile $(PROFILE) --name /github_token --with-decryption | jq -r '.Parameter.Value')

snapshot:
	ACCOUNT=$(ACCOUNT) PROFILE=$(PROFILE) goreleaser build --snapshot --clean

release:
	aws ecr get-login-password  --profile $(PROFILE) --region ca-central-1 | docker login --username AWS --password-stdin $(ACCOUNT).dkr.ecr.ca-central-1.amazonaws.com
	GITHUB_TOKEN=$(GITHUB_TOKEN) ACCOUNT=$(ACCOUNT) PROFILE=$(PROFILE) goreleaser release --clean

pre-release:
	aws ecr get-login-password  --profile $(PROFILE) --region ca-central-1 | docker login --username AWS --password-stdin $(ACCOUNT).dkr.ecr.ca-central-1.amazonaws.com
	GITHUB_TOKEN=$(GITHUB_TOKEN) ACCOUNT=$(ACCOUNT) PROFILE=$(PROFILE) goreleaser release --clean --skip publish --auto-snapshot

	
.PHONY: tidy-modules
tidy-modules:
	@find . -type d \( -name build -prune \) -o -name go.mod -print | while read -r gomod_path; do \
		dir_path=$$(dirname "$$gomod_path"); \
		echo "Executing 'go mod tidy' in directory: $$dir_path"; \
		(cd "$$dir_path"  && GOPROXY=$(GOPROXY) go mod tidy) || exit 1; \
	done

upgrade-modules:
	@find . -type d \( -name build -prune \) -o -name go.mod -print | while read -r gomod_path; do \
		dir_path=$$(dirname "$$gomod_path"); \
		echo "Executing 'go get -u' in directory: $$dir_path"; \
		(cd "$$dir_path"  && GOPROXY=$(GOPROXY) go get -u ./... && GOPROXY=$(GOPROXY) go mod tidy) || exit 1; \
	done
