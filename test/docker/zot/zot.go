// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package zot

import (
	"fmt"
	"time"

	"go.opendefense.cloud/solar/test/docker"
)

type Zot struct {
	*docker.Container

	p *docker.Pool
}

const (
	image      = "ghcr.io/project-zot/zot-minimal"
	version    = "latest"
	name       = "zot"
	portString = "5000/tcp"
)

func New(expiresIn time.Duration) (*Zot, error) {
	pool, err := docker.NewPool()
	if err != nil {
		return nil, fmt.Errorf("could not create a new container pool: %w", err)
	}

	return NewWithPool(pool, expiresIn)
}

func NewWithPool(pool *docker.Pool, expiresIn time.Duration) (*Zot, error) {
	container, err := pool.NewContainerWithOptions(name, image, version, expiresIn, docker.WithExposedPorts(portString))
	if err != nil {
		return nil, err
	}

	return &Zot{p: pool, Container: container}, nil
}

func (z *Zot) GetPort() int {
	p, err := z.Container.GetPort(portString)
	if err != nil {
		panic(err)
	}

	return p
}

func (z *Zot) WaitFor(timeout time.Duration) error {
	return z.Container.WaitFor(portString, timeout)
}
