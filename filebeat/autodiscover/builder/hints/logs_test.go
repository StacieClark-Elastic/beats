// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package hints

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-agent-autodiscover/bus"
	conf "github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/logp/logptest"
	"github.com/elastic/elastic-agent-libs/mapstr"
	"github.com/elastic/elastic-agent-libs/paths"
)

func TestMain(m *testing.M) {
	InitializeModule()

	os.Exit(m.Run())
}

func TestGenerateHints(t *testing.T) {
	customDockerCfg := conf.MustNewConfigFrom(map[string]interface{}{
		"default_config": map[string]interface{}{
			"type": "docker",
			"containers": map[string]interface{}{
				"ids": []string{
					"${data.container.id}",
				},
			},
			"close_timeout": "true",
		},
	})

	customContainerCfg := conf.MustNewConfigFrom(map[string]interface{}{
		"default_config": map[string]interface{}{
			"type": "container",
			"paths": []string{
				"/var/lib/docker/containers/${data.container.id}/*-json.log",
			},
			"close_timeout": "true",
			"processors": []interface{}{
				map[string]interface{}{
					"add_tags": map[string]interface{}{
						"tags":   []string{"web"},
						"target": "environment",
					},
				},
			},
		},
	})

	customFilestreamCfg := conf.MustNewConfigFrom(map[string]interface{}{
		"default_config": map[string]interface{}{
			"type": "filestream",
			"id":   "kubernetes-container-logs-${data.kubernetes.container.id}",
			"prospector": map[string]interface{}{
				"scanner": map[string]interface{}{
					"fingerprint.enabled": true,
					"symlinks":            true,
				},
			},
			"file_identity.fingerprint": nil,
			"paths": []string{
				"/var/log/containers/*-${data.kubernetes.container.id}.log",
			},
			"parsers": []interface{}{
				map[string]interface{}{
					"container": map[string]interface{}{
						"stream": "all",
						"format": "auto",
					},
				},
			},
		},
	})

	defaultCfg := conf.NewConfig()

	defaultDisabled := conf.MustNewConfigFrom(map[string]interface{}{
		"default_config": map[string]interface{}{
			"enabled": "false",
		},
	})

	tests := []struct {
		msg    string
		config *conf.C
		event  bus.Event
		len    int
		result []mapstr.M
	}{
		{
			msg:    "Default config is correct(default input)",
			config: defaultCfg,
			event: bus.Event{
				"host": "1.2.3.4",
				"kubernetes": mapstr.M{
					"container": mapstr.M{
						"name": "foobar",
						"id":   "abc",
					},
				},
				"container": mapstr.M{
					"name": "foobar",
					"id":   "abc",
				},
			},
			len: 1,
			result: []mapstr.M{
				{
					"id":    "kubernetes-container-logs-abc",
					"paths": []interface{}{"/var/log/containers/*-abc.log"},
					"parsers": []interface{}{
						map[string]interface{}{
							"container": map[string]interface{}{
								"format": "auto",
								"stream": "all",
							},
						},
					},
					"prospector": map[string]interface{}{
						"scanner": map[string]interface{}{
							"symlinks": true,
							"fingerprint": map[string]interface{}{
								"enabled": true,
							},
						},
					},
					"file_identity": map[string]interface{}{
						"fingerprint": nil,
					},
					"type": "filestream",
				},
			},
		},
		{
			msg:    "Config disabling works",
			config: defaultDisabled,
			event: bus.Event{
				"host": "1.2.3.4",
				"kubernetes": mapstr.M{
					"container": mapstr.M{
						"name": "foobar",
						"id":   "abc",
					},
				},
				"container": mapstr.M{
					"name": "foobar",
					"id":   "abc",
				},
			},
			len:    0,
			result: []mapstr.M{},
		},
		{
			msg:    "Hint to enable when disabled by default works(filestream)",
			config: defaultDisabled,
			event: bus.Event{
				"host": "1.2.3.4",
				"kubernetes": mapstr.M{
					"container": mapstr.M{
						"name": "foobar",
						"id":   "abc",
					},
				},
				"container": mapstr.M{
					"name": "foobar",
					"id":   "abc",
				},
				"hints": mapstr.M{
					"logs": mapstr.M{
						"enabled":       "true",
						"exclude_lines": "^test2, ^test3",
					},
				},
			},
			len: 1,
			result: []mapstr.M{
				{
					"id":    "kubernetes-container-logs-abc",
					"paths": []interface{}{"/var/log/containers/*-abc.log"},
					"parsers": []interface{}{
						map[string]interface{}{
							"container": map[string]interface{}{
								"format": "auto",
								"stream": "all",
							},
						},
					},
					"prospector": map[string]interface{}{
						"scanner": map[string]interface{}{
							"symlinks": true,
							"fingerprint": map[string]interface{}{
								"enabled": true,
							},
						},
					},
					"file_identity": map[string]interface{}{
						"fingerprint": nil,
					},
					"exclude_lines": []interface{}{"^test2", "^test3"},
					"type":          "filestream",
				},
			},
		},
		{
			msg:    "Hints without host should return nothing",
			config: customDockerCfg,
			event: bus.Event{
				"hints": mapstr.M{
					"metrics": mapstr.M{
						"module": "prometheus",
					},
				},
			},
			len:    0,
			result: []mapstr.M{},
		},
		{
			msg:    "Hints with logs.disable should return nothing",
			config: customDockerCfg,
			event: bus.Event{
				"hints": mapstr.M{
					"logs": mapstr.M{
						"disable": "true",
					},
				},
			},
			len:    0,
			result: []mapstr.M{},
		},
		{
			msg:    "Empty event hints should return default config(docker input)",
			config: customDockerCfg,
			event: bus.Event{
				"host": "1.2.3.4",
				"kubernetes": mapstr.M{
					"container": mapstr.M{
						"name": "foobar",
						"id":   "abc",
					},
				},
				"container": mapstr.M{
					"name": "foobar",
					"id":   "abc",
				},
			},
			len: 1,
			result: []mapstr.M{
				{
					"type": "docker",
					"containers": map[string]interface{}{
						"ids": []interface{}{"abc"},
					},
					"close_timeout": "true",
				},
			},
		},
		{
			msg:    "Hint with include|exclude_lines must be part of the input config(docker input)",
			config: customDockerCfg,
			event: bus.Event{
				"host": "1.2.3.4",
				"kubernetes": mapstr.M{
					"container": mapstr.M{
						"name": "foobar",
						"id":   "abc",
					},
				},
				"container": mapstr.M{
					"name": "foobar",
					"id":   "abc",
				},
				"hints": mapstr.M{
					"logs": mapstr.M{
						"include_lines": "^test, ^test1",
						"exclude_lines": "^test2, ^test3",
					},
				},
			},
			len: 1,
			result: []mapstr.M{
				{
					"type": "docker",
					"containers": map[string]interface{}{
						"ids": []interface{}{"abc"},
					},
					"include_lines": []interface{}{"^test", "^test1"},
					"exclude_lines": []interface{}{"^test2", "^test3"},
					"close_timeout": "true",
				},
			},
		},
		{
			msg:    "Hints with  two sets of include|exclude_lines must be part of the input config(docker input)",
			config: customDockerCfg,
			event: bus.Event{
				"host": "1.2.3.4",
				"kubernetes": mapstr.M{
					"container": mapstr.M{
						"name": "foobar",
						"id":   "abc",
					},
				},
				"container": mapstr.M{
					"name": "foobar",
					"id":   "abc",
				},
				"hints": mapstr.M{
					"logs": mapstr.M{
						"1": mapstr.M{
							"exclude_lines": "^test1, ^test2",
						},
						"2": mapstr.M{
							"include_lines": "^test1, ^test2",
						},
					},
				},
			},
			len: 2,
			result: []mapstr.M{
				{
					"type": "docker",
					"containers": map[string]interface{}{
						"ids": []interface{}{"abc"},
					},
					"exclude_lines": []interface{}{"^test1", "^test2"},
					"close_timeout": "true",
				},
				{
					"type": "docker",
					"containers": map[string]interface{}{
						"ids": []interface{}{"abc"},
					},
					"include_lines": []interface{}{"^test1", "^test2"},
					"close_timeout": "true",
				},
			},
		},
		{
			msg:    "Hint with multiline config must have a multiline in the input config(docker input)",
			config: customDockerCfg,
			event: bus.Event{
				"host": "1.2.3.4",
				"kubernetes": mapstr.M{
					"container": mapstr.M{
						"name": "foobar",
						"id":   "abc",
					},
				},
				"container": mapstr.M{
					"name": "foobar",
					"id":   "abc",
				},
				"hints": mapstr.M{
					"logs": mapstr.M{
						"multiline": mapstr.M{
							"pattern": "^test",
							"negate":  "true",
						},
					},
				},
			},
			len: 1,
			result: []mapstr.M{
				{
					"type": "docker",
					"containers": map[string]interface{}{
						"ids": []interface{}{"abc"},
					},
					"multiline": map[string]interface{}{
						"pattern": "^test",
						"negate":  "true",
					},
					"close_timeout": "true",
				},
			},
		},
		{
			msg:    "Hint with multiline config must have a multiline in the input config parsers(filestream input)",
			config: customFilestreamCfg,
			event: bus.Event{
				"host": "1.2.3.4",
				"kubernetes": mapstr.M{
					"container": mapstr.M{
						"name": "foobar",
						"id":   "abc",
					},
				},
				"container": mapstr.M{
					"name": "foobar",
					"id":   "abc",
				},
				"hints": mapstr.M{
					"logs": mapstr.M{
						"multiline": mapstr.M{
							"pattern": "^test",
							"negate":  "true",
						},
					},
				},
			},
			len: 1,
			result: []mapstr.M{
				{
					"id":    "kubernetes-container-logs-abc",
					"paths": []interface{}{"/var/log/containers/*-abc.log"},
					"parsers": []interface{}{
						map[string]interface{}{
							"container": map[string]interface{}{
								"format": "auto",
								"stream": "all",
							},
						},
						map[string]interface{}{
							"multiline": map[string]interface{}{
								"pattern": "^test",
								"negate":  "true",
							},
						},
					},
					"prospector": map[string]interface{}{
						"scanner": map[string]interface{}{
							"symlinks": true,
							"fingerprint": map[string]interface{}{
								"enabled": true,
							},
						},
					},
					"file_identity": map[string]interface{}{
						"fingerprint": nil,
					},
					"type": "filestream",
				},
			},
		},
		{
			msg:    "Hint with json config options must include them in the input config ndjson parser(filestream input)",
			config: customFilestreamCfg,
			event: bus.Event{
				"host": "1.2.3.4",
				"kubernetes": mapstr.M{
					"container": mapstr.M{
						"name": "foobar",
						"id":   "abc",
					},
				},
				"container": mapstr.M{
					"name": "foobar",
					"id":   "abc",
				},
				"hints": mapstr.M{
					"logs": mapstr.M{
						"json": mapstr.M{
							"add_error_key": true,
							"expand_keys":   true,
						},
					},
				},
			},
			len: 1,
			result: []mapstr.M{
				{
					"id":    "kubernetes-container-logs-abc",
					"paths": []interface{}{"/var/log/containers/*-abc.log"},
					"parsers": []interface{}{
						map[string]interface{}{
							"container": map[string]interface{}{
								"format": "auto",
								"stream": "all",
							},
						},
						map[string]interface{}{
							"ndjson": map[string]interface{}{
								"add_error_key": true,
								"expand_keys":   true,
							},
						},
					},
					"prospector": map[string]interface{}{
						"scanner": map[string]interface{}{
							"symlinks": true,
							"fingerprint": map[string]interface{}{
								"enabled": true,
							},
						},
					},
					"file_identity": map[string]interface{}{
						"fingerprint": nil,
					},
					"type": "filestream",
				},
			},
		},
		{
			msg:    "Hint with json config options must include them in the input config(container input)",
			config: customContainerCfg,
			event: bus.Event{
				"host": "1.2.3.4",
				"kubernetes": mapstr.M{
					"container": mapstr.M{
						"name": "foobar",
						"id":   "abc",
					},
				},
				"container": mapstr.M{
					"name": "foobar",
					"id":   "abc",
				},
				"hints": mapstr.M{
					"logs": mapstr.M{
						"json": mapstr.M{
							"add_error_key": true,
							"expand_keys":   true,
						},
					},
				},
			},
			len: 1,
			result: []mapstr.M{
				{
					"type": "container",
					"paths": []interface{}{
						"/var/lib/docker/containers/abc/*-json.log",
					},
					"close_timeout": "true",
					"json": map[string]interface{}{
						"add_error_key": true,
						"expand_keys":   true,
					},
					"processors": []interface{}{
						map[string]interface{}{
							"add_tags": map[string]interface{}{
								"tags":   []interface{}{"web"},
								"target": "environment",
							},
						},
					},
				},
			},
		},
		{
			msg:    "Hint with inputs config as json must be accepted(docker input)",
			config: customDockerCfg,
			event: bus.Event{
				"host": "1.2.3.4",
				"kubernetes": mapstr.M{
					"container": mapstr.M{
						"name": "foobar",
						"id":   "abc",
					},
				},
				"container": mapstr.M{
					"name": "foobar",
					"id":   "abc",
				},
				"hints": mapstr.M{
					"logs": mapstr.M{
						"raw": "[{\"containers\":{\"ids\":[\"${data.container.id}\"]},\"multiline\":{\"negate\":\"true\",\"pattern\":\"^test\"},\"type\":\"docker\"}]",
					},
				},
			},
			len: 1,
			result: []mapstr.M{
				{
					"type": "docker",
					"containers": map[string]interface{}{
						"ids": []interface{}{"abc"},
					},
					"multiline": map[string]interface{}{
						"pattern": "^test",
						"negate":  "true",
					},
				},
			},
		},
		{
			msg:    "Hint with processors config must have a processors in the input config(docker input)",
			config: customDockerCfg,
			event: bus.Event{
				"host": "1.2.3.4",
				"kubernetes": mapstr.M{
					"container": mapstr.M{
						"name": "foobar",
						"id":   "abc",
					},
				},
				"container": mapstr.M{
					"name": "foobar",
					"id":   "abc",
				},
				"hints": mapstr.M{
					"logs": mapstr.M{
						"processors": mapstr.M{
							"1": mapstr.M{
								"dissect": mapstr.M{
									"tokenizer": "%{key1} %{key2}",
								},
							},
							"drop_event": mapstr.M{},
						},
					},
				},
			},
			len: 1,
			result: []mapstr.M{
				{
					"type": "docker",
					"containers": map[string]interface{}{
						"ids": []interface{}{"abc"},
					},
					"close_timeout": "true",
					"processors": []interface{}{
						map[string]interface{}{
							"dissect": map[string]interface{}{
								"tokenizer": "%{key1} %{key2}",
							},
						},
						map[string]interface{}{
							"drop_event": nil,
						},
					},
				},
			},
		},
		{
			msg:    "Processors in hints must be appended in the processors of the default config(container input)",
			config: customContainerCfg,
			event: bus.Event{
				"host": "1.2.3.4",
				"kubernetes": mapstr.M{
					"container": mapstr.M{
						"name": "foobar",
						"id":   "abc",
					},
				},
				"container": mapstr.M{
					"name": "foobar",
					"id":   "abc",
				},
				"hints": mapstr.M{
					"logs": mapstr.M{
						"processors": mapstr.M{
							"1": mapstr.M{
								"dissect": mapstr.M{
									"tokenizer": "%{key1} %{key2}",
								},
							},
							"drop_event": mapstr.M{},
						},
					},
				},
			},
			len: 1,
			result: []mapstr.M{
				{
					"type": "container",
					"paths": []interface{}{
						"/var/lib/docker/containers/abc/*-json.log",
					},
					"close_timeout": "true",
					"processors": []interface{}{
						map[string]interface{}{
							"add_tags": map[string]interface{}{
								"tags":   []interface{}{"web"},
								"target": "environment",
							},
						},
						map[string]interface{}{
							"dissect": map[string]interface{}{
								"tokenizer": "%{key1} %{key2}",
							},
						},
						map[string]interface{}{
							"drop_event": nil,
						},
					},
				},
			},
		},
		{
			msg:    "Hint with module should attach input to its filesets(docker input)",
			config: customDockerCfg,
			event: bus.Event{
				"host": "1.2.3.4",
				"kubernetes": mapstr.M{
					"container": mapstr.M{
						"name": "foobar",
						"id":   "abc",
					},
				},
				"container": mapstr.M{
					"name": "foobar",
					"id":   "abc",
				},
				"hints": mapstr.M{
					"logs": mapstr.M{
						"module": "apache",
					},
				},
			},
			len: 1,
			result: []mapstr.M{
				{
					"module": "apache",
					"error": map[string]interface{}{
						"enabled": true,
						"input": map[string]interface{}{
							"type": "docker",
							"containers": map[string]interface{}{
								"stream": "all",
								"ids":    []interface{}{"abc"},
							},
							"close_timeout": "true",
						},
					},
					"access": map[string]interface{}{
						"enabled": true,
						"input": map[string]interface{}{
							"type": "docker",
							"containers": map[string]interface{}{
								"stream": "all",
								"ids":    []interface{}{"abc"},
							},
							"close_timeout": "true",
						},
					},
				},
			},
		},
		{
			msg:    "Hint with module should honor defined filesets(docker input)",
			config: customDockerCfg,
			event: bus.Event{
				"host": "1.2.3.4",
				"kubernetes": mapstr.M{
					"container": mapstr.M{
						"name": "foobar",
						"id":   "abc",
					},
				},
				"container": mapstr.M{
					"name": "foobar",
					"id":   "abc",
				},
				"hints": mapstr.M{
					"logs": mapstr.M{
						"module":  "apache",
						"fileset": "access",
					},
				},
			},
			len: 1,
			result: []mapstr.M{
				{
					"module": "apache",
					"access": map[string]interface{}{
						"enabled": true,
						"input": map[string]interface{}{
							"type": "docker",
							"containers": map[string]interface{}{
								"stream": "all",
								"ids":    []interface{}{"abc"},
							},
							"close_timeout": "true",
						},
					},
					"error": map[string]interface{}{
						"enabled": false,
						"input": map[string]interface{}{
							"type": "docker",
							"containers": map[string]interface{}{
								"stream": "all",
								"ids":    []interface{}{"abc"},
							},
							"close_timeout": "true",
						},
					},
				},
			},
		},
		{
			msg:    "Hint with module should honor defined filesets with streams(docker input)",
			config: customDockerCfg,
			event: bus.Event{
				"host": "1.2.3.4",
				"kubernetes": mapstr.M{
					"container": mapstr.M{
						"name": "foobar",
						"id":   "abc",
					},
				},
				"container": mapstr.M{
					"name": "foobar",
					"id":   "abc",
				},
				"hints": mapstr.M{
					"logs": mapstr.M{
						"module":         "apache",
						"fileset.stdout": "access",
						"fileset.stderr": "error",
					},
				},
			},
			len: 1,
			result: []mapstr.M{
				{
					"module": "apache",
					"access": map[string]interface{}{
						"enabled": true,
						"input": map[string]interface{}{
							"type": "docker",
							"containers": map[string]interface{}{
								"stream": "stdout",
								"ids":    []interface{}{"abc"},
							},
							"close_timeout": "true",
						},
					},
					"error": map[string]interface{}{
						"enabled": true,
						"input": map[string]interface{}{
							"type": "docker",
							"containers": map[string]interface{}{
								"stream": "stderr",
								"ids":    []interface{}{"abc"},
							},
							"close_timeout": "true",
						},
					},
				},
			},
		},
		{
			msg:    "Hint with module should attach input to its filesets(default input)",
			config: defaultCfg,
			event: bus.Event{
				"host": "1.2.3.4",
				"kubernetes": mapstr.M{
					"container": mapstr.M{
						"name": "foobar",
						"id":   "abc",
					},
				},
				"container": mapstr.M{
					"name": "foobar",
					"id":   "abc",
				},
				"hints": mapstr.M{
					"logs": mapstr.M{
						"module": "apache",
					},
				},
			},
			len: 1,
			result: []mapstr.M{
				{
					"module": "apache",
					"error": map[string]interface{}{
						"enabled": true,
						"input": map[string]interface{}{
							"id":    "kubernetes-container-logs-abc",
							"paths": []interface{}{"/var/log/containers/*-abc.log"},
							"parsers": []interface{}{
								map[string]interface{}{
									"container": map[string]interface{}{
										"format": "auto",
										"stream": "all",
									},
								},
							},
							"prospector": map[string]interface{}{
								"scanner": map[string]interface{}{
									"symlinks": true,
									"fingerprint": map[string]interface{}{
										"enabled": true,
									},
								},
							},
							"file_identity": map[string]interface{}{
								"fingerprint": nil,
							},
							"type": "filestream",
						},
					},
					"access": map[string]interface{}{
						"enabled": true,
						"input": map[string]interface{}{
							"id":    "kubernetes-container-logs-abc",
							"paths": []interface{}{"/var/log/containers/*-abc.log"},
							"parsers": []interface{}{
								map[string]interface{}{
									"container": map[string]interface{}{
										"format": "auto",
										"stream": "all",
									},
								},
							},
							"prospector": map[string]interface{}{
								"scanner": map[string]interface{}{
									"symlinks": true,
									"fingerprint": map[string]interface{}{
										"enabled": true,
									},
								},
							},
							"file_identity": map[string]interface{}{
								"fingerprint": nil,
							},
							"type": "filestream",
						},
					},
				},
			},
		},
		{
			msg:    "Hint with module should honor defined filesets(default input)",
			config: defaultCfg,
			event: bus.Event{
				"host": "1.2.3.4",
				"kubernetes": mapstr.M{
					"container": mapstr.M{
						"name": "foobar",
						"id":   "abc",
					},
				},
				"container": mapstr.M{
					"name": "foobar",
					"id":   "abc",
				},
				"hints": mapstr.M{
					"logs": mapstr.M{
						"module":  "apache",
						"fileset": "access",
					},
				},
			},
			len: 1,
			result: []mapstr.M{
				{
					"module": "apache",
					"access": map[string]interface{}{
						"enabled": true,
						"input": map[string]interface{}{
							"id":    "kubernetes-container-logs-abc",
							"paths": []interface{}{"/var/log/containers/*-abc.log"},
							"parsers": []interface{}{
								map[string]interface{}{
									"container": map[string]interface{}{
										"format": "auto",
										"stream": "all",
									},
								},
							},
							"prospector": map[string]interface{}{
								"scanner": map[string]interface{}{
									"symlinks": true,
									"fingerprint": map[string]interface{}{
										"enabled": true,
									},
								},
							},
							"file_identity": map[string]interface{}{
								"fingerprint": nil,
							},
							"type": "filestream",
						},
					},
					"error": map[string]interface{}{
						"enabled": false,
						"input": map[string]interface{}{
							"id":    "kubernetes-container-logs-abc",
							"paths": []interface{}{"/var/log/containers/*-abc.log"},
							"parsers": []interface{}{
								map[string]interface{}{
									"container": map[string]interface{}{
										"format": "auto",
										"stream": "all",
									},
								},
							},
							"prospector": map[string]interface{}{
								"scanner": map[string]interface{}{
									"symlinks": true,
									"fingerprint": map[string]interface{}{
										"enabled": true,
									},
								},
							},
							"file_identity": map[string]interface{}{
								"fingerprint": nil,
							},
							"type": "filestream",
						},
					},
				},
			},
		},
		{
			msg:    "Hint with module should honor defined filesets with streams(default input)",
			config: defaultCfg,
			event: bus.Event{
				"host": "1.2.3.4",
				"kubernetes": mapstr.M{
					"container": mapstr.M{
						"name": "foobar",
						"id":   "abc",
					},
				},
				"container": mapstr.M{
					"name": "foobar",
					"id":   "abc",
				},
				"hints": mapstr.M{
					"logs": mapstr.M{
						"module":         "apache",
						"fileset.stdout": "access",
						"fileset.stderr": "error",
					},
				},
			},
			len: 1,
			result: []mapstr.M{
				{
					"module": "apache",
					"access": map[string]interface{}{
						"enabled": true,
						"input": map[string]interface{}{
							"id":    "kubernetes-container-logs-abc",
							"paths": []interface{}{"/var/log/containers/*-abc.log"},
							"parsers": []interface{}{
								map[string]interface{}{
									"container": map[string]interface{}{
										"format": "auto",
										"stream": "stdout",
									},
								},
							},
							"prospector": map[string]interface{}{
								"scanner": map[string]interface{}{
									"symlinks": true,
									"fingerprint": map[string]interface{}{
										"enabled": true,
									},
								},
							},
							"file_identity": map[string]interface{}{
								"fingerprint": nil,
							},
							"type": "filestream",
						},
					},
					"error": map[string]interface{}{
						"enabled": true,
						"input": map[string]interface{}{
							"id":    "kubernetes-container-logs-abc",
							"paths": []interface{}{"/var/log/containers/*-abc.log"},
							"parsers": []interface{}{
								map[string]interface{}{
									"container": map[string]interface{}{
										"format": "auto",
										"stream": "stderr",
									},
								},
							},
							"prospector": map[string]interface{}{
								"scanner": map[string]interface{}{
									"symlinks": true,
									"fingerprint": map[string]interface{}{
										"enabled": true,
									},
								},
							},
							"file_identity": map[string]interface{}{
								"fingerprint": nil,
							},
							"type": "filestream",
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		// Configure path for modules access
		abs, _ := filepath.Abs("../../..")
		require.NoError(t, paths.InitPaths(&paths.Path{
			Home: abs,
		}))

		logger := logptest.NewTestingLogger(t, "")
		l, err := NewLogHints(test.config, logger)
		if err != nil {
			t.Fatal(err)
		}

		cfgs := l.CreateConfig(test.event)
		assert.Equal(t, test.len, len(cfgs), test.msg)
		configs := make([]mapstr.M, 0)
		for _, cfg := range cfgs {
			config := mapstr.M{}
			err := cfg.Unpack(&config)
			ok := assert.Nil(t, err, test.msg)
			if !ok {
				break
			}
			configs = append(configs, config)
		}
		assert.Equal(t, test.result, configs, test.msg)
	}
}

func TestGenerateHintsWithPaths(t *testing.T) {
	tests := []struct {
		msg    string
		event  bus.Event
		path   string
		len    int
		result mapstr.M
	}{
		{
			msg: "Empty event hints should return default config",
			event: bus.Event{
				"host": "1.2.3.4",
				"kubernetes": mapstr.M{
					"container": mapstr.M{
						"name": "foobar",
						"id":   "abc",
					},
					"pod": mapstr.M{
						"name": "pod",
						"uid":  "12345",
					},
				},
				"container": mapstr.M{
					"name": "foobar",
					"id":   "abc",
				},
			},
			path: "/var/lib/docker/containers/${data.kubernetes.container.id}/*-json.log",
			len:  1,
			result: mapstr.M{
				"type": "docker",
				"containers": map[string]interface{}{
					"paths": []interface{}{"/var/lib/docker/containers/abc/*-json.log"},
				},
				"close_timeout": "true",
			},
		},
		{
			msg: "Hint with processors config must have a processors in the input config",
			event: bus.Event{
				"host": "1.2.3.4",
				"kubernetes": mapstr.M{
					"container": mapstr.M{
						"name": "foobar",
						"id":   "abc",
					},
					"pod": mapstr.M{
						"name": "pod",
						"uid":  "12345",
					},
				},
				"container": mapstr.M{
					"name": "foobar",
					"id":   "abc",
				},
				"hints": mapstr.M{
					"logs": mapstr.M{
						"processors": mapstr.M{
							"1": mapstr.M{
								"dissect": mapstr.M{
									"tokenizer": "%{key1} %{key2}",
								},
							},
							"drop_event": mapstr.M{},
						},
					},
				},
			},
			len:  1,
			path: "/var/log/pods/${data.kubernetes.pod.uid}/${data.kubernetes.container.name}/*.log",
			result: mapstr.M{
				"type": "docker",
				"containers": map[string]interface{}{
					"paths": []interface{}{"/var/log/pods/12345/foobar/*.log"},
				},
				"close_timeout": "true",
				"processors": []interface{}{
					map[string]interface{}{
						"dissect": map[string]interface{}{
							"tokenizer": "%{key1} %{key2}",
						},
					},
					map[string]interface{}{
						"drop_event": nil,
					},
				},
			},
		},
		{
			msg: "Hint with module should attach input to its filesets",
			event: bus.Event{
				"host": "1.2.3.4",
				"kubernetes": mapstr.M{
					"container": mapstr.M{
						"name": "foobar",
						"id":   "abc",
					},
					"pod": mapstr.M{
						"name": "pod",
						"uid":  "12345",
					},
				},
				"container": mapstr.M{
					"name": "foobar",
					"id":   "abc",
				},
				"hints": mapstr.M{
					"logs": mapstr.M{
						"module": "apache",
					},
				},
			},
			len:  1,
			path: "/var/log/pods/${data.kubernetes.pod.uid}/${data.kubernetes.container.name}/*.log",
			result: mapstr.M{
				"module": "apache",
				"error": map[string]interface{}{
					"enabled": true,
					"input": map[string]interface{}{
						"type": "docker",
						"containers": map[string]interface{}{
							"stream": "all",
							"paths":  []interface{}{"/var/log/pods/12345/foobar/*.log"},
						},
						"close_timeout": "true",
					},
				},
				"access": map[string]interface{}{
					"enabled": true,
					"input": map[string]interface{}{
						"type": "docker",
						"containers": map[string]interface{}{
							"stream": "all",
							"paths":  []interface{}{"/var/log/pods/12345/foobar/*.log"},
						},
						"close_timeout": "true",
					},
				},
			},
		},
		{
			msg: "Hint with module should honor defined filesets",
			event: bus.Event{
				"host": "1.2.3.4",
				"kubernetes": mapstr.M{
					"container": mapstr.M{
						"name": "foobar",
						"id":   "abc",
					},
					"pod": mapstr.M{
						"name": "pod",
						"uid":  "12345",
					},
				},
				"container": mapstr.M{
					"name": "foobar",
					"id":   "abc",
				},
				"hints": mapstr.M{
					"logs": mapstr.M{
						"module":  "apache",
						"fileset": "access",
					},
				},
			},
			len:  1,
			path: "/var/log/pods/${data.kubernetes.pod.uid}/${data.kubernetes.container.name}/*.log",
			result: mapstr.M{
				"module": "apache",
				"access": map[string]interface{}{
					"enabled": true,
					"input": map[string]interface{}{
						"type": "docker",
						"containers": map[string]interface{}{
							"stream": "all",
							"paths":  []interface{}{"/var/log/pods/12345/foobar/*.log"},
						},
						"close_timeout": "true",
					},
				},
				"error": map[string]interface{}{
					"enabled": false,
					"input": map[string]interface{}{
						"type": "docker",
						"containers": map[string]interface{}{
							"stream": "all",
							"paths":  []interface{}{"/var/log/pods/12345/foobar/*.log"},
						},
						"close_timeout": "true",
					},
				},
			},
		},
	}

	for _, test := range tests {
		cfg, _ := conf.NewConfigFrom(map[string]interface{}{
			"default_config": map[string]interface{}{
				"type": "docker",
				"containers": map[string]interface{}{
					"paths": []string{
						test.path,
					},
				},
				"close_timeout": "true",
			},
		})

		// Configure path for modules access
		abs, _ := filepath.Abs("../../..")
		require.NoError(t, paths.InitPaths(&paths.Path{
			Home: abs,
		}))
		logger := logptest.NewTestingLogger(t, "")
		l, err := NewLogHints(cfg, logger)
		if err != nil {
			t.Fatal(err)
		}

		cfgs := l.CreateConfig(test.event)
		require.Equal(t, test.len, len(cfgs), test.msg)
		if test.len != 0 {
			config := mapstr.M{}
			err := cfgs[0].Unpack(&config)
			assert.Nil(t, err, test.msg)

			assert.Equal(t, test.result, config, test.msg)
		}

	}
}
