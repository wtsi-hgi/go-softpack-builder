package remove

import (
	"encoding/json"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/wtsi-hgi/go-softpack-builder/build"
	"github.com/wtsi-hgi/go-softpack-builder/config"
	"github.com/wtsi-hgi/go-softpack-builder/core"
)

const groupsDir = "groups"

type mockS3 struct{}

func (mockS3) RemoveFile(_ string) error {
	return nil
}

func TestRemove(t *testing.T) {
	programLevel := new(slog.LevelVar)

	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: programLevel})))

	programLevel.Set(slog.LevelDebug)

	Convey("With a valid config and a test environment to be removed", t, func() {
		conf, group, env, version := createTestEnv(t)

		envPath := filepath.Join(groupsDir, group, env)

		var response core.Response

		mockCore := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			json.NewEncoder(w).Encode(response) //nolint:errcheck
		}))

		conf.CoreURL = mockCore.URL

		s3Mock := new(mockS3)

		Convey("Remove() call fails if the environments module dir or script dir is not removable", func() {
			for _, p := range [...]string{
				filepath.Join(conf.Module.ModuleInstallDir, groupsDir, group),
				filepath.Join(conf.Module.ModuleInstallDir, groupsDir, group, env),
				filepath.Join(conf.Module.ScriptsInstallDir, groupsDir, group),
				filepath.Join(conf.Module.ScriptsInstallDir, groupsDir, group, env),
				filepath.Join(conf.Module.ScriptsInstallDir, groupsDir, group, env, version+build.ScriptsDirSuffix),
			} {
				err := os.Chmod(p, 0)
				So(err, ShouldBeNil)

				err = Remove(conf, s3Mock, envPath, version)
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldEqual, "no write access to dir ("+p+"): permission denied")

				err = os.Chmod(p, 0755)
				So(err, ShouldBeNil)
			}

			_, err := os.Stat(filepath.Join(conf.Module.ModuleInstallDir, groupsDir, group, env, version))
			So(err, ShouldBeNil)

			_, err = os.Stat(filepath.Join(conf.Module.ScriptsInstallDir, groupsDir,
				group, env, version+build.ScriptsDirSuffix, core.ImageBasename))
			So(err, ShouldBeNil)

			removing := filepath.Join(conf.Module.ModuleInstallDir, groupsDir, group, env)

			err = os.RemoveAll(removing)
			So(err, ShouldBeNil)

			err = Remove(conf, s3Mock, envPath, version)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldEqual, "no write access to dir ("+removing+"): no such file or directory")

			_, err = os.Stat(filepath.Join(conf.Module.ModuleInstallDir, groupsDir, group, env, version))
			So(err, ShouldNotBeNil)
		})

		Convey("Remove() call fails if environment is not successfully removed from Core", func() {
			response.Data.DeleteEnvironment = &core.EnvironmentResponse{
				TypeName: "EnvironmentNotFoundError",
				Message:  "No environment with this name found in this location.",
			}

			err := Remove(conf, s3Mock, envPath, version)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldEqual, "No environment with this name found in this location.")

			conf.CoreURL = "http://invalid-url:1234/"

			err = Remove(conf, s3Mock, envPath, version)
			So(err, ShouldNotBeNil)
		})

		Convey("Can use Remove() to delete an existing environment", func() {
			response.Data.DeleteEnvironment = &core.EnvironmentResponse{
				TypeName: core.DeleteMutationSuccess,
				Message:  "Successfully deleted the environment",
			}

			modulePath := filepath.Join(conf.Module.ModuleInstallDir, groupsDir, group, env)
			scriptsPath := filepath.Join(conf.Module.ScriptsInstallDir, groupsDir, group,
				env, version+build.ScriptsDirSuffix)

			err := Remove(conf, s3Mock, envPath, version)
			So(err, ShouldBeNil)

			_, err = os.Stat(modulePath)
			So(err, ShouldWrap, os.ErrNotExist)

			_, err = os.Stat(scriptsPath)
			So(err, ShouldWrap, os.ErrNotExist)
		})
	})
}

func createTestEnv(t *testing.T) (*config.Config, string, string, string) {
	t.Helper()

	conf := new(config.Config)
	conf.Module.ModuleInstallDir = t.TempDir()
	conf.Module.ScriptsInstallDir = t.TempDir()

	group := genRandString(5)
	env := genRandString(8)
	version := genRandString(3)

	modulePath := filepath.Join(conf.Module.ModuleInstallDir, groupsDir, group, env)
	scriptsPath := filepath.Join(conf.Module.ScriptsInstallDir, groupsDir, group, env, version+build.ScriptsDirSuffix)

	err := os.MkdirAll(modulePath, 0755)
	So(err, ShouldBeNil)

	err = os.MkdirAll(scriptsPath, 0755)
	So(err, ShouldBeNil)

	f, err := os.Create(filepath.Join(modulePath, version))
	So(err, ShouldBeNil)

	_, err = io.WriteString(f, "A Module File")
	So(err, ShouldBeNil)
	So(f.Close(), ShouldBeNil)

	f, err = os.Create(filepath.Join(scriptsPath, core.ImageBasename))
	So(err, ShouldBeNil)

	_, err = io.WriteString(f, "An Image File")
	So(err, ShouldBeNil)
	So(f.Close(), ShouldBeNil)

	return conf, group, env, version
}

func genRandString(length int) string {
	var sb strings.Builder

	sb.Grow(length)

	for ; length > 0; length-- {
		letter := rand.Intn(52) //nolint:gosec

		if letter < 26 {
			sb.WriteByte(byte(letter) + 'A')
		} else {
			sb.WriteByte(byte(letter) - 26 + 'a')
		}
	}

	return sb.String()
}
