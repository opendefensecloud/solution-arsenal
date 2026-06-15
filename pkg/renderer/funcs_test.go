// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("template helper functions", func() {
	Describe("toYAML", func() {
		It("marshals a value to YAML without a trailing newline", func() {
			Expect(toYAML(map[string]string{"foo": "bar"})).To(Equal("foo: bar"))
		})

		It("returns an empty string for values that cannot be marshaled", func() {
			Expect(toYAML(make(chan int))).To(Equal(""))
		})
	})

	Describe("mustToYAML", func() {
		It("marshals a value to YAML without a trailing newline", func() {
			Expect(mustToYAML(map[string]string{"foo": "bar"})).To(Equal("foo: bar"))
		})

		It("panics for values that cannot be marshaled", func() {
			Expect(func() { mustToYAML(make(chan int)) }).To(Panic())
		})
	})

	Describe("fromYAML", func() {
		It("unmarshals valid YAML into a map", func() {
			Expect(fromYAML("foo: bar")).To(Equal(map[string]any{"foo": "bar"}))
		})

		It("returns the error under the Error key for invalid YAML", func() {
			result := fromYAML("foo: [bar")
			Expect(result).To(HaveKey("Error"))
		})
	})

	Describe("fromYAMLArray", func() {
		It("unmarshals a valid YAML array", func() {
			Expect(fromYAMLArray("- foo\n- bar")).To(Equal([]any{"foo", "bar"}))
		})

		It("returns the error as a single-element slice for invalid YAML", func() {
			result := fromYAMLArray("- [bar")
			Expect(result).To(HaveLen(1))
		})
	})

	Describe("toJSON", func() {
		It("marshals a value to JSON", func() {
			Expect(toJSON(map[string]string{"foo": "bar"})).To(Equal(`{"foo":"bar"}`))
		})

		It("returns an empty string for values that cannot be marshaled", func() {
			Expect(toJSON(make(chan int))).To(Equal(""))
		})
	})

	Describe("mustToJSON", func() {
		It("marshals a value to JSON", func() {
			Expect(mustToJSON(map[string]string{"foo": "bar"})).To(Equal(`{"foo":"bar"}`))
		})

		It("panics for values that cannot be marshaled", func() {
			Expect(func() { mustToJSON(make(chan int)) }).To(Panic())
		})
	})

	Describe("fromJSON", func() {
		It("unmarshals valid JSON into a map", func() {
			Expect(fromJSON(`{"foo":"bar"}`)).To(Equal(map[string]any{"foo": "bar"}))
		})

		It("returns the error under the Error key for invalid JSON", func() {
			result := fromJSON("{not valid json")
			Expect(result).To(HaveKey("Error"))
		})
	})

	Describe("fromJSONArray", func() {
		It("unmarshals a valid JSON array", func() {
			Expect(fromJSONArray(`["foo","bar"]`)).To(Equal([]any{"foo", "bar"}))
		})

		It("returns the error as a single-element slice for invalid JSON", func() {
			result := fromJSONArray("[not valid")
			Expect(result).To(HaveLen(1))
		})
	})

	Describe("funcMap", func() {
		It("registers the template helper functions and removes env/expandenv", func() {
			fm := funcMap()

			for _, name := range []string{
				"toYaml", "mustToYaml", "fromYaml", "fromYamlArray",
				"toJson", "mustToJson", "fromJson", "fromJsonArray",
			} {
				Expect(fm).To(HaveKey(name))
			}

			Expect(fm).NotTo(HaveKey("env"))
			Expect(fm).NotTo(HaveKey("expandenv"))
		})
	})
})
