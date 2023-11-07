package remove

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/wtsi-hgi/go-softpack-builder/build"
	"github.com/wtsi-hgi/go-softpack-builder/config"
	"golang.org/x/sys/unix"
)

type Error string

func (e Error) Error() string {
	return string(e)
}

type coreResponse struct {
	Message string `json:"message,omitempty"`
	Path    string `json:"path,omitempty"`
	Name    string `json:"name,omitempty"`
}

const graphQLDeleteEnvironment = `mutation ($name: String!, $envPath: String!) {
	deleteEnvironment(
			name: $name
			path: $envPath
	) {
		... on DeleteEnvironmentSuccess {
				message
		}
		... on EnvironmentNotFoundError {
				message
				path
				name
		}
	}
}`

const graphQLEndpoint = "/graphql"

type graphQLDeleteEnvironmentMutation struct {
	Query     string `json:"query"` //graphQLDeleteEnvironment
	Variables struct {
		Name    string `json:"name"`
		EnvPath string `json:"envPath"`
	} `json:"variables"`
}

func Remove(conf *config.Config, envPath, version string) error {
	envPath = filepath.Clean(string(filepath.Separator) + envPath)[1:]

	modulePath := filepath.Join(conf.Module.ModuleInstallDir, envPath)
	scriptPath := filepath.Join(conf.Module.ScriptsInstallDir, envPath, version+build.ScriptsDirSuffix)

	if err := checkWriteAccess(modulePath, scriptPath); err != nil {
		return err
	}

	if err := removeEnvFromCore(conf, envPath); err != nil {
		return err
	}

	if err := removeLocalFiles(modulePath, scriptPath); err != nil {
		return err
	}

	return nil
}

func checkWriteAccess(modulePath, scriptPath string) error {
	for _, p := range [...]string{
		filepath.Dir(modulePath),
		modulePath,
		filepath.Dir(filepath.Dir(scriptPath)),
		filepath.Dir(scriptPath),
		scriptPath,
	} {
		if err := unix.Access(p, unix.W_OK); err != nil {
			return Error(fmt.Sprintf("no write access to dir (%s): %s", p, err))
		}
	}

	return nil
}

func removeEnvFromCore(conf *config.Config, envPath string) error {
	mutation := graphQLDeleteEnvironmentMutation{
		Query: graphQLDeleteEnvironment,
	}

	mutation.Variables.Name = filepath.Base(envPath)
	mutation.Variables.EnvPath = filepath.Dir(envPath)

	var buf bytes.Buffer

	json.NewEncoder(&buf).Encode(mutation)

	req, err := http.NewRequest(http.MethodPost, conf.CoreURL+graphQLEndpoint, &buf)
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	var cr coreResponse

	err = json.NewDecoder(resp.Body).Decode(&cr)
	if err != nil {
		return err
	}

	if cr.Name != "" {
		return Error(cr.Message)
	}

	return nil
}

func removeLocalFiles(modulePath, scriptPath string) error {
	err := os.RemoveAll(modulePath)
	if err != nil {
		return err
	}

	err = os.RemoveAll(scriptPath)
	if err != nil {
		return err
	}

	return nil
}
