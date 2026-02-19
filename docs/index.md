# Home

[![Build status](https://github.com/opendefensecloud/solution-arsenal/actions/workflows/golang.yaml/badge.svg)](https://github.com/opendefensecloud/solution-arsenal/actions/workflows/golang.yaml)
[![Coverage Status](https://coveralls.io/repos/github/opendefensecloud/solution-arsenal/badge.svg?branch=main)](https://coveralls.io/github/opendefensecloud/solution-arsenal?branch=main)

## What is Solution Arsenal (SolAr)?

Solution Arsenal (SolAr) is an application catalog based on Open Component Model packages ([ocm.software](https://ocm.software)) and fleet rollout management for these solutions onto Kubernetes Clusters.

## System Architecture

For a detailed architecture overview, see the [Architecture documentation](./developer-guide/architecture.md).

## Core Concepts

SolAr manages software delivery through several key resources:

- **Component/ComponentVersion** - OCM components representing deployable software packages
- **Release** - A specific deployment instance of a component with configuration
- **Target** - A deployment target environment (cluster/namespace)
- **HydratedTarget** - A fully resolved target with concrete releases and configuration
- **Discovery** - Automated scanning of registries for new components

For the complete API specification, see the [API Reference](./user-guide/api-reference.md).

## Quickstart

- [Get started here](getting-started.md)

## Project Resources

- [Open Defense Cloud Organization](https://github.com/opendefensecloud)
- [Solution Arsenal Repository](https://github.com/opendefensecloud/solution-arsenal)
