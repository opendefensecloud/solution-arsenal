# Home

[![Build status](https://github.com/opendefensecloud/solution-arsenal/actions/workflows/golang.yaml/badge.svg)](https://github.com/opendefensecloud/solution-arsenal/actions/workflows/golang.yaml)
[![Coverage Status](https://coveralls.io/repos/github/opendefensecloud/solution-arsenal/badge.svg?branch=main)](https://coveralls.io/github/opendefensecloud/solution-arsenal?branch=main)

## What is Solution Arsenal (SolAr)?

Solution Arsenal (SolAr) is an application catalog based on Open Component Model packages ([ocm.software](https://ocm.software)) and fleet rollout management for these solutions onto Kubernetes Clusters.

## System Architecture

For a detailed architecture overview, see the [Architecture documentation](./developer-guide/architecture.md).

## Core Concepts

SolAr manages software delivery through several key resources:

- **Component / ComponentVersion** — OCM components representing deployable software packages, discovered automatically by solar-discovery
- **Release** — a deployment configuration for a ComponentVersion
- **Target** — a deployment target environment (e.g. a cluster), references a render Registry
- **Registry** — an OCI registry configuration with hostname and push credentials
- **ReleaseBinding** — declares that a Release should be deployed to a Target
- **Profile** — matches Targets by label selector and automatically creates ReleaseBindings for a Release
- **RenderTask** — internal resource that drives Helm chart rendering jobs
- **Discovery** — automated scanning of OCI registries for new OCM components

For the complete API specification, see the [API Reference](./user-guide/api-reference.md).

## Quickstart

- [Get started here](getting-started.md)

## Project Resources

- [Open Defense Cloud Organization](https://github.com/opendefensecloud)
- [Solution Arsenal Repository](https://github.com/opendefensecloud/solution-arsenal)
