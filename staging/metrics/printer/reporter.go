package printer

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/uber-go/tally"
)

// Initialize max vars in init function to avoid lint error.
var (
	maxInt64   int64
	maxFloat64 float64
)

const (
	// ServiceTag is the name of the M3 service tag.
	ServiceTag = "service"
	// EnvTag is the name of the M3 env tag.
	EnvTag = "env"
	// HostTag is the name of the M3 host tag.
	HostTag = "host"
	// DefaultMaxQueueSize is the default M3 reporter queue size.
	DefaultMaxQueueSize = 4096
	// DefaultMaxPacketSize is the default M3 reporter max packet size.
	DefaultMaxPacketSize = int32(1440)
	// DefaultHistogramBucketIDName is the default histogram bucket ID tag name
	DefaultHistogramBucketIDName = "bucketid"
	// DefaultHistogramBucketName is the default histogram bucket name tag name
	DefaultHistogramBucketName = "bucket"
	// DefaultHistogramBucketTagPrecision is the default
	// precision to use when formatting the metric tag
	// with the histogram bucket bound values.
	DefaultHistogramBucketTagPrecision = uint(6)

	emitMetricBatchOverhead    = 19
	minMetricBucketIDTagLength = 4
)

func init() {
	maxInt64 = math.MaxInt64
	maxFloat64 = math.MaxFloat64
}

type metricType int

const (
	counterType metricType = iota + 1
	timerType
	gaugeType
)

// reporter is a metrics backend that reports metrics to a local or
// remote M3 collector, metrics are batched together and emitted
// via either thrift compact or binary protocol in batch UDP packets.
type reporter struct {
	// client          *m3thrift.M3Client
	// curBatch        *m3thrift.MetricBatch
	// curBatchLock    sync.Mutex
	// calc            *customtransport.TCalcTransport
	// calcProto       thrift.TProtocol
	// calcLock        sync.Mutex
	// commonTags      map[*m3thrift.MetricTag]bool
	// freeBytes       int32
	// processors      sync.WaitGroup
	resourcePool    *resourcePool
	bucketIDTagName string
	bucketTagName   string
	bucketValFmt    string

	// status reporterStatus
	// metCh  chan sizedMetric
}

// Options is a set of options for the M3 reporter.
type Options struct {
	// HostPorts                   []string
	// Service                     string
	// Env                         string
	// CommonTags                  map[string]string
	// IncludeHost                 bool
	// Protocol                    Protocol
	// MaxQueueSize                int
	// MaxPacketSizeBytes          int32
	HistogramBucketIDName       string
	HistogramBucketName         string
	HistogramBucketTagPrecision uint
}

// NewReporter creates a new M3 reporter.
func NewReporter(opts Options) (tally.CachedStatsReporter, error) {
	// if opts.MaxQueueSize <= 0 {
	// 	opts.MaxQueueSize = DefaultMaxQueueSize
	// }
	// if opts.MaxPacketSizeBytes <= 0 {
	// 	opts.MaxPacketSizeBytes = DefaultMaxPacketSize
	// }
	if opts.HistogramBucketIDName == "" {
		opts.HistogramBucketIDName = DefaultHistogramBucketIDName
	}
	if opts.HistogramBucketName == "" {
		opts.HistogramBucketName = DefaultHistogramBucketName
	}
	if opts.HistogramBucketTagPrecision == 0 {
		opts.HistogramBucketTagPrecision = DefaultHistogramBucketTagPrecision
	}

	// Create M3 thrift client
	// var trans thrift.TTransport
	// var err error
	// if len(opts.HostPorts) == 0 {
	// 	err = errNoHostPorts
	// } else if len(opts.HostPorts) == 1 {
	// 	trans, err = thriftudp.NewTUDPClientTransport(opts.HostPorts[0], "")
	// } else {
	// 	trans, err = thriftudp.NewTMultiUDPClientTransport(opts.HostPorts, "")
	// }
	// if err != nil {
	// 	return nil, err
	// }

	// var protocolFactory thrift.TProtocolFactory
	// if opts.Protocol == Compact {
	// 	protocolFactory = thrift.NewTCompactProtocolFactory()
	// } else {
	// 	protocolFactory = thrift.NewTBinaryProtocolFactoryDefault()
	// }

	// client := m3thrift.NewM3ClientFactory(trans, protocolFactory)
	resourcePool := newResourcePool()

	// // Create common tags
	// tags := resourcePool.getTagList()
	// for k, v := range opts.CommonTags {
	// 	tags[createTag(resourcePool, k, v)] = true
	// }
	// if opts.CommonTags[ServiceTag] == "" {
	// 	if opts.Service == "" {
	// 		return nil, fmt.Errorf("%s common tag is required", ServiceTag)
	// 	}
	// 	tags[createTag(resourcePool, ServiceTag, opts.Service)] = true
	// }
	// if opts.CommonTags[EnvTag] == "" {
	// 	if opts.Env == "" {
	// 		return nil, fmt.Errorf("%s common tag is required", EnvTag)
	// 	}
	// 	tags[createTag(resourcePool, EnvTag, opts.Env)] = true
	// }
	// if opts.IncludeHost {
	// 	if opts.CommonTags[HostTag] == "" {
	// 		hostname, err := os.Hostname()
	// 		if err != nil {
	// 			return nil, fmt.Errorf("error resolving host tag: %v", err)
	// 		}
	// 		tags[createTag(resourcePool, HostTag, hostname)] = true
	// 	}
	// }

	// Calculate size of common tags
	// batch := resourcePool.getBatch()
	// batch.CommonTags = tags
	// batch.Metrics = []*m3thrift.Metric{}
	// proto := resourcePool.getProto()
	// batch.Write(proto)
	// calc := proto.Transport().(*customtransport.TCalcTransport)
	// numOverheadBytes := emitMetricBatchOverhead + calc.GetCount()
	// calc.ResetCount()

	// freeBytes := opts.MaxPacketSizeBytes - numOverheadBytes
	// if freeBytes <= 0 {
	// 	return nil, errCommonTagSize
	// }

	r := &reporter{
		// client:          client,
		// curBatch:        batch,
		// calc:            calc,
		// calcProto:       proto,
		// commonTags:      tags,
		// freeBytes:       freeBytes,
		resourcePool:    resourcePool,
		bucketIDTagName: opts.HistogramBucketIDName,
		bucketTagName:   opts.HistogramBucketName,
		bucketValFmt:    "%." + strconv.Itoa(int(opts.HistogramBucketTagPrecision)) + "f",
		// metCh:           make(chan sizedMetric, opts.MaxQueueSize),
	}

	// r.processors.Add(1)
	// go r.process()

	return r, nil
}

// AllocateCounter implements tally.CachedStatsReporter.
func (r *reporter) AllocateCounter(
	name string, tags map[string]string,
) tally.CachedCount {
	return r.allocateCounter(name, tags)
}

func (r *reporter) allocateCounter(
	name string, tags map[string]string,
) cachedMetric {
	counter := r.newMetric(name, tags, counterType)
	//size := r.calculateSize(counter)
	return cachedMetric{counter, r}
}

// AllocateGauge implements tally.CachedStatsReporter.
func (r *reporter) AllocateGauge(
	name string, tags map[string]string,
) tally.CachedGauge {
	gauge := r.newMetric(name, tags, gaugeType)
	// size := r.calculateSize(gauge)
	return cachedMetric{gauge, r}
}

// AllocateTimer implements tally.CachedStatsReporter.
func (r *reporter) AllocateTimer(
	name string, tags map[string]string,
) tally.CachedTimer {
	timer := r.newMetric(name, tags, timerType)
	// size := r.calculateSize(timer)
	return cachedMetric{timer, r}
}

// AllocateHistogram implements tally.CachedStatsReporter.
func (r *reporter) AllocateHistogram(
	name string,
	tags map[string]string,
	buckets tally.Buckets,
) tally.CachedHistogram {
	var (
		cachedValueBuckets    []cachedHistogramBucket
		cachedDurationBuckets []cachedHistogramBucket
	)
	bucketIDLen := len(strconv.Itoa(buckets.Len()))
	bucketIDLen = int(math.Max(float64(bucketIDLen),
		float64(minMetricBucketIDTagLength)))
	bucketIDLenStr := strconv.Itoa(bucketIDLen)
	bucketIDFmt := "%0" + bucketIDLenStr + "d"
	for i, pair := range tally.BucketPairs(buckets) {
		valueTags, durationTags := make(map[string]string), make(map[string]string)
		for k, v := range tags {
			valueTags[k], durationTags[k] = v, v
		}

		idTagValue := fmt.Sprintf(bucketIDFmt, i)

		valueTags[r.bucketIDTagName] = idTagValue
		valueTags[r.bucketTagName] = fmt.Sprintf("%s-%s",
			r.valueBucketString(pair.LowerBoundValue()),
			r.valueBucketString(pair.UpperBoundValue()))

		cachedValueBuckets = append(cachedValueBuckets,
			cachedHistogramBucket{pair.UpperBoundValue(),
				pair.UpperBoundDuration(),
				r.allocateCounter(name, valueTags)})

		durationTags[r.bucketIDTagName] = idTagValue
		durationTags[r.bucketTagName] = fmt.Sprintf("%s-%s",
			r.durationBucketString(pair.LowerBoundDuration()),
			r.durationBucketString(pair.UpperBoundDuration()))

		cachedDurationBuckets = append(cachedDurationBuckets,
			cachedHistogramBucket{pair.UpperBoundValue(),
				pair.UpperBoundDuration(),
				r.allocateCounter(name, durationTags)})
	}
	return cachedHistogram{r, name, tags, buckets,
		cachedValueBuckets, cachedDurationBuckets}
}

func (r *reporter) valueBucketString(v float64) string {
	if v == math.MaxFloat64 {
		return "infinity"
	}
	if v == -math.MaxFloat64 {
		return "-infinity"
	}
	return fmt.Sprintf(r.bucketValFmt, v)
}

func (r *reporter) durationBucketString(d time.Duration) string {
	if d == 0 {
		return "0"
	}
	if d == time.Duration(math.MaxInt64) {
		return "infinity"
	}
	if d == time.Duration(math.MinInt64) {
		return "-infinity"
	}
	return d.String()
}

func (r *reporter) newMetric(
	name string,
	tags map[string]string,
	t metricType,
) *Metric {
	var (
		m      = r.resourcePool.getMetric()
		metVal = r.resourcePool.getValue()
	)
	m.Name = name
	if tags != nil {
		metTags := r.resourcePool.getTagList()
		for k, v := range tags {
			val := v
			metTag := r.resourcePool.getTag()
			metTag.TagName = k
			metTag.TagValue = &val
			metTags[metTag] = true
		}
		m.Tags = metTags
	} else {
		m.Tags = nil
	}
	m.Timestamp = &maxInt64

	switch t {
	case counterType:
		c := r.resourcePool.getCount()
		c.I64Value = &maxInt64
		metVal.Count = c
	case gaugeType:
		g := r.resourcePool.getGauge()
		g.DValue = &maxFloat64
		metVal.Gauge = g
	case timerType:
		t := r.resourcePool.getTimer()
		t.I64Value = &maxInt64
		metVal.Timer = t
	}
	m.MetricValue = metVal

	return m
}

// func (r *reporter) calculateSize(m *m3thrift.Metric) int32 {
// 	r.calcLock.Lock()
// 	m.Write(r.calcProto)
// 	size := r.calc.GetCount()
// 	r.calc.ResetCount()
// 	r.calcLock.Unlock()
// 	return size
// }

// func (r *reporter) reportCopyMetric(
// 	m *m3thrift.Metric,
// 	size int32,
// 	t metricType,
// 	iValue int64,
// 	dValue float64,
// ) {
// 	copy := r.resourcePool.getMetric()
// 	copy.Name = m.Name
// 	copy.Tags = m.Tags
// 	timestampNano := time.Now().UnixNano()
// 	copy.Timestamp = &timestampNano
// 	copy.MetricValue = r.resourcePool.getValue()
//
// 	switch t {
// 	case counterType:
// 		c := r.resourcePool.getCount()
// 		c.I64Value = &iValue
// 		copy.MetricValue.Count = c
// 	case gaugeType:
// 		g := r.resourcePool.getGauge()
// 		g.DValue = &dValue
// 		copy.MetricValue.Gauge = g
// 	case timerType:
// 		t := r.resourcePool.getTimer()
// 		t.I64Value = &iValue
// 		copy.MetricValue.Timer = t
// 	}
//
// 	r.status.RLock()
// 	if !r.status.closed {
// 		select {
// 		case r.metCh <- sizedMetric{copy, size}:
// 		default:
// 		}
// 	}
// 	r.status.RUnlock()
// }

func (r *reporter) Flush() {}

func (r *reporter) Close() (err error) { return nil }

func (r *reporter) Capabilities() tally.Capabilities { return r }

func (r *reporter) Reporting() bool { return true }

func (r *reporter) Tagging() bool { return true }

func createTag(
	pool *resourcePool,
	tagName, tagValue string,
) *MetricTag {
	tag := pool.getTag()
	tag.TagName = tagName
	if tagValue != "" {
		tag.TagValue = &tagValue
	}

	return tag
}

type cachedMetric struct {
	metric   *Metric
	reporter *reporter
}

func (c cachedMetric) String() string {
	tags := []string{}
	for k, v := range c.metric.Tags {
		if v && k.TagValue != nil {
			tags = append(tags, fmt.Sprintf("%s=\"%s\"", k.TagName, *k.TagValue))
		}

	}
	if len(tags) > 0 {
		return fmt.Sprintf("%s{%s}", c.metric.Name, strings.Join(tags, ","))
	}
	return fmt.Sprintf("%s", c.metric.Name)
}

func (c cachedMetric) ReportCount(value int64) {
	fmt.Printf("C %s %d\n", c, value)
	//c.reporter.reportCopyMetric(c.metric, c.size, counterType, value, 0)
}

func (c cachedMetric) ReportGauge(value float64) {
	fmt.Printf("G %s %f\n", c, value)
	// c.reporter.reportCopyMetric(c.metric, c.size, gaugeType, 0, value)
}

func (c cachedMetric) ReportTimer(interval time.Duration) {
	fmt.Printf("T %s %s\n", c, interval)
	// val := int64(interval)
	// c.reporter.reportCopyMetric(c.metric, c.size, timerType, val, 0)
}

func (c cachedMetric) ReportSamples(value int64) {
	fmt.Printf("S %s %d\n", c, value)
	// c.reporter.reportCopyMetric(c.metric, c.size, counterType, value, 0)
}

type noopMetric struct {
}

func (c noopMetric) ReportCount(value int64) {
}

func (c noopMetric) ReportGauge(value float64) {
}

func (c noopMetric) ReportTimer(interval time.Duration) {
}

func (c noopMetric) ReportSamples(value int64) {
}

type cachedHistogram struct {
	r                     *reporter
	name                  string
	tags                  map[string]string
	buckets               tally.Buckets
	cachedValueBuckets    []cachedHistogramBucket
	cachedDurationBuckets []cachedHistogramBucket
}

type cachedHistogramBucket struct {
	valueUpperBound    float64
	durationUpperBound time.Duration
	metric             cachedMetric
}

func (h cachedHistogram) ValueBucket(
	bucketLowerBound, bucketUpperBound float64,
) tally.CachedHistogramBucket {
	for _, b := range h.cachedValueBuckets {
		if b.valueUpperBound >= bucketUpperBound {
			return b.metric
		}
	}
	return noopMetric{}
}

func (h cachedHistogram) DurationBucket(
	bucketLowerBound, bucketUpperBound time.Duration,
) tally.CachedHistogramBucket {
	for _, b := range h.cachedDurationBuckets {
		if b.durationUpperBound >= bucketUpperBound {
			return b.metric
		}
	}
	return noopMetric{}
}

type sizedMetric struct {
	m    *Metric
	size int32
}
