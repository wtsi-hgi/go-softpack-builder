package remove

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/wtsi-hgi/go-softpack-builder/build"
	"github.com/wtsi-hgi/go-softpack-builder/config"
	"golang.org/x/sys/unix"
)

type Error string

func (e Error) Error() string {
	return string(e)
}

type coreResponse struct {
	Data struct {
		Message string `json:"message,omitempty"`
		Path    string `json:"path,omitempty"`
		Name    string `json:"name,omitempty"`
	} `json:"data"`
}

type S3Remover interface {
	RemoveFile(string) error
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

type graphQLDeleteEnvironmentVariables struct {
	Name    string `json:"name"`
	EnvPath string `json:"envPath"`
}

type graphQLDeleteEnvironmentMutation struct {
	Query     string `json:"query"` //graphQLDeleteEnvironment
	Variables string `json:"variables"`
}

func Remove(conf *config.Config, s S3Remover, envPath, version string) error {
	envPath = filepath.Clean(string(filepath.Separator) + envPath)[1:]

	modulePath := filepath.Join(conf.Module.ModuleInstallDir, envPath)
	scriptPath := filepath.Join(conf.Module.ScriptsInstallDir, envPath, version+build.ScriptsDirSuffix)

	if err := checkWriteAccess(modulePath, scriptPath); err != nil {
		return err
	}

	if err := removeEnvFromCore(conf, envPath+"-"+version); err != nil {
		return err
	}

	if err := removeLocalFiles(modulePath, scriptPath); err != nil {
		return err
	}

	if err := removeFromS3(s, modulePath); err != nil {
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
	fmt.Printf("Removing env %s from core\n", envPath)

	variables := graphQLDeleteEnvironmentVariables{
		Name:    filepath.Base(envPath),
		EnvPath: filepath.Dir(envPath),
	}

	var buf bytes.Buffer

	json.NewEncoder(&buf).Encode(variables)

	mutation := graphQLDeleteEnvironmentMutation{
		Query:     graphQLDeleteEnvironment,
		Variables: strings.TrimSpace(buf.String()),
	}

	buf.Reset()

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

	if cr.Data.Name != "" {
		return Error(cr.Data.Message)
	}

	return nil
}

func removeLocalFiles(modulePath, scriptPath string) error {
	err := removeAllNoDescend(modulePath)
	if err != nil {
		return err
	}

	err = removeAllNoDescend(scriptPath)
	if err != nil {
		return err
	}

	return nil
}

func removeAllNoDescend(path string) error {
	files, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	for _, file := range files {
		toRemove := filepath.Join(path, file.Name())

		fmt.Printf("Removing file: %s\n", toRemove)

		if err := os.Remove(toRemove); err != nil {
			return err
		}
	}

	return os.Remove(path)
}

var files = [...]string{"build.out", "singularity.def", "singularity.sif", "executables"}

func removeFromS3(s S3Remover, path string) error {
	for _, file := range files {
		toRemove := filepath.Join(path, file)

		fmt.Printf("Removing file from S3: %s\n", toRemove)

		if err := s.RemoveFile(toRemove); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}

	return nil
}
