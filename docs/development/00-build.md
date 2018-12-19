# Building `dklb`

## Prerequisites

To build `dklb`, the following software is required:

* [`git`].
* [`make`].
* [Go] 1.11+.
  * `dklb` makes use of the [Go modules] experiment present in Go 1.11+ only.
  
To run `dklb`, the following additional software is required:
  
* [`skaffold`]
* A [DC/OS] 1.12+ cluster having [EdgeLB] installed.
* An [MKE] cluster.
  * The current kubeconfig context must be configured to point at this cluster.
  
A Docker Hub account with read/write access to the [`mesosphere/dklb`] image repository is additionally required.

## Cloning the repository

To clone the repository, the following command may be run:

```console
$ git clone git@github.com:mesosphere/dklb.git /some/path
```

**NOTE:** As `dklb` relies on Go modules, it is **NOT** necessary to clone to a directory inside `$GOPATH`.

## Installing dependencies

To install all the dependencies required to build `dklb`, the following command may be run:

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

[`git`]: https://git-scm.com/
[Go]: https://golang.org/
[Go modules]: https://github.com/golang/go/wiki/Modules
[`make`]: https://www.gnu.org/software/make/
[`skaffold`]: https://github.com/GoogleContainerTools/skaffold
[DC/OS]: https://dcos.io/
[EdgeLB]: https://docs.mesosphere.com/services/edge-lb/
[MKE]: https://mesosphere.com/product/kubernetes-engine/
[`mesosphere/dklb`]: https://hub.docker.com/r/mesosphere/dklb
