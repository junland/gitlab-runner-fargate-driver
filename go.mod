module gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate

go 1.13

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/jstemmer/go-junit-report v0.9.1
	github.com/mitchellh/gox v1.0.1
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/afero v1.2.2
	github.com/stretchr/testify v1.4.0
	github.com/urfave/cli v1.22.2
	github.com/vektra/mockery v0.0.0-20181123154057-e78b021dcbb5
	gitlab.com/ayufan/golang-cli-helpers v0.0.0-20171103152739-a7cf72d604cd
	gitlab.com/gitlab-org/ci-cd/runner-tools/release-index-generator v0.0.0-20200128145756-4b03f39cb151
	gitlab.com/gitlab-org/gitlab-runner v12.5.0+incompatible
)
