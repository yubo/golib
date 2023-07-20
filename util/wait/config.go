package wait

import (
	"github.com/yubo/golib/api"
)

type BackoffConfig struct {
	// The initial duration.
	Duration api.Duration `json:"duration"`
	// Duration is multiplied by factor each iteration, if factor is not zero
	// and the limits imposed by Steps and Cap have not been reached.
	// Should not be negative.
	// The jitter does not contribute to the updates to the duration parameter.
	Factor float64 `json:"factor"`
	// The sleep at each iteration is the duration plus an additional
	// amount chosen uniformly at random from the interval between
	// zero and `jitter*duration`.
	Jitter float64 `json:"jitter"`
	// The remaining number of iterations in which the duration
	// parameter may change (but progress can be stopped earlier by
	// hitting the cap). If not positive, the duration is not
	// changed. Used for exponential backoff in combination with
	// Factor and Cap.
	Steps int `json:"steps"`
	// A limit on revised values of the duration parameter. If a
	// multiplication by the factor parameter would make the duration
	// exceed the cap then the duration is set to the cap and the
	// steps parameter is set to zero.
	Cap api.Duration `json:"cap"`
}

func (c *BackoffConfig) Backoff() *Backoff {
	if c == nil {
		return nil
	}
	return &Backoff{
		Duration: c.Duration.Duration,
		Factor:   c.Factor,
		Jitter:   c.Jitter,
		Steps:    c.Steps,
		Cap:      c.Cap.Duration,
	}
}

func (b *Backoff) BackoffConfig() *BackoffConfig {
	if b == nil {
		return nil
	}
	return &BackoffConfig{
		Duration: api.Duration{Duration: b.Duration},
		Factor:   b.Factor,
		Jitter:   b.Jitter,
		Steps:    b.Steps,
		Cap:      api.Duration{Duration: b.Cap},
	}
}
