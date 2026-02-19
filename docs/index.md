# Home

[![Build status](https://github.com/opendefensecloud/solution-arsenal/actions/workflows/golang.yaml/badge.svg)](https://github.com/opendefensecloud/solution-arsenal/actions/workflows/golang.yaml)
[![Coverage Status](https://coveralls.io/repos/github/opendefensecloud/solution-arsenal/badge.svg?branch=main)](https://coveralls.io/github/opendefensecloud/solution-arsenal?branch=main)

## What is Solution Arsenal (SolAr)?

Solution Arsenal (SolAr) is an application catalog based on Open Component Model packages ([ocm.software](https://ocm.software)) and fleet rollout management for these solutions onto Kubernetes Clusters.

## System Architecture

SolAr is implemented as a Kubernetes Extension API Server integrated with the Kubernetes API Aggregation Layer. This architectural approach provides several advantages over Custom Resource Definitions (CRDs), including dedicated storage isolation, custom API implementation flexibility, and reduced risk to the hosting cluster's control plane.

## Core Concepts

- [Core Concepts](./user-guide/core-concepts.md)

## Quickstart

- [Get started here](getting-started.md)

## Project Resources

- [Open Defense Cloud Organization](https://github.com/opendefensecloud)
- [Solution Arsenal Repository](https://github.com/opendefensecloud/solution-arsenal)
