package read_values

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/perf/go/perfresults"
)

var testData = map[string]perfresults.PerfResults{
	"rendering.desktop": {
		Histograms: map[string]perfresults.Histogram{
			"thread_total_rendering_cpu_time_per_frame": {
				Name:         "thread_total_rendering_cpu_time_per_frame",
				Unit:         "unitless_smallerIsBetter",
				SampleValues: []float64{12.9322},
			},
			"tasks_per_frame_browser": {
				Name:         "tasks_per_frame_browser",
				Unit:         "unitless_smallerIsBetter",
				SampleValues: []float64{0.3917, 0.34},
			},
			"empty_samples": {
				Name:         "empty_samples",
				Unit:         "unitless_smallerIsBetter",
				SampleValues: nil,
			},
			"Compositing.Display.DrawToSwapUs": {
				Name:         "Compositing.Display.DrawToSwapUs",
				Unit:         "unitless_smallerIsBetter",
				SampleValues: []float64{169.9406, 169.9406, 206.3219, 654.8641},
			},
		},
	},
	// the possibility of two benchmarks in one results dataset
	// is very unlikely. We test for it out of an abundance of caution.
	"rendering.desktop.notracing": {
		Histograms: map[string]perfresults.Histogram{
			"thread_total_rendering_cpu_time_per_frame": {
				Name:         "thread_total_rendering_cpu_time_per_frame",
				Unit:         "unitless_smallerIsBetter",
				SampleValues: []float64{0.78},
			},
		},
	},
}

type mockedProvider struct {
}

func (mp mockedProvider) Fetch(ctx context.Context, cas *swarming.SwarmingRpcsCASReference) (map[string]perfresults.PerfResults, error) {
	return testData, nil
}

func TestReadChart_ReadSampleValues(t *testing.T) {
	c := perfCASClient{
		provider: mockedProvider{},
	}
	test := func(name, benchmark, chart string, expected ...float64) {
		t.Run(name, func(t *testing.T) {
			values, err := c.ReadValuesByChart(context.Background(), benchmark, chart, []*swarming.SwarmingRpcsCASReference{{}}, "")
			assert.NoError(t, err)
			assert.EqualValues(t, expected, values)
		})
	}

	test("basic chart test", "rendering.desktop", "thread_total_rendering_cpu_time_per_frame", 12.9322)
	test("multiple value test", "rendering.desktop", "Compositing.Display.DrawToSwapUs", 169.9406, 169.9406, 206.3219, 654.8641)
	test("null case", "fake benchmark", "fake chart")
}

func TestReadChart_ReadAggregatedValues(t *testing.T) {
	c := perfCASClient{
		provider: mockedProvider{},
	}
	test := func(name, benchmark, chart, agg string, expected ...float64) {
		t.Run(name, func(t *testing.T) {
			// Load three same CAS
			values, err := c.ReadValuesByChart(context.Background(), benchmark, chart, []*swarming.SwarmingRpcsCASReference{{}, {}, {}}, agg)
			assert.NoError(t, err)
			assert.EqualValues(t, expected, values)
		})
	}

	// compute min for each sample set which only contains one value
	test("single value chart - min", "rendering.desktop", "thread_total_rendering_cpu_time_per_frame", "min", 12.9322, 12.9322, 12.9322)

	// compute mean for each sample set
	test("multiple values chart - mean", "rendering.desktop", "Compositing.Display.DrawToSwapUs", "mean", 300.2668, 300.2668, 300.2668)

	// load the same samples three times
	test("multiple values chart - all samples", "rendering.desktop", "tasks_per_frame_browser", "", 0.3917, 0.34, 0.3917, 0.34, 0.3917, 0.34)

	test("multiple empty values chart - min", "rendering.desktop", "empty_samples", "min")

	// compute max on the empty set
	test("null case", "fake benchmark", "fake chart", "max")
}

func TestAggData_NonBlankData_AggData(t *testing.T) {
	testData := perfresults.Histogram{
		SampleValues: []float64{8, 2, -9, 15, 4},
	}
	test := func(name string, expected float64) {
		t.Run(name, func(t *testing.T) {
			ans, ok := aggregationMapping[name]
			assert.True(t, ok)
			assert.InDelta(t, expected, ans(testData), 1e-6)
		})
	}

	test("count", 5.0)
	test("mean", 4.0)
	test("max", 15.0)
	test("min", -9.0)
	test("std", 8.803408)
	test("sum", 20.0)
}
