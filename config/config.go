/*******************************************************************************
 * Copyright (c) 2023 Genome Research Ltd.
 *
 * Permission is hereby granted, free of charge, to any person obtaining
 * a copy of this software and associated documentation files (the
 * "Software"), to deal in the Software without restriction, including
 * without limitation the rights to use, copy, modify, merge, publish,
 * distribute, sublicense, and/or sell copies of the Software, and to
 * permit persons to whom the Software is furnished to do so, subject to
 * the following conditions:
 *
 * The above copyright notice and this permission notice shall be included
 * in all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
 * EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
 * MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.
 * IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY
 * CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT,
 * TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE
 * SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
 ******************************************************************************/

package config

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"

	"github.com/wtsi-hgi/go-softpack-builder/internal"
	yaml "gopkg.in/yaml.v3"
)

// Config holds our config options.
type Config struct {
	S3 struct {
		BinaryCache string `yaml:"binaryCache"`
		BuildBase   string `yaml:"buildBase"`
	} `yaml:"s3"`
	Module struct {
		ModuleInstallDir  string   `yaml:"moduleInstallDir"`
		ScriptsInstallDir string   `yaml:"scriptsInstallDir"`
		LoadPath          string   `yaml:"loadPath"`
		Dependencies      []string `yaml:"dependencies"`
		WrapperScript     string   `yaml:"wrapperScript"`
	} `yaml:"module"`
	CustomSpackRepo string `yaml:"customSpackRepo"`
	Spack           struct {
		BuildImage      string `yaml:"buildImage"`
		FinalImage      string `yaml:"finalImage"`
		ProcessorTarget string `yaml:"processorTarget"`
	} `yaml:"spack"`
	CoreURL      string `yaml:"coreURL"`
	ListenURL    string `yaml:"listenURL"`
	WRDeployment string `yaml:"wrDeployment"`
}

// GetConfig returns a config based on the given config file path. If it's
// blank, looks for ~/.softpack/builder/gsb-config.yml.
func GetConfig(configPath string) (*Config, error) {
	if configPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, internal.Error(fmt.Sprintf("could not get home dir: %s", err))
		}

		configPath = filepath.Join(home, ".softpack", "builder", "gsb-config.yml")
	}

	f, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}

	conf, err := Parse(f)
	f.Close()

	if err != nil {
		return nil, err
	}

	return conf, nil
}

// Parse parses a YAML file of our config options.
func Parse(r io.Reader) (*Config, error) {
	c := new(Config)
	if err := yaml.NewDecoder(r).Decode(c); err != nil {
		return nil, err
	}

	if c.CustomSpackRepo != "" {
		if _, err := url.Parse(c.CustomSpackRepo); err != nil {
			return nil, fmt.Errorf("invalid customSpackRepo.url: %w", err)
		}
	}

	if c.CoreURL != "" {
		if _, err := url.Parse(c.CoreURL); err != nil {
			return nil, fmt.Errorf("invalid coreURL: %w", err)
		}
	}

	return c, nil
}
