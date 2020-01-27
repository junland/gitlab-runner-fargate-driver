## Fargate Custom Executor driver

This project contains a driver for [GitLab Runner's Custom
Executor](https://docs.gitlab.com/runner/executors/custom.html) that will
use [AWS Fargate](https://aws.amazon.com/fargate/) to execute the jobs.

**The project is very young and still under a heavy development.**

## Release channels

The driver is released in the form of a binary and is available from our S3 bucket.

For the latest beta version - based on the current state of `master` branch,
please visit the <https://gitlab-runner-custom-fargate-downloads.s3.amazonaws.com/master/index.html>
page.

For specific, tagged versions, please replace the `master` in the URL with a specific tag, for example
to download version `v0.1.0` please open `https://gitlab-runner-custom-fargate-downloads.s3.amazonaws.com/v0.1.0/index.html`.

All stable and RC releases, together with links to the specific S3 bucket, will be also listed
at the [Releases](https://gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/-/releases)
page.

## License

MIT

