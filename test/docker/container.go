package docker

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"syscall"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/ory/dockertest/v3"
)

// Pool is the base implementation of test containers
// using github.com/ory/dockertest
type Pool struct {
	pool       *dockertest.Pool
	containers map[string]Container
}

type Container struct {
	name  string
	p     *Pool
	r     *dockertest.Resource
	ports map[string]int
}

// New creates a new pool
func NewPool() (*Pool, error) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		return nil, fmt.Errorf("failed to create pool: %w", err)
	}

	return &Pool{
		pool:       pool,
		containers: make(map[string]Container),
	}, nil
}

// WithEnv applies the given key/value pairs in the format ENV_VAR=value to the environment
// of the container
func WithEnv(envKV ...string) func(options *dockertest.RunOptions) {
	return func(options *dockertest.RunOptions) {
		if envKV != nil {
			options.Env = envKV
		}
	}
}

// WithExposedPorts makes all given ports (in the format '8080/tcp') accessible
func WithExposedPorts(ports ...string) func(*dockertest.RunOptions) {
	return func(opts *dockertest.RunOptions) {
		opts.ExposedPorts = ports
	}
}

// NewContainer runs and attaches the given repo/tag, name is a unique identifier of your choice for the
// container
func (p *Pool) NewContainer(name, repo, tag string, expiration time.Duration, env ...string) (*Container, error) {
	return p.NewContainerWithOptions(name, repo, tag, expiration, WithEnv(env...))
}

// NewContainerWithOptions is just like [Pool.NewContainer], but applies the given options before starting the container
func (p *Pool) NewContainerWithOptions(name, repo, tag string, expiration time.Duration, runOptions ...func(options *dockertest.RunOptions)) (*Container, error) {
	if _, exists := p.containers[name]; exists {
		return nil, fmt.Errorf("container with name '%s' already exists", name)
	}

	opts := &dockertest.RunOptions{
		Repository: repo,
		Tag:        tag,
	}

	for _, runOption := range runOptions {
		runOption(opts)
	}

	resource, err := p.pool.RunWithOptions(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	if err = resource.Expire(uint(expiration.Seconds())); err != nil {
		return nil, err
	}

	c := Container{
		name:  name,
		r:     resource,
		ports: make(map[string]int),
	}

	p.containers[name] = c

	return &c, nil
}

func (p *Pool) Close(name string) error {
	container, ok := p.containers[name]
	if !ok {
		return errors.New("container not found")
	}

	if err := container.r.Close(); err != nil {
		return fmt.Errorf("failed to close container: %w", err)
	}

	delete(p.containers, name)

	return nil
}

// WaitFor uses GetPort to find the exposed port of the container
// and waits until it can connect
func (c *Container) WaitFor(port string, timeout time.Duration) error {
	ebo := backoff.NewExponentialBackOff(
		backoff.WithMaxInterval(time.Second),
		backoff.WithMaxElapsedTime(timeout),
	)

	const host = "localhost"

	return backoff.Retry(func() error {
		exposedPort, err := c.GetPort(port)
		if err != nil {
			return err
		}

		dialer := net.Dialer{
			Timeout: timeout,
		}
		conn, err := dialer.DialContext(context.TODO(), "tcp", net.JoinHostPort(host, strconv.Itoa(exposedPort)))
		if err != nil {
			if !errors.Is(err, syscall.ECONNREFUSED) {
				return backoff.Permanent(err)
			}

			return err
		}

		// we don't care
		_ = conn.Close()

		return nil
	}, ebo)
}

// GetPort returns the container port the service was
// published at. servicePort must be passed in the form
// port/protocol, e.g. "6379/tcp"
func (c *Container) GetPort(servicePort string) (int, error) {
	if _, known := c.ports[servicePort]; !known {
		strPort := c.r.GetPort(servicePort)

		if strPort == "" {
			return 0, fmt.Errorf("could not find port for service %s", servicePort)
		}

		port, err := strconv.Atoi(strPort)
		if err != nil {
			return 0, err
		}

		c.ports[servicePort] = port
	}

	return c.ports[servicePort], nil
}

func (c *Container) Close() error {
	return c.p.Close(c.name)
}
