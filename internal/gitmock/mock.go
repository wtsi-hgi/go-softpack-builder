package gitmock

import (
	crypto "crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"math/rand"
	"net/http"

	"github.com/wtsi-hgi/go-softpack-builder/internal"
)

const (
	refsPath         = "/info/refs"
	refsQuery        = "?service=git-upload-pack"
	headPath         = "/HEAD"
	smartContentType = "application/x-git-upload-pack-advertisement"
	expectedHeader   = "001e# service=git-upload-pack\n0000"
	headRef          = "HEAD"

	numRefGen = 5
	hashLen   = 40
)

const (
	ErrNotFound = internal.Error("not found")
)

// MockGit can be used to start a pretend git server for testing custom spack
// repos.
type MockGit struct {
	refs       map[string]string
	masterName string
	Smart      bool
}

// New returns a new MockGit that provides a git repo with a random number of
// random refs.
func New() (*MockGit, string) {
	numRefs := rand.Intn(numRefGen) + numRefGen //nolint:gosec

	refs := make(map[string]string, numRefs)

	var (
		masterName, masterCommit string
		hash                     [20]byte
	)

	for i := 0; i < numRefs; i++ {
		randChars := make([]byte, rand.Intn(numRefGen)+numRefGen) //nolint:gosec
		crypto.Read(randChars)                                    //nolint:errcheck
		crypto.Read(hash[:])                                      //nolint:errcheck

		masterName = base64.RawStdEncoding.EncodeToString(randChars)
		masterCommit = fmt.Sprintf("%020X", hash)

		refs[masterName] = masterCommit
	}

	return &MockGit{
		refs:       refs,
		masterName: masterName,
	}, masterCommit
}

// ServeHTTP is to implement http.Handler so you can httptest.NewServer(m).
func (m *MockGit) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := m.handle(w, r.URL.Path); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (m *MockGit) handle(w http.ResponseWriter, path string) error {
	switch path {
	case refsPath:
		if m.Smart {
			w.Header().Set("Content-Type", smartContentType)

			return m.handleSmartRefs(w)
		}

		return m.handleRefs(w)
	case headPath:
		return m.handleHead(w)
	}

	return ErrNotFound
}

func (m *MockGit) handleSmartRefs(w io.Writer) error {
	if _, err := fmt.Fprintf(w, "%s002D%s %s\n", expectedHeader, m.refs[m.masterName], headRef); err != nil {
		return err
	}

	for ref, commit := range m.refs {
		if _, err := fmt.Fprintf(w, "%04X%s %s\n", hashLen+1+len(ref), commit, ref); err != nil {
			return err
		}
	}

	_, err := io.WriteString(w, "0000")

	return err
}

func (m *MockGit) handleRefs(w io.Writer) error {
	for ref, commit := range m.refs {
		if _, err := fmt.Fprintf(w, "%s\t%s\n", commit, ref); err != nil {
			return err
		}
	}

	return nil
}

func (m *MockGit) handleHead(w io.Writer) error {
	_, err := io.WriteString(w, "ref: "+m.masterName)

	return err
}
