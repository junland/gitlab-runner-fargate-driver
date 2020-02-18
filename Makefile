export VERSION ?= $(shell (scripts/version.sh 2>/dev/null || echo "dev") | sed -e 's/^v//g')
export REVISION ?= $(shell git rev-parse --short HEAD || echo "unknown")
export BRANCH ?= $(shell git show-ref | grep "$(REVISION)" | grep -v HEAD | awk '{print $$2}' | sed 's|refs/remotes/origin/||' | sed 's|refs/heads/||' | sort | head -n 1)
export BUILT ?= $(shell date -u +%Y-%m-%dT%H:%M:%S%z)
export CGO_ENABLED ?= 0
export GOPATH ?= ".go/"
export BUILD_PLATFORMS ?= -osarch 'linux/amd64 linux/386'

export goJunitReport ?= $(GOPATH)/bin/go-junit-report
mockery ?= $(GOPATH)/bin/mockery
gox ?= $(GOPATH)/bin/gox

PKG := $(shell go list .)
PKGs := $(shell go list ./... | grep -vE "^/vendor/")

GO_LDFLAGS := -X $(PKG).VERSION=$(VERSION) \
              -X $(PKG).REVISION=$(REVISION) \
              -X $(PKG).BRANCH=$(BRANCH) \
              -X $(PKG).BUILT=$(BUILT) \
              -s -w

.PHONY: compile
compile:
	go build \
			-o build/fargate \
			-ldflags "$(GO_LDFLAGS)" \
			./cmd/fargate

.PHONY: compile_all
compile_all: $(gox)
	# Building project in version $(VERSION) for $(BUILD_PLATFORMS)
	$(gox) $(BUILD_PLATFORMS) \
			-ldflags "$(GO_LDFLAGS)" \
			-output="build/fargate-{{.OS}}-{{.Arch}}" \
			./cmd/fargate

export testsDir = ./.tests

.PHONY: tests
tests: $(testsDir) $(goJunitReport)
	@./scripts/tests.sh normal

.PHONY: tests_race
tests_race: $(testsDir) $(goJunitReport)
	@./scripts/tests.sh race

.PHONY: codequality
# Using codeclimate linters forks, because the official releases are built on top of old versions of Go,
# which generates errors, e.g. when sing new error handling implemented in stdlib `errors` package in Go 1.13.
# We've prepared updates and pushed patches to the upstream, but until they will be merged and released
# the only way is to use the forks.
codequality: CODECLIMATE_VERSION ?= 0.85.5
codequality: REPORT_FILE ?= gl-code-quality-report.json
codequality:
	# Pulling gocyclo analyzer
	@docker pull registry.gitlab.com/tmaczukin/codeclimate-gocyclo:tm-go1.12.5-alpine3.9-1 > /dev/null
	@docker tag registry.gitlab.com/tmaczukin/codeclimate-gocyclo:tm-go1.12.5-alpine3.9-1 codeclimate/codeclimate-gocyclo > /dev/null
	# Pulling gofmt analyzer
	@docker pull registry.gitlab.com/tmaczukin/codeclimate-gofmt:tm-go1.12.5-alpine3.9-2 > /dev/null
	@docker tag registry.gitlab.com/tmaczukin/codeclimate-gofmt:tm-go1.12.5-alpine3.9-2 codeclimate/codeclimate-gofmt > /dev/null
	# Pulling govet analyzer
	@docker pull registry.gitlab.com/tmaczukin/codeclimate-govet:tm-go1.12.5-alpine3.9-2 > /dev/null
	@docker tag registry.gitlab.com/tmaczukin/codeclimate-govet:tm-go1.12.5-alpine3.9-2 codeclimate/codeclimate-govet > /dev/null
	# Pulling golint analyzer
	@docker pull registry.gitlab.com/tmaczukin/codeclimate-golint:tm-go1.12.5-alpine3.9-2 > /dev/null
	@docker tag registry.gitlab.com/tmaczukin/codeclimate-golint:tm-go1.12.5-alpine3.9-2 codeclimate/codeclimate-golint > /dev/null
	# Starting codeclimate
	@docker run --rm --env CODECLIMATE_CODE="$$(pwd)" \
		--volume "$$(pwd)":/code \
		--volume /var/run/docker.sock:/var/run/docker.sock \
		--volume /tmp/cc:/tmp/cc \
		codeclimate/codeclimate:$(CODECLIMATE_VERSION) \
		analyze -f json --dev | tee $(REPORT_FILE)
	# Checking if offenses are reported
	@if [ "$$(cat $(REPORT_FILE))" != "[]" ] ; then \
	  jq -C . $(REPORT_FILE); \
	  exit 1 ;\
	fi

$(testsDir):
	# Preparing tests output directory
	@mkdir -p $@

.PHONY: fmt
fmt:
	# Fixing project code formatting...
	@go fmt $(PKGs) | awk '{if (NF > 0) {if (NR == 1) print "Please run go fmt for:"; print "- "$$1}} END {if (NF > 0) {if (NR > 0) exit 1}}'

.PHONY: mocks
mocks: $(mockery)
	# Removing existing mocks
	@find * -name "mock_*.go" -delete
	# Generating new mocks
	@$(mockery) -recursive -all -inpkg -dir ./

.PHONY: check_mocks
check_mocks:
	# Checking if mocks are up-to-date
	# Saving the list of committed mocks
	@git status -sb > /tmp/mocks-$${CI_JOB_ID}-before
	@$(MAKE) mocks
	# Saving the list of created mocks
	@git status -sb > /tmp/mocks-$${CI_JOB_ID}-after
	# Checking the difference
	@diff -U0 /tmp/mocks-$${CI_JOB_ID}-before /tmp/mocks-$${CI_JOB_ID}-after

.PHONY: prepare_ci_image
prepare_ci_image: CI_IMAGE ?= fargate-ci-image
prepare_ci_image: CI_REGISTRY ?= ""
prepare_ci_image:
	# Builiding the $(CI_IMAGE) image
	@docker build --pull --no-cache -t $(CI_IMAGE) -f dockerfiles/ci/Dockerfile dockerfiles/ci/
ifneq ($(CI_REGISTRY),)
	# Pushing the $(CI_IMAGE) image to $(CI_REGISTRY)
	@docker login --username $${CI_REGISTRY_USER} --password $${CI_REGISTRY_PASSWORD} $(CI_REGISTRY)
	@docker push $(CI_IMAGE)
	@docker logout $(CI_REGISTRY)
else
	# No CI_REGISTRY value, skipping image push
endif

.PHONY: release_s3
release_s3: CI_COMMIT_REF_NAME ?= $(BRANCH)
release_s3: CI_COMMIT_SHA ?= $(REVISION)
release_s3: S3_BUCKET ?=
release_s3: latest_stable_tag := $(shell git -c versionsort.prereleaseSuffix="-rc" tag -l "v*.*.*" --sort=-v:refname | awk '!/rc/' | head -n 1)
release_s3:
	# Preparing index page
	@$(MAKE) index_file REF=$(CI_COMMIT_REF_NAME) REV=$(CI_COMMIT_SHA)
ifneq ($(S3_BUCKET),)
	# Syncing with "s3://$(S3_BUCKET)/$(CI_COMMIT_REF_NAME)"
	@$(MAKE) sync_s3_release S3_URL="s3://$(S3_BUCKET)/$(CI_COMMIT_REF_NAME)"
	@if [ $$(git describe --exact-match --match $(latest_stable_tag) >/dev/null 2>&1) ]; then \
		@$(MAKE) sync_s3_release S3_URL="s3://$(S3_BUCKET)/latest"; \
    fi
	@$(MAKE) release_gitlab
endif

.PHONY: sync_s3_release
sync_s3_release: S3_URL ?=
sync_s3_release:
	# Syncing with $(S3_URL)
	@aws s3 sync build "$(S3_URL)" --acl public-read

.PHONY: remove_s3_release
remove_s3_release: CI_COMMIT_REF_NAME ?= $(BRANCH)
remove_s3_release: S3_BUCKET ?=
remove_s3_release:
ifneq ($(S3_BUCKET),)
	@aws s3 rm "s3://$(S3_BUCKET)/$(CI_COMMIT_REF_NAME)" --recursive
endif

.PHONY: release_gitlab
release_gitlab: export CI_COMMIT_TAG ?=
release_gitlab: export CI_PROJECT_URL ?=
release_gitlab:
ifneq ($(CI_COMMIT_TAG),)
	# Saving as GitLab release at $(CI_PROJECT_URL)/-/releases
	@$(shell ./scripts/gitlab_release)
endif

.PHONY: index_file
index_file: REF ?= $(BRANCH)
index_file: REV ?= $(shell git rev-parse HEAD)
index_file:
	# generating index.html file
	@go run ./scripts/generate-index-file/main.go build/ $(VERSION) $(REF) $(REV)

.PHONY: check_modules
check_modules:
	@git diff go.sum > /tmp/gosum-$${CI_JOB_ID}-before
	@go mod tidy
	@git diff go.sum > /tmp/gosum-$${CI_JOB_ID}-after
	@diff -U0 /tmp/gosum-$${CI_JOB_ID}-before /tmp/gosum-$${CI_JOB_ID}-after

$(mockery):
	# Installing mockery
	@go install github.com/vektra/mockery/cmd/mockery

$(goJunitReport):
	# Installing go-junit-report
	@go install github.com/jstemmer/go-junit-report

$(gox):
	# Installing gox
	@go install github.com/mitchellh/gox
