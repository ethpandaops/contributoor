package ethereum

//// BeaconWrapper wraps ethcore beacon with contributoor functionality
//type BeaconWrapper struct {
//	*ethcore.BeaconNode // Embed ethcore beacon
//
//	log        logrus.FieldLogger
//	traceID    string
//	clockDrift clockdrift.ClockDrift
//	sinks      []sinks.ContributoorSink
//	cache      *events.DuplicateCache
//	summary    *events.Summary
//	metrics    *events.Metrics
//}
//
//// NewBeaconWrapper creates a wrapped beacon node
//func NewBeaconWrapper(
//	log logrus.FieldLogger,
//	traceID string,
//	config *Config,
//	sinks []sinks.ContributoorSink,
//	clockDrift clockdrift.ClockDrift,
//	cache *events.DuplicateCache,
//	summary *events.Summary,
//	metrics *events.Metrics,
//	opt *Options,
//) (*BeaconWrapper, error) {
