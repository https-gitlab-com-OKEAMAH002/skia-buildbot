package cd_metrics

/*
	Package cd_metrics ingests data about commits and Docker images to produce
	metrics, for example the latency between a commit landing and a Docker image
	being built for it.
*/

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"strings"
	"time"

	"go.skia.org/infra/go/gcr"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/gitstore/bt_gitstore"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/metrics2/events"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"golang.org/x/oauth2"
)

const (
	containerRegistryProject = "skia-public"
	louhiUser                = "louhi"
	repoStateClean           = "clean"
	commitHashLength         = 7
	infraRepoUrl             = "https://skia.googlesource.com/buildbot.git"
	k8sConfigRepoUrl         = "https://skia.googlesource.com/k8s-config.git"
	measurementImageLatency  = "cd_image_build_latency_s"

	// overlapDuration indicates how long to extend the time range past the time
	// at which we last finished ingesting data.  This allows us to revisit
	// commits for which the CD pipeline may not have finished.  Its value
	// should be longer than we ever expect the CD pipeline to take.
	overlapDuration = 6 * time.Hour
)

var (
	// beginningOfTime is considered to be the earliest time from which we'll
	// ingest data. The CD pipeline didn't exist before this point, so there's
	// no reason to load earlier data.
	beginningOfTime = time.Date(2022, time.June, 13, 0, 0, 0, 0, time.UTC)

	timePeriods = []time.Duration{24 * time.Hour, 7 * 24 * time.Hour}

	repoUrls = []string{infraRepoUrl, k8sConfigRepoUrl}
)

// cycle performs one cycle of metrics ingestion.
func cycle(ctx context.Context, imageName string, repos repograph.Map, edb events.EventDB, em *events.EventMetrics, lastFinished, now time.Time, ts oauth2.TokenSource) ([]metrics2.Int64Metric, error) {
	sklog.Infof("CD metrics for %s", imageName)
	// Setup.
	infraRepo := repos[infraRepoUrl]
	gcrClient := gcr.NewClient(ts, containerRegistryProject, imageName)
	resp, err := gcrClient.Tags(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to retrieve Docker image data")
	}
	sklog.Infof("  Found %d docker images for %s", len(resp.Manifest), imageName)

	// Create a mapping of shortened hash to commit details for easy retrieval.
	commits, err := infraRepo.GetCommitsNewerThan(lastFinished.Add(-overlapDuration))
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to retrieve commits")
	}
	commitMap := make(map[string]*vcsinfo.LongCommit, len(commits))
	for _, c := range commits {
		commitMap[c.Hash[:commitHashLength]] = c
	}
	sklog.Infof("  Found %d commits since %s.", len(commitMap), lastFinished.Add(-overlapDuration))

	// Go through the Docker images we've uploaded and map them to the commits
	// from which they were built.
	commitToDockerImageDigest := make(map[*vcsinfo.LongCommit]string, len(commitMap))
	commitToDockerImageTime := make(map[*vcsinfo.LongCommit]time.Time, len(commitMap))
	for digest, manifest := range resp.Manifest {
		for _, tag := range manifest.Tags {
			m := gcr.DockerTagRegex.FindStringSubmatch(tag)
			if len(m) != 5 {
				continue
			}
			timeStr := m[1]
			user := m[2]
			hash := m[3]
			state := m[4]

			// We only care about clean builds generated by our CD system.
			if user != louhiUser || state != repoStateClean {
				continue
			}

			// We only care about commits in the specified time range.
			commit, ok := commitMap[hash]
			if !ok {
				continue
			}

			// Record the digest and creation time of the Docker image for the
			// current commit.  Use the timestamp from the tag instead of the
			// timestamp on the Docker image itself, because some commits may
			// generate the same digest as previous a commit, and in those cases
			// using the timestamp of the image would paint a misleading picture
			// of the latency between commit time and Docker image creation.
			dockerImageTime, err := time.Parse("2006-01-02T15_04_05Z", timeStr)
			if err != nil {
				sklog.Errorf("Invalid timestamp in tag %q: %s", tag, err)
				continue
			}
			if existingTs, ok := commitToDockerImageTime[commit]; !ok || dockerImageTime.Before(existingTs) {
				commitToDockerImageDigest[commit] = digest
				commitToDockerImageTime[commit] = dockerImageTime
			}
		}
	}

	// Create an EventDB event for each commit. Produce metrics for individual
	// commits.
	newMetrics := make([]metrics2.Int64Metric, 0, len(commits))
	for _, commit := range commits {
		// Create the event and insert it into the DB.
		dockerImageTime := time.Now()
		ts, haveDockerImage := commitToDockerImageTime[commit]
		if haveDockerImage {
			dockerImageTime = ts
		}

		// Log the commit-to-docker-image latency for this commit.
		logStr := fmt.Sprintf("%s: %s", commit.Hash[:7], dockerImageTime.Sub(commit.Timestamp))
		if haveDockerImage {
			logStr += fmt.Sprintf(" (%s)", commitToDockerImageDigest[commit])
		}
		sklog.Info(logStr)

		data := Event{
			CommitHash:        commit.Hash,
			CommitTime:        commit.Timestamp,
			DockerImageDigest: commitToDockerImageDigest[commit],
			DockerImageTime:   commitToDockerImageTime[commit],
			K8sConfigHash:     "",          // TODO
			K8sConfigTime:     time.Time{}, // TODO
		}
		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(&data); err != nil {
			return nil, skerr.Wrapf(err, "failed to encode event")
		}
		ev := &events.Event{
			Stream:    fmtStream(infraRepoUrl, imageName),
			Timestamp: commit.Timestamp,
			Data:      buf.Bytes(),
		}
		if err := edb.Insert(ev); err != nil {
			return nil, skerr.Wrapf(err, "failed to insert event")
		}

		// Add other metrics.

		// Latency between commit landing and docker image built. We have the
		// EventDB to give us aggregate metrics, so we'll just use this gauge
		// for alerts in the case where the latency for a given commit is too
		// high. Therefore, we don't need this if we've already built the Docker
		// image.
		if !haveDockerImage {
			tags := map[string]string{
				"commit": commit.Hash,
				"image":  imageName,
			}
			m := metrics2.GetInt64Metric(measurementImageLatency, tags)
			m.Update(int64(dockerImageTime.Sub(data.CommitTime).Seconds()))
			newMetrics = append(newMetrics, m)
		}
	}

	return newMetrics, nil
}

// Event is an entry in the EventDB which details the time taken
type Event struct {
	CommitHash        string
	CommitTime        time.Time
	DockerImageDigest string
	DockerImageTime   time.Time
	K8sConfigHash     string
	K8sConfigTime     time.Time
}

// addAggregateMetrics adds aggregate metrics to the stream.
func addAggregateMetrics(s *events.EventStream, imageName string, period time.Duration) error {
	if err := s.AggregateMetric(map[string]string{
		"image":  imageName,
		"metric": "image_build_latency",
	}, period, func(ev []*events.Event) (float64, error) {
		totalLatency := float64(0)
		for _, e := range ev {
			var data Event
			if err := gob.NewDecoder(bytes.NewReader(e.Data)).Decode(&data); err != nil {
				return 0.0, skerr.Wrap(err)
			}
			totalLatency += float64(data.DockerImageTime.Sub(data.CommitTime))
		}
		return totalLatency / float64(len(ev)), nil
	}); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// getLastIngestionTs returns the timestamp of the last commit for which we
// successfully ingested events.
func getLastIngestionTs(edb events.EventDB, imageName string) (time.Time, error) {
	timeEnd := time.Now()
	window := time.Hour
	for {
		timeStart := timeEnd.Add(-window)
		var latest time.Time
		ev, err := edb.Range(fmtStream(infraRepoUrl, imageName), timeStart, timeEnd)
		if err != nil {
			return beginningOfTime, err
		}
		if len(ev) > 0 {
			ts := ev[len(ev)-1].Timestamp
			if ts.After(latest) {
				latest = ts
			}
		}
		if !util.TimeIsZero(latest) {
			return latest, nil
		}
		if timeStart.Before(beginningOfTime) {
			return beginningOfTime, nil
		}
		window *= 2
		timeEnd = timeStart
	}
}

// Start initiates the metrics data generation for Docker images.
func Start(ctx context.Context, imageNames []string, btConf *bt_gitstore.BTConfig, ts oauth2.TokenSource) error {
	// Set up event metrics.
	edb, err := events.NewBTEventDB(ctx, btConf.ProjectID, btConf.InstanceID, ts)
	if err != nil {
		return skerr.Wrapf(err, "Failed to create EventDB")
	}
	em, err := events.NewEventMetrics(edb, "cd_pipeline")
	if err != nil {
		return skerr.Wrapf(err, "failed to create EventMetrics")
	}
	repos, err := bt_gitstore.NewBTGitStoreMap(ctx, repoUrls, btConf)
	if err != nil {
		sklog.Fatal(err)
	}

	// Find the timestamp of the last-ingested commit.
	lastFinished := time.Now()
	for _, imageName := range imageNames {
		s := em.GetEventStream(fmtStream(infraRepoUrl, imageName))
		for _, p := range timePeriods {
			if err := addAggregateMetrics(s, infraRepoUrl, p); err != nil {
				return skerr.Wrapf(err, "failed to add metric")
			}
		}
		lastFinishedForImage, err := getLastIngestionTs(edb, imageName)
		if err != nil {
			return skerr.Wrapf(err, "failed to get timestamp of last successful ingestion")
		}
		if lastFinishedForImage.Before(lastFinished) {
			lastFinished = lastFinishedForImage
		}
	}

	// Start ingesting data.
	lv := metrics2.NewLiveness("last_successful_cd_pipeline_metrics")
	oldMetrics := map[metrics2.Int64Metric]struct{}{}
	go util.RepeatCtx(ctx, 2*time.Minute, func(ctx context.Context) {
		sklog.Infof("CD metrics loop start.")

		// These repos aren't shared with the rest of Datahopper, so we need to
		// update them.
		if err := repos.Update(ctx); err != nil {
			sklog.Errorf("Failed to update repos: %s", err)
			return
		}

		now := time.Now()
		anyFailed := false
		newMetrics := map[metrics2.Int64Metric]struct{}{}
		for _, imageName := range imageNames {
			m, err := cycle(ctx, imageName, repos, edb, em, lastFinished, now, ts)
			if err != nil {
				sklog.Errorf("Failed to obtain CD pipeline metrics: %s", err)
				anyFailed = true
			}
			for _, metric := range m {
				newMetrics[metric] = struct{}{}
			}
		}
		if !anyFailed {
			lastFinished = now
			lv.Reset()

			// Delete any metrics which we haven't generated again.
			for m := range oldMetrics {
				if _, ok := newMetrics[m]; !ok {
					if err := m.Delete(); err != nil {
						sklog.Warningf("Failed to delete metric: %s", err)
						// If we failed to delete the metric, add it to the
						// "new" metrics list, so that we'll carry it over and
						// try again on the next cycle.
						newMetrics[m] = struct{}{}
					}
				}
			}
			oldMetrics = newMetrics
		}
		sklog.Infof("CD metrics loop end.")
	})
	em.Start(ctx)
	return nil
}

// fmtStream returns the name of an event stream given a repo URL and image name.
func fmtStream(repo, imageName string) string {
	split := strings.Split(repo, "/")
	repoName := strings.TrimSuffix(split[len(split)-1], ".git")
	return fmt.Sprintf("cd-commits-%s", repoName)
}
