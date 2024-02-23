package remove

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/wtsi-hgi/go-softpack-builder/build"
	"github.com/wtsi-hgi/go-softpack-builder/config"
	"github.com/wtsi-hgi/go-softpack-builder/core"
	"golang.org/x/sys/unix"
)

var s3BasenamesForDeletion = [...]string{ //nolint:gochecknoglobals
	core.SingularityDefBasename,
	core.ExesBasename,
	core.SoftpackYaml,
	core.SpackLockFile,
	core.BuilderOut,
	core.UsageBasename,
	core.ImageBasename,
}

type Error string

func (e Error) Error() string {
	return string(e)
}

type s3Remover interface {
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

type graphQLDeleteEnvironmentMutation struct {
	Query     string `json:"query"` // graphQLDeleteEnvironment
	Variables struct {
		Name    string `json:"name"`
		EnvPath string `json:"envPath"`
	} `json:"variables"`
}

// Remove will attempt to remove an environments artefacts from Core, S3, and
// the installed locations.
func Remove(conf *config.Config, s3r s3Remover, envPath, version string) error {
	envDir, envName := filepath.Split(envPath)
	modulePath := build.ModuleDirFromName(conf.Module.ModuleInstallDir, envDir, envName)
	scriptPath := build.ScriptsDirFromNameAndVersion(conf.Module.ScriptsInstallDir, envDir, envName, version)

	if err := checkWriteAccess(modulePath, scriptPath); err != nil {
		return err
	}

	slog.Info(fmt.Sprintf("removing env %s from core\n", envPath))

	core, err := core.New(conf)
	if err != nil {
		return err
	}

	err = core.Delete(envPath + "-" + version)
	if err != nil {
		return err
	}

	if err := removeLocalFiles(modulePath, scriptPath); err != nil {
		return err
	}

	return removeFromS3(s3r, modulePath)
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

func removeLocalFiles(modulePath, scriptPath string) error {
	if err := removeAllNoDescend(modulePath); err != nil {
		return err
	}

	return removeAllNoDescend(scriptPath)
}

func removeAllNoDescend(path string) error {
	files, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	for _, file := range files {
		toRemove := filepath.Join(path, file.Name())

		slog.Info(fmt.Sprintf("Removing file: %s\n", toRemove))

		if err := os.Remove(toRemove); err != nil {
			return err
		}
	}

	slog.Info(fmt.Sprintf("Removing directory: %s\n", path))

	return os.Remove(path)
}

func removeFromS3(s3r s3Remover, path string) error {
	for _, file := range s3BasenamesForDeletion {
		toRemove := filepath.Join(path, file)

		slog.Info(fmt.Sprintf("Removing file from S3: %s\n", toRemove))

		if err := s3r.RemoveFile(toRemove); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}

	return nil
}
