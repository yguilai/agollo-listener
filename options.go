package apollox

import "time"

type (
	options struct {
		waitTimeout time.Duration
		namespaces  []string
		replaceEnv  bool
	}

	Option func(*options)
)

// WithNamespaces gives some namespaces that your listener will focus on those,
// if namespaces is not exists, the listener will focus on default namespace (application)
func WithNamespaces(namespaces ...string) Option {
	return func(o *options) {
		o.namespaces = append(o.namespaces, namespaces...)
	}
}

// WithExtraNamespace gives some namespaces along with the default namespace
func WithExtraNamespace(extraNamespaces ...string) Option {
	return func(o *options) {
		o.namespaces = append([]string{defaultNamespace}, extraNamespaces...)
	}
}

// WithWaitTimeout gives a duration, used in ConfigListener.Poll
func WithWaitTimeout(timeout time.Duration) Option {
	return func(o *options) {
		o.waitTimeout = timeout
	}
}

// WithReplaceEnv enables replace env placeholder
func WithReplaceEnv() Option {
	return func(o *options) {
		o.replaceEnv = true
	}
}
