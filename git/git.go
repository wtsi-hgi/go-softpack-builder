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

package git

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
)

type Error string

func (e Error) Error() string {
	return string(e)
}

const (
	refsPath         = "/info/refs"
	refsQuery        = "?service=git-upload-pack"
	headPath         = "/HEAD"
	smartContentType = "application/x-git-upload-pack-advertisement"
	expectedHeader   = "001e# service=git-upload-pack\n0000"
	headRef          = "HEAD"

	maxHeadRead   = 1024
	minHeadLength = 6
	maxSplitPart  = 2

	ErrInvalidHead = Error("invalid head response")
	ErrInvalidRefs = Error("invalid refs response")
	ErrNoHash      = Error("no hash found")
)

func getURL(url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	return http.DefaultClient.Do(req)
}

// GetLatestCommit gets the latest head commit hash for the given remote git
// repo.
func GetLatestCommit(url string) (string, error) {
	resp, err := getURL(url + refsPath + refsQuery)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	if resp.Header.Get("Content-Type") == smartContentType {
		return getLatestCommitFromSmartResponse(resp.Body)
	}

	return getLatestCommitFromBasicResponse(url, resp.Body)
}

// getLatestCommitFromSmartResponse parses a response that looks like:
//
// 001e# service=git-upload-pack
// 000001554ca80c5acce050fa8f7156af419933cae60b75b0 HEAD\x00multi_ack...
// 003f4ca80c5acce050fa8f7156af419933cae60b75b0 refs/heads/master
// 003ee2357da0f7d1093e39cd72e0301abfcd7d98cf8d refs/tags/v1.0.0.
func getLatestCommitFromSmartResponse(r io.Reader) (string, error) {
	if header, err := io.ReadAll(io.LimitReader(r, int64(len(expectedHeader)))); err != nil {
		return "", err
	} else if string(header) != expectedHeader {
		return "", ErrInvalidRefs
	}

	for {
		line, err := readLine(r)
		if err != nil {
			return "", err
		}

		if commit, ref, err := getHashRefFromLine(line, " "); err != nil {
			return "", err
		} else if ref == headRef {
			return commit, nil
		}
	}
}

func readLine(r io.Reader) (string, error) {
	l, err := readLineLength(r)
	if err != nil {
		return "", err
	}

	line := make([]byte, l)

	if _, err := io.ReadFull(r, line); err != nil {
		return "", err
	}

	return string(line), nil
}

func readLineLength(r io.Reader) (uint64, error) {
	var length [4]byte

	if _, err := io.ReadFull(r, length[:]); err != nil {
		return 0, err
	}

	l, err := strconv.ParseUint(string(length[:]), 16, 16)
	if err != nil {
		return 0, err
	} else if l == 0 {
		return 0, ErrNoHash
	}

	return l, nil
}

func getHashRefFromLine(line string, sep string) (string, string, error) {
	parts := strings.SplitN(line, sep, maxSplitPart)
	if len(parts) != maxSplitPart {
		return "", "", ErrInvalidRefs
	}

	for _, c := range parts[0] {
		if !isHexChar(c) {
			return "", "", ErrInvalidRefs
		}
	}

	return parts[0], strings.TrimSpace(strings.Split(parts[1], "\x00")[0]), nil
}

func isHexChar(c rune) bool {
	return isNum(c) || isAtoF(c)
}

func isNum(c rune) bool {
	return c >= '0' && c <= '9'
}

func isAtoF(c rune) bool {
	return c >= 'A' && c <= 'F' || c >= 'a' && c <= 'f'
}

// getLatestCommitFromBasicResponse parses a response that looks like:
// 4ca80c5acce050fa8f7156af419933cae60b75b0	refs/heads/master
// 4ca80c5acce050fa8f7156af419933cae60b75b0	refs/remotes/github/master
// 5509bde7642645fc5f851598ca0dc528c8f6a085	refs/tags/v1.0.2
//
// /HEAD
// ref: refs/heads/master
func getLatestCommitFromBasicResponse(url string, r io.Reader) (string, error) {
	headRef, err := getBasicHeadRef(url)
	if err != nil {
		return "", err
	}

	br := bufio.NewReader(r)

	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return "", err
		}

		if commit, ref, err := getHashRefFromLine(line, "\t"); err != nil {
			return "", err
		} else if ref == headRef {
			return commit, nil
		}
	}
}

func getBasicHeadRef(url string) (string, error) {
	resp, err := getURL(url + headPath)
	if err != nil {
		return "", err
	}

	headRef, err := io.ReadAll(io.LimitReader(resp.Body, maxHeadRead))

	resp.Body.Close()

	if err != nil {
		return "", err
	} else if len(headRef) < minHeadLength {
		return "", ErrInvalidHead
	}

	return string(headRef[5:]), nil
}
