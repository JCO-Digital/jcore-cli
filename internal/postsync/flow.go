package postsync

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// A sync copies from a source site to a target site: local to remote when
// pushing, remote to local when pulling. The engine only ever talks to the
// two ends of a flow, so the same code serves either direction.

// end is one site of a sync.
type end struct {
	name   string // "local" or "remote", for messages
	title  string // "Local" or "Remote"
	where  string // "locally" or "on the remote"
	site   string // "the local site" or "the remote"
	run    func(...string) (string, error)
	stdin  func(string, ...string) (string, error)
	pll    *PLLState // nil: Polylang isn't active
	domain string    // localDomain or remoteDomain
	wpRoot string    // WordPress root, as that site's wp-cli sees it
	url    string    // siteurl
}

// flow is the direction of one sync.
type flow struct {
	r        Runner
	src, dst *end
	pull     bool
	warnings []string
}

// verb is what running the sync is called in messages.
func (f *flow) verb() string {
	if f.pull {
		return "pull"
	}
	return "sync"
}

// syncHint is the command that copies source post id to the target.
func (f *flow) syncHint(id int) string {
	if f.pull {
		return fmt.Sprintf("jcore sync --pull %d", id)
	}
	return fmt.Sprintf("jcore sync %d", id)
}

// copyVerb is what copying an uploads file to the target is called.
func (f *flow) copyVerb() string {
	if f.pull {
		return "download"
	}
	return "upload"
}

// fileStatus compares the source's wp-content/uploads/<rel> to the
// target's, without changing anything.
func (f *flow) fileStatus(rel string) (FileState, error) {
	if f.pull {
		return f.r.PullFileStatus(rel)
	}
	return f.r.FileStatus(rel)
}

// copyFile copies wp-content/uploads/<rel> to the target, never
// overwriting a file that already exists there.
func (f *flow) copyFile(rel string) error {
	if f.pull {
		return f.r.Download(rel)
	}
	return f.r.Upload(rel)
}

// preflight checks that both sites are reachable single-site installs,
// reads their siteurls and Polylang setup, and returns the flow opts asks
// for. It never writes.
func preflight(r Runner, opts Options) (*flow, error) {
	if _, err := r.Local("core", "is-installed"); err != nil {
		return nil, fmt.Errorf("local WordPress isn't reachable (is the project running? try `jcore start`): %w", err)
	}
	if _, err := r.Local("core", "is-installed", "--network"); err == nil {
		return nil, errors.New("the local site is a multisite network, which sync doesn't support yet")
	}

	local := &end{name: "local", title: "Local", where: "locally", site: "the local site", run: r.Local, stdin: r.LocalStdin, domain: opts.LocalDomain, wpRoot: localWPRoot}
	remote := &end{name: "remote", title: "Remote", where: "on the remote", site: "the remote", run: r.Remote, stdin: r.RemoteStdin, domain: opts.RemoteDomain, wpRoot: opts.RemotePath}
	f := &flow{r: r, src: local, dst: remote, pull: opts.Pull}
	if opts.Pull {
		f.src, f.dst = remote, local
	}

	out, err := r.Remote("option", "get", "siteurl")
	if err != nil {
		return nil, fmt.Errorf("couldn't run wp-cli on the remote over SSH: %w", err)
	}
	remote.url = lastLine(out)
	if _, err := r.Remote("core", "is-installed", "--network"); err == nil {
		return nil, errors.New("the remote site is a multisite network, which sync doesn't support yet")
	}
	if u, err := url.Parse(remote.url); err == nil && opts.RemoteDomain != "" &&
		!strings.EqualFold(strings.TrimPrefix(u.Host, "www."), strings.TrimPrefix(opts.RemoteDomain, "www.")) {
		dir := "to"
		if opts.Pull {
			dir = "from"
		}
		f.warnings = append(f.warnings, fmt.Sprintf("remote siteurl is %s but remoteDomain is %q; URLs will be rewritten %s %q", remote.url, opts.RemoteDomain, dir, opts.RemoteDomain))
	}
	if out, err = r.Local("option", "get", "siteurl"); err != nil {
		return nil, fmt.Errorf("reading the local siteurl: %w", err)
	}
	local.url = lastLine(out)

	if local.pll, err = detectPolylang(r.Local); err != nil {
		return nil, fmt.Errorf("checking for Polylang locally: %w", err)
	}
	if remote.pll, err = detectPolylang(r.Remote); err != nil {
		return nil, fmt.Errorf("checking for Polylang on the remote: %w", err)
	}
	return f, nil
}
