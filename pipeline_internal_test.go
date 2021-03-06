package ep

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPipeline_creationSingleRunner(t *testing.T) {
	runner := Pipeline(Gather())
	_, isPipe := runner.(pipeline)
	require.False(t, isPipe)

	passThrough := Pipeline(PassThrough())
	_, isPipe = passThrough.(pipeline)
	require.False(t, isPipe)

	projectWithPipeline := Pipeline(Project(Pipeline(Scatter(), Scatter()), Gather()))
	_, isPipe = projectWithPipeline.(pipeline)
	require.False(t, isPipe)
}

func TestPipeline_creationFlat(t *testing.T) {
	runner := Pipeline(Gather(), Scatter())
	runner = Pipeline(runner, Pipeline(Scatter(), Pipeline(Scatter(), Scatter())))

	p, isPipe := runner.(pipeline)
	require.True(t, isPipe)
	require.Equal(t, 5, len(p))

	nestedPipeline := Pipeline(Pipeline(Scatter(), Scatter()))
	p, isPipe = nestedPipeline.(pipeline)
	require.True(t, isPipe)
	require.Equal(t, 2, len(p))
	_, isPipe = p[0].(pipeline)
	require.False(t, isPipe)
}

func TestPipeline_creationFlatSkipPassThrough(t *testing.T) {
	runner := Pipeline(Gather(), PassThrough())
	runner = Pipeline(runner, Pipeline(Scatter(), Pipeline(PassThrough(), Scatter())))

	p, isPipe := runner.(pipeline)
	require.True(t, isPipe)
	// 5 runners without 2 skipped passThrough
	require.Equal(t, 3, len(p))
}

func TestPipeline_creationPreserveProject(t *testing.T) {
	runner := Project(Gather(), Pipeline(Scatter(), Scatter()), Pipeline(Scatter(), Scatter()))
	runner = Pipeline(runner, Gather())

	p, isPipe := runner.(pipeline)
	require.True(t, isPipe)
	require.Equal(t, 2, len(p))
}

func TestPipeline_creationSingleRunnerAfterFlat(t *testing.T) {
	runner := Pipeline(PassThrough(), Gather(), PassThrough())
	_, isPipe := runner.(pipeline)
	require.False(t, isPipe)

	nestedPipeline := Pipeline(PassThrough(), Pipeline(Gather(), PassThrough()), PassThrough())
	_, isPipe = nestedPipeline.(pipeline)
	require.False(t, isPipe)

	onlyPassThrough := Pipeline(PassThrough(), PassThrough())
	_, isPipe = onlyPassThrough.(pipeline)
	require.False(t, isPipe)
	_, isPassThrough := onlyPassThrough.(*passThrough)
	require.True(t, isPassThrough)

	nestedPipelineWithOnlyPassThrough := Pipeline(
		PassThrough(),
		Pipeline(PassThrough(), PassThrough()),
		PassThrough(),
		Pipeline(PassThrough(), Pipeline(PassThrough(), PassThrough())),
	)
	_, isPipe = nestedPipelineWithOnlyPassThrough.(pipeline)
	require.False(t, isPipe)
	_, isPassThrough = onlyPassThrough.(*passThrough)
	require.True(t, isPassThrough)
}

func TestPipeline_creationComposable(t *testing.T) {
	t.Run("nestedPipeline", func(t *testing.T) {
		nestedPipeline := Pipeline(PassThrough(), Pipeline(&dummyRunner{}, PassThrough()), PassThrough())
		_, isCmp := nestedPipeline.(Composable)
		require.True(t, isCmp)
	})

	t.Run("nestedComposablePipeline", func(t *testing.T) {
		nestedComposablePipeline := Pipeline(
			Pipeline(&dummyRunner{}, &dummyRunner{}),
			Pipeline(&dummyRunner{}, &dummyRunner{}, passThroughSingleton),
		)

		c, isCmp := nestedComposablePipeline.(*compose)
		require.True(t, isCmp)
		require.Equal(t, 2, len(c.Cmps))
	})

	t.Run("pipelineWithProject", func(t *testing.T) {
		pipelineWithProject := Pipeline(&dummyRunner{}, Compose(nil, ComposeProject(passThroughSingleton, &dummyRunner{})))
		c, isCmp := pipelineWithProject.(*compose)
		require.True(t, isCmp)
		require.Equal(t, 2, len(c.Cmps))
		p, isCmp := isComposeProject(c.Cmps[1])
		require.True(t, isCmp)
		require.Equal(t, 2, len(p))
	})
}

// Measures the number of datasets (ops) per second going through a pipeline
// composed of 3 passThrough-s. At the time of writing, it was evident that
// performance is not impacted by the size of the datasets (sensible, given
// the implementation details).
func BenchmarkPipeline(b *testing.B) {
	run := func(r Runner) func(b *testing.B) {
		return func(b *testing.B) {
			var err error

			data := NewDataset(dummy.Data(-1))
			inp := make(chan Dataset)
			out := make(chan Dataset)

			go Run(context.Background(), r, inp, out, nil, &err)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// run a single dataset through the pipeline
				inp <- data
				<-out
			}
			close(inp)
			drain(out)

			require.NoError(b, err)
		}
	}
	b.Run("runner", func(b *testing.B) {
		r := pipeline{PassThrough(), PassThrough(), PassThrough()}
		run(r)(b)
	})
	b.Run("compose", func(b *testing.B) {
		r := &compose{Cmps: []Composable{passThroughSingleton, passThroughSingleton, passThroughSingleton}}
		run(r)(b)
	})

}
