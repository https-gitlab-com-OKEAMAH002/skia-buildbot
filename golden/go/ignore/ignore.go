package ignore

import (
	"context"
	"net/url"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/util"
)

// Store is an interface for a database that saves ignore rules.
type Store interface {
	// Create adds a new rule to the ignore store. The ID will be set if this call is successful.
	Create(context.Context, Rule) error

	// List returns all ignore rules in the ignore store.
	List(context.Context) ([]Rule, error)

	// Update sets a Rule.
	Update(ctx context.Context, rule Rule) error

	// Delete removes a Rule from the store. The boolean indicates if the deletion was successful
	// or not (e.g. the rule didn't exist).
	Delete(ctx context.Context, id string) (bool, error)
}

// Rule defines a single ignore rule, matching zero or more traces based on
// Query.
type Rule struct {
	// ID is the id used to store this Rule in a Store. They should be unique.
	ID string
	// Name is the email of the user who created the rule.
	CreatedBy string
	// UpdatedBy is the email of the user who last updated the rule.
	UpdatedBy string
	// Expires indicates a time at which a human should re-consider the rule and see if
	// it still needs to be applied.
	Expires time.Time
	// Query is a url-encoded set of key-value pairs that can be used to match traces.
	// For example: "config=angle_d3d9_es2&cpu_or_gpu_value=RadeonHD7770"
	Query string
	// Note is a comment by a developer, typically a bug.
	Note string
}

// NewRule creates a new ignore rule with the given data.
func NewRule(createdByUser string, expires time.Time, queryStr string, note string) Rule {
	return Rule{
		CreatedBy: createdByUser,
		UpdatedBy: createdByUser,
		Expires:   expires,
		Query:     queryStr,
		Note:      note,
	}
}

// toQuery makes a slice of url.Values from the given slice of Rules.
func toQuery(ignores []Rule) ([]url.Values, error) {
	var ret []url.Values
	for _, ignore := range ignores {
		v, err := url.ParseQuery(ignore.Query)
		if err != nil {
			return nil, skerr.Wrapf(err, "invalid ignore rule id %q; query %q", ignore.ID, ignore.Query)
		}
		ret = append(ret, v)
	}
	return ret, nil
}

// FilterIgnored returns a copy of the given tile with all traces removed
// that match the ignore rules in the given ignore store. It also returns the
// ignore rules for later matching.
func FilterIgnored(inputTile *tiling.Tile, ignores []Rule) (*tiling.Tile, paramtools.ParamMatcher, error) {
	// Make a shallow copy with a new Traces map
	ret := &tiling.Tile{
		Traces:   map[tiling.TraceID]tiling.Trace{},
		ParamSet: inputTile.ParamSet,
		Commits:  inputTile.Commits,

		Scale:     inputTile.Scale,
		TileIndex: inputTile.TileIndex,
	}

	// Then, add any traces that don't match any ignore rules
	ignoreQueries, err := toQuery(ignores)
	if err != nil {
		return nil, nil, err
	}
nextTrace:
	for id, tr := range inputTile.Traces {
		for _, q := range ignoreQueries {
			if tiling.Matches(tr, q) {
				continue nextTrace
			}
		}
		ret.Traces[id] = tr
	}

	ignoreRules := make([]paramtools.ParamSet, len(ignoreQueries))
	for idx, q := range ignoreQueries {
		ignoreRules[idx] = paramtools.ParamSet(q)
	}
	return ret, ignoreRules, nil
}

func oneStep(ctx context.Context, store Store, metric metrics2.Int64Metric) error {
	list, err := store.List(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}
	n := 0
	for _, rule := range list {
		if time.Now().After(rule.Expires) {
			n += 1
		}
	}
	metric.Update(int64(n))
	return nil
}

// StartMetrics starts a new monitoring routine for the given
// ignore.Store that counts expired ignore rules and pushes
// that info into a metric.
func StartMetrics(ctx context.Context, store Store, interval time.Duration) error {
	numExpired := metrics2.GetInt64Metric("gold_num_expired_ignore_rules", nil)
	liveness := metrics2.NewLiveness("gold_expired_ignore_rules_monitoring")

	if err := oneStep(ctx, store, numExpired); err != nil {
		return skerr.Wrapf(err, "starting to monitor ignore rules")
	}
	go util.RepeatCtx(interval, ctx, func(ctx context.Context) {
		if err := oneStep(ctx, store, numExpired); err != nil {
			sklog.Errorf("Failed one step of monitoring ignore rules: %s", err)
			return
		}
		liveness.Reset()
	})
	return nil
}
