# Quick Start

To try out SolAr, you can install it and run example orders.

## Prerequisites

Before installing SolAr, you need a Kubernetes cluster and `kubectl` configured to access it.
For quick testing, you can use a local cluster with [kind](https://kind.sigs.k8s.io/) or similar tools.

SolAr requires [cert-manager](https://cert-manager.io/docs/installation/) as a dependency.

!!! note

    These instructions are intended to help you get started quickly. They are not suitable for production. For production installs, please refer to the [installation documentation](./operator-manual/installation.md).

## Install SolAr

## Kustomize

First, specify the version you want to install in an environment variable. Modify the command below:

    SOLAR_VERSION="main"

Then, copy the commands below to apply the kustomization:

    kubectl apply -k "https://github.com/opendefensecloud/solution-arsenal/examples/deployment?ref=${SOLAR_VERSION}"

## Submit an example order

// TODO
