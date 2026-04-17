// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart/loader"
	"helm.sh/helm/v4/pkg/cli"
	releasev1 "helm.sh/helm/v4/pkg/release/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestRenderer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Renderer")
}

// helmTemplate templates a helm chart in path given the release name and namespace.
// It returns a list of the templated manifests or an error.
func helmTemplate(name, namespace, path string) ([]unstructured.Unstructured, error) {
	GinkgoHelper()

	settings := cli.New()
	actionConfig := &action.Configuration{}

	if err := actionConfig.Init(settings.RESTClientGetter(), namespace, os.Getenv("HELM_DRIVER")); err != nil {
		return nil, err
	}

	chart, err := loader.Load(path)
	if err != nil {
		return nil, err
	}

	install := action.NewInstall(actionConfig)
	install.DryRunStrategy = action.DryRunClient
	install.ReleaseName = name
	install.Namespace = namespace

	releaser, err := install.Run(chart, nil)
	if err != nil {
		return nil, err
	}

	rel, ok := releaser.(*releasev1.Release)
	if !ok {
		return nil, fmt.Errorf("releaser could not be cast to a release")
	}

	manifests := []unstructured.Unstructured{}
	for m := range strings.SplitSeq(rel.Manifest, "---") {
		if strings.TrimSpace(m) == "" {
			continue
		}

		var object map[string]any
		if err := yaml.Unmarshal([]byte(m), &object); err != nil {
			return nil, err
		}
		manifest := unstructured.Unstructured{
			Object: object,
		}
		manifests = append(manifests, manifest)
	}

	return manifests, nil
}
