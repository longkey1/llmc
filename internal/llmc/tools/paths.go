package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PathRules is a list of path patterns.
//
// A pattern containing a path separator matches a path and everything under
// it, after `~` expansion and resolution against the current directory:
// "~/.ssh" covers "~/.ssh/config". A pattern without a separator is matched
// against each component of the path, as a glob: ".git" covers any .git
// directory, and "*.pem" covers any .pem file.
type PathRules []string

// pathDecision applies the allow/deny rules to a file path. The path is
// canonicalized first: rules are useless if "./a/../../../etc/hosts" or a
// symlinked parent directory can sidestep them.
func pathDecision(path string, allowed, denied PathRules, unlisted UnlistedAction) decision {
	resolved, err := resolvePath(path)
	if err != nil {
		// An unresolvable path is treated as unlisted rather than allowed.
		return unlistedDecision(unlisted)
	}

	if matchesPathRules(resolved, denied) {
		return decisionDeny
	}
	if matchesPathRules(resolved, allowed) {
		return decisionAllow
	}
	return unlistedDecision(unlisted)
}

// resolvePath turns a user-supplied path into an absolute path with symlinks
// resolved, so that rule matching sees the file that would actually be
// touched. The path need not exist: when it doesn't, the parent directory is
// resolved instead.
func resolvePath(path string) (string, error) {
	expanded, err := expandHome(path)
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(expanded)
	if err != nil {
		return "", fmt.Errorf("resolving path: %v", err)
	}

	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved, nil
	}
	dir, name := filepath.Split(abs)
	if resolvedDir, err := filepath.EvalSymlinks(filepath.Clean(dir)); err == nil {
		return filepath.Join(resolvedDir, name), nil
	}
	return abs, nil
}

// expandHome expands a leading "~" to the user's home directory. The
// "~user" form is not supported.
func expandHome(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~"+string(filepath.Separator)) {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("expanding ~: %v", err)
	}
	if path == "~" {
		return home, nil
	}
	return filepath.Join(home, path[2:]), nil
}

// matchesPathRules reports whether a canonical path matches any of the rules.
func matchesPathRules(path string, rules PathRules) bool {
	for _, rule := range rules {
		rule = strings.TrimSpace(rule)
		if rule == "" {
			continue
		}
		if strings.ContainsRune(rule, filepath.Separator) {
			if matchesPathPrefix(path, rule) {
				return true
			}
			continue
		}
		if matchesPathComponent(path, rule) {
			return true
		}
	}
	return false
}

// matchesPathPrefix reports whether path is the rule's path or lives under it.
// filepath.Rel is used rather than a string prefix so that a rule for "/tmp/a"
// does not match "/tmp/ab".
func matchesPathPrefix(path, rule string) bool {
	base, err := resolvePath(rule)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return false
	}
	return rel == "." || !strings.HasPrefix(rel, "..")
}

// matchesPathComponent reports whether any component of path matches the glob.
func matchesPathComponent(path, pattern string) bool {
	for part := range strings.SplitSeq(path, string(filepath.Separator)) {
		if part == "" {
			continue
		}
		if ok, err := filepath.Match(pattern, part); err == nil && ok {
			return true
		}
	}
	return false
}

// refuseSymlink reports an error when path itself is a symbolic link. Writing
// through a link would affect a file outside the path the user confirmed.
func refuseSymlink(path string) error {
	expanded, err := expandHome(path)
	if err != nil {
		return err
	}
	info, err := os.Lstat(expanded)
	if err != nil {
		// A missing target is fine; the write creates it.
		return nil
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, _ := os.Readlink(expanded)
		return fmt.Errorf("%s is a symbolic link (to %s); llmc does not write through symlinks", path, target)
	}
	return nil
}
