package templated

import (
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"log/slog"
	"slices"
	"sync"
	"text/template"

	"github.com/lxc/incus-compose/iclient"
	"github.com/lxc/incus-compose/ievent/iutil"
)

type deployResult struct {
	targetKey string
	hash      []byte
}

type deployState struct {
	lastDeployed map[string][]byte
	cancels      map[string]context.CancelFunc
	done         chan deployResult
	wg           sync.WaitGroup
}

func newDeployState() *deployState {
	return &deployState{
		lastDeployed: make(map[string][]byte),
		cancels:      make(map[string]context.CancelFunc),
		done:         make(chan deployResult, 32),
	}
}

func cancelAll(cancels map[string]context.CancelFunc) {
	for _, cancel := range cancels {
		cancel()
	}
}

// data collects the instances carrying the target's label, sorted so the render is stable.
// The plugin only holds running instances, so that is not checked again here.
func data(target Target, instances []*iutil.Event) Data {
	out := Data{Label: target.Label, Flags: target.Flags, Instances: []Instance{}}

	for _, ev := range instances {
		i := newInstance(ev, target.Label)
		if !HasLabel(i.Config, target.Label) {
			continue
		}

		out.Instances = append(out.Instances, i)
	}

	slices.SortFunc(out.Instances, func(a, b Instance) int {
		return cmp.Or(cmp.Compare(a.Project, b.Project), cmp.Compare(a.Name, b.Name))
	})

	return out
}

// renderTarget renders d, leaving out the instances that fail to render on their own, so one bad
// instance does not hold back every other change to the file. It returns how many it left out.
func renderTarget(logger *slog.Logger, target Target, d Data, funcs template.FuncMap) ([]byte, int, error) {
	content, err := render(target, d, funcs)
	if err == nil {
		return content, 0, nil
	}

	kept := []Instance{}

	for _, inst := range d.Instances {
		_, instErr := render(target, Data{Label: d.Label, Flags: d.Flags, Instances: []Instance{inst}}, funcs)
		if instErr != nil {
			logger.Error("rendering; leaving the instance out", "target", target.String(), "instance", inst.Project+"/"+inst.Name, "err", instErr)

			continue
		}

		kept = append(kept, inst)
	}

	// Every instance failing alone points at the target rather than at them, and rendering
	// none of them would empty the live file.
	if len(kept) == 0 || len(kept) == len(d.Instances) {
		return nil, 0, err
	}

	leftOut := len(d.Instances) - len(kept)
	d.Instances = kept

	content, err = render(target, d, funcs)
	if err != nil {
		return nil, 0, err
	}

	return content, leftOut, nil
}

func reconcileDeployments(
	ctx context.Context,
	state *deployState,
	logger *slog.Logger,
	conn *iclient.Connection,
	targets []Target,
	funcs template.FuncMap,
	instances []*iutil.Event,
	metrics bool,
) {
	for _, target := range targets {
		targetKey := target.Key()

		content, leftOut, err := renderTarget(logger, target, data(target, instances), funcs)

		if metrics {
			failed := 0.0
			if err != nil {
				failed = 1
			}

			renderFailedGauge.WithLabelValues(targetKey).Set(failed)
			renderLeftOutGauge.WithLabelValues(targetKey).Set(float64(leftOut))
		}

		if err != nil {
			logger.Error("rendering", "target", target.String(), "err", err)

			continue
		}

		sum := sha256.Sum256(content)
		if bytes.Equal(state.lastDeployed[targetKey], sum[:]) {
			continue
		}

		if target.Instance != "" && conn == nil {
			continue
		}

		cancel, ok := state.cancels[targetKey]
		if ok {
			cancel()
		}

		deployCtx, cancel := context.WithCancel(ctx)
		state.cancels[targetKey] = cancel

		state.wg.Go(func() {
			var err error
			if target.Instance != "" {
				err = deploy(deployCtx, logger, conn, target, content)
			} else {
				err = deployOS(deployCtx, logger, target, content)
			}

			if err != nil {
				// Canceled by a newer reconcile, which deploys its own content, or by shutdown; neither is a failure.
				if deployCtx.Err() != nil {
					logger.Debug("Deployment superseded", "target", target.String(), "err", err)

					return
				}

				logger.Error("deploying", "target", target.String(), "err", err)

				return
			}

			select {
			case state.done <- deployResult{targetKey: targetKey, hash: bytes.Clone(sum[:])}:
			case <-deployCtx.Done():
			}
		})
	}
}
