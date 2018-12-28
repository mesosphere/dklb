# Developing `dklb`

## Prerequisites

To build `dklb`, the following software is required:

* [`git`].
* [`make`].
* [Go] 1.11.4+.
  * `dklb` makes use of the [Go modules] experiment present in Go 1.11+ only.
  * Go 1.11.3 and earlier were found to have issues computing the checksum of certain modules.
  
To run `dklb`, the following additional software is required:
  
* [`skaffold`]
* A [DC/OS] cluster having [EdgeLB] installed.
  * DC/OS must be v1.12.0 or later.
  * EdgeLB must be [`v1.2.3-22-ga35988a`] or later.
  * The DC/OS CLI must be configured to access the DC/OS cluster.
* An [MKE] cluster.
  * `kubernetes-cluster` must be [`d74b25e`] or later.
  * The current kubeconfig context must be configured to point at this cluster.
  
A Docker Hub account with read/write access to the [`mesosphere/dklb`] image repository is additionally required.

## Cloning the repository

To clone the repository, the following command may be run:

```console
$ git clone git@github.com:mesosphere/dklb.git /some/path
```

**NOTE:** As `dklb` relies on Go modules, it is **NOT** necessary to clone to a directory inside `$GOPATH`.

## Installing dependencies

`dklb` depends on private GitHub repositories (notably `mesosphere/dklb`).
To allow for `go mod` to access these repositories, the following command must be run:

```
$ git config --global url."git@github.com:".insteadOf "https://github.com/"
```

To install all the dependencies required to build `dklb`, the following command must then be run:

```console
$ make mod
```

## Building `dklb`

To build the `dklb` binary, the following command may be run:

```console
$ make build
```

By default, this will create a `build/dklb` binary targeting `linux-amd64`.
This binary is suitable to be imported to inside a container image and ran inside a Kubernetes cluster.

**NOTE:** Even though it is not recommended or supported, it is possible to build a binary targeting a different platform by running a command similar to the following one:

```console
$ make build GOOS=darwin LDFLAGS=
```

This can be useful to perform local testing with the generated binary.

## Running `dklb`

[`skaffold`] is used to ease the process of running and testing `dklb` during day-to-day development.
`skaffold` builds a Docker image containing the `dklb` binary and pushes it to the [`mesosphere/dklb`] image repository.
This repository is currently private, and accessible only by members of the `kubernetes` team in the Mesosphere organization.
Hence, in order to push the image, it is necessary to login to Docker Hub with adequate credentials:

```console
$ docker login
```

Additionally, and so that Kubernetes can pull the image built and pushed by `skaffold`, it is necessary to create a Kubernetes secret containing credentials with read access to the Docker Hub repository.
This secret **MUST** be created in the `kube-system` namespace.
To create said secret, the following command may be run:

```console
$ kubectl -n kube-system create secret docker-registry \
    docker-hub \
    --docker-username "<username>" \
    --docker-password "<password>"
```

To deploy `dklb` to the MKE cluster targeted by the current kubeconfig context, the following command may then be run:

```console
$ make skaffold
```

These command will perform the following tasks:

1. Build the `build/dklb` binary.
1. Build the `mesosphere/dklb` Docker image based on said binary.
1. Push the `mesosphere/dklb` Docker image to Docker Hub.
1. Create or update a `dklb` service account, cluster role and cluster role binding.
1. Deploy `dklb` as a single pod that uses the `kube-system/mke-cluster-info` configmap to configure its environment.
1. Stream logs from the `dklb` pod until `Ctrl+C` is hit.

To simply deploy the `dklb` pod without streaming logs, the following command may be run instead:

```console
$ make skaffold MODE=run
```

To delete any resources that may have been created by `skaffold` (and hence uninstall `dklb`), the following command may be run:

```console
$ make skaffold MODE=delete
```

## Testing `dklb`

### Running the unit test suite

In order to run the unit test suite for `dklb`, the following command may be run:

```console
$ make test.unit
```

### Running the end-to-end test suite

As of this writing, `dklb`'s end-to-end test suite has the following additional requirements:

* The target DC/OS cluster must have a _single_ DC/OS agent with the `slave_public` role;
  * It is assumed that the external IP for said DC/OS agent is `<dcos-public-agent-ip>`.
* The end-to-end test suite must run from _outside_ the target DC/OS cluster.

To run the end-to-end test suite against the MKE cluster targeted by `$HOME/.kube/config`, the following command may be run:

```console
$ make test.e2e DCOS_PUBLIC_AGENT_IP="<dcos-public-agent-ip>"
```

The output of a successful run of the end-to-end test suite will be similar to the following:

```text
(...)
Ran 2 of 2 Specs in 199.297 seconds
SUCCESS! -- 2 Passed | 0 Failed | 0 Pending | 0 Skipped
--- PASS: TestEndToEnd (199.30s)
PASS
ok  	github.com/mesosphere/dklb/test/e2e	199.338s
```


[`git`]: https://git-scm.com/
[Go]: https://golang.org/
[Go modules]: https://github.com/golang/go/wiki/Modules
[`make`]: https://www.gnu.org/software/make/
[`skaffold`]: https://github.com/GoogleContainerTools/skaffold
[DC/OS]: https://dcos.io/
[EdgeLB]: https://docs.mesosphere.com/services/edge-lb/
[`v1.2.3-22-ga35988a`]: https://github.com/mesosphere/dcos-edge-lb/commit/a35988a489ab4d515cd4d023ec0742466a3c272b
[MKE]: https://mesosphere.com/product/kubernetes-engine/
[`d74b25e`]: https://github.com/mesosphere/dcos-kubernetes-cluster/commit/d74b25e8d7e4e283ba4a66fc0f027669aa4c9fc2
[`mesosphere/dklb`]: https://hub.docker.com/r/mesosphere/dklb
