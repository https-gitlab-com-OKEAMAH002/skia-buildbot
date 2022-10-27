package revision_filter

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/skerr"
)

// ValidHTTPRevisionFilter is a RevisionFilter implementation which
// obtains a single valid revision from a file which is retrieved via HTTP.
type ValidHTTPRevisionFilter struct {
	client           *http.Client
	regex            *regexp.Regexp
	fileURL          string
	validRevision    string
	validRevisionMtx sync.Mutex
}

// NewValidRevisionFromHTTPRevisionFilter returns a RevisionSelector instance.
func NewValidRevisionFromHTTPRevisionFilter(cfg *config.ValidHttpRevisionFilterConfig, client *http.Client) (*ValidHTTPRevisionFilter, error) {
	var regex *regexp.Regexp
	if cfg.Regex != "" {
		var err error
		regex, err = regexp.Compile(cfg.Regex)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
	}
	return &ValidHTTPRevisionFilter{
		client:  client,
		regex:   regex,
		fileURL: cfg.FileUrl,
	}, nil
}

// extractRevision extracts the valid revision out of the given fileContents.
func (f *ValidHTTPRevisionFilter) extractRevision(fileContents []byte) (string, error) {
	if f.regex != nil {
		match := f.regex.FindSubmatch(fileContents)
		if len(match) == 0 {
			return "", skerr.Fmt("no matches found for regex %q in:\n%s", f.regex.String(), string(fileContents))
		}
		if len(match) != 2 {
			return "", skerr.Fmt("multiple matches found for regex %q in:\n%s", f.regex.String(), string(fileContents))
		}
		return string(match[1]), nil
	}
	return strings.TrimSpace(string(fileContents)), nil
}

// Skip implements RevisionFilter.
func (f *ValidHTTPRevisionFilter) Skip(ctx context.Context, r revision.Revision) (string, error) {
	f.validRevisionMtx.Lock()
	defer f.validRevisionMtx.Unlock()
	if r.Id != f.validRevision {
		return fmt.Sprintf("revision %q doesn't match expected revision %q obtained from %s", r.Id, f.validRevision, f.fileURL), nil
	}
	return "", nil
}

// Update implements RevisionFilter.
func (f *ValidHTTPRevisionFilter) Update(ctx context.Context) error {
	// Perform the HTTP request to retrieve the current version of the revision
	// selector file.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.fileURL, nil)
	if err != nil {
		return skerr.Wrapf(err, "failed to create HTTP request")
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return skerr.Wrapf(err, "failed to execute HTTP request")
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return skerr.Wrapf(err, "failed to read response body")
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return skerr.Fmt("got HTTP status code %d; body: %s", resp.StatusCode, string(b))
	}

	// Extract the revision from the file.
	validRevision, err := f.extractRevision(b)
	if err != nil {
		return skerr.Wrapf(err, "failed to extract revision")
	}

	// Update the validRevision.
	f.validRevisionMtx.Lock()
	defer f.validRevisionMtx.Unlock()
	f.validRevision = validRevision
	return nil
}

var _ RevisionFilter = &ValidHTTPRevisionFilter{}
