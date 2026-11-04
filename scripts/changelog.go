package scripts

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	git "github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

var repositoryName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*/[A-Za-z0-9][A-Za-z0-9_.-]*$`)
var fullCommit = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)
var squashSubject = regexp.MustCompile(`^(.*) \(#([0-9]+)\)$`)

func ResolveRelease(repo *git.Repository, requested string) (plumbing.Hash, error) {
	if requested == "" || strings.HasPrefix(requested, "-") {
		return plumbing.ZeroHash, errors.New("empty or option-shaped release revision")
	}
	if fullCommit.MatchString(requested) {
		hash := plumbing.NewHash(requested)
		_, err := repo.CommitObject(hash)
		return hash, err
	}
	candidates := []plumbing.ReferenceName{plumbing.NewBranchReferenceName(requested), plumbing.NewRemoteReferenceName("origin", requested)}
	if strings.HasPrefix(requested, "refs/") {
		candidates = []plumbing.ReferenceName{plumbing.ReferenceName(requested)}
	}
	for _, name := range candidates {
		if err := name.Validate(); err != nil {
			return plumbing.ZeroHash, err
		}
		ref, err := repo.Reference(name, true)
		if err == nil {
			_, err = repo.CommitObject(ref.Hash())
			return ref.Hash(), err
		}
		if !errors.Is(err, plumbing.ErrReferenceNotFound) {
			return plumbing.ZeroHash, err
		}
	}
	if strings.HasPrefix(requested, "refs/") {
		return plumbing.ZeroHash, fmt.Errorf("release ref not found: %s", requested)
	}
	source := plumbing.NewBranchReferenceName(requested)
	destination := plumbing.NewRemoteReferenceName("origin", requested)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err := repo.FetchContext(ctx, &git.FetchOptions{RemoteName: "origin", RefSpecs: []gitconfig.RefSpec{gitconfig.RefSpec("+" + source.String() + ":" + destination.String())}})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return plumbing.ZeroHash, fmt.Errorf("fetch %s: %w", requested, err)
	}
	ref, err := repo.Reference(destination, true)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	_, err = repo.CommitObject(ref.Hash())
	return ref.Hash(), err
}

func GenerateChangelog(directory, repository, base, head string) (string, error) {
	if !repositoryName.MatchString(repository) || strings.HasSuffix(repository, ".git") || strings.Contains(repository, "..") {
		return "", errors.New("set CHANGELOG_REPO to a validated GitHub owner/repository")
	}
	repo, err := git.PlainOpenWithOptions(directory, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return "", err
	}
	baseHash, err := ResolveRelease(repo, base)
	if err != nil {
		return "", err
	}
	headHash, err := ResolveRelease(repo, head)
	if err != nil {
		return "", err
	}
	return ChangelogBetween(repo, repository, baseHash, headHash, strings.TrimPrefix(head, "release/"))
}

func ChangelogBetween(repo *git.Repository, repository string, base, head plumbing.Hash, label string) (string, error) {
	if !repositoryName.MatchString(repository) {
		return "", errors.New("invalid changelog repository")
	}
	excluded := make(map[plumbing.Hash]bool)
	history, err := repo.Log(&git.LogOptions{From: base})
	if err != nil {
		return "", err
	}
	err = history.ForEach(func(commit *object.Commit) error { excluded[commit.Hash] = true; return nil })
	history.Close()
	if err != nil {
		return "", err
	}
	history, err = repo.Log(&git.LogOptions{From: head, Order: git.LogOrderCommitterTime})
	if err != nil {
		return "", err
	}
	defer history.Close()
	titles := make(map[uint64]string)
	err = history.ForEach(func(commit *object.Commit) error {
		if excluded[commit.Hash] {
			return nil
		}
		subject := strings.SplitN(commit.Message, "\n", 2)[0]
		match := squashSubject.FindStringSubmatch(subject)
		if match == nil {
			return nil
		}
		number, parseErr := strconv.ParseUint(match[2], 10, 64)
		if parseErr != nil {
			return parseErr
		}
		if _, exists := titles[number]; !exists {
			titles[number] = match[1]
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(titles) == 0 {
		return "", nil
	}
	numbers := make([]uint64, 0, len(titles))
	for number := range titles {
		numbers = append(numbers, number)
	}
	sort.Slice(numbers, func(i, j int) bool { return numbers[i] > numbers[j] })
	var output strings.Builder
	fmt.Fprintf(&output, "## %s\npax-chain\n", label)
	for _, number := range numbers {
		fmt.Fprintf(&output, "* [#%d](https://github.com/%s/pull/%d) %s\n", number, repository, number, titles[number])
	}
	return output.String(), nil
}
