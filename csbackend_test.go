package vulcan

import (
	"fmt"
	"github.com/mailgun/gocql"
	. "launchpad.net/gocheck"
	"os"
	"time"
)

type CassandraBackendSuite struct {
	timeProvider *FreezedTime
	backend      *CassandraBackend
	shouldSkip   bool
}

var _ = Suite(&CassandraBackendSuite{})

func (b *CassandraBackendSuite) dropKeyspace(config *CassandraConfig) error {
	// first session creates a keyspace
	cluster := config.newCluster()
	cluster.Keyspace = ""
	session := cluster.CreateSession()
	defer session.Close()
	return session.Query(
		fmt.Sprintf(`DROP KEYSPACE %s`, config.Keyspace)).Exec()
}

func (s *CassandraBackendSuite) GetConfig() *CassandraConfig {
	cassandraConfig := &CassandraConfig{
		Servers:       []string{"localhost"},
		Keyspace:      "vulcan_test",
		Consistency:   gocql.One,
		LaunchCleanup: false,
	}
	return cassandraConfig
}

func (s *CassandraBackendSuite) SetUpTest(c *C) {
	if os.Getenv("CASSANDRA") != "yes" {
		s.shouldSkip = true
		return
	}
	start := time.Date(2012, 3, 4, 5, 6, 7, 0, time.UTC)
	s.timeProvider = &FreezedTime{CurrentTime: start}

	config := s.GetConfig()
	config.applyDefaults()
	s.dropKeyspace(config)

	backend, err := NewCassandraBackend(s.GetConfig(), s.timeProvider)
	c.Assert(err, IsNil)
	s.backend = backend
}

func (s *CassandraBackendSuite) TestUtcNow(c *C) {
	if s.shouldSkip {
		c.Skip("Cassandra backend is not activated")
	}
	c.Assert(s.backend.utcNow(), Equals, s.timeProvider.CurrentTime)
}

// make sure the backend init is reentrable and does not alter existing data
func (s *CassandraBackendSuite) TestReentrable(c *C) {
	if s.shouldSkip {
		c.Skip("Cassandra backend is not activated")
	}

	_, err := NewCassandraBackend(s.GetConfig(), s.timeProvider)
	c.Assert(err, IsNil)

	_, err = NewCassandraBackend(s.GetConfig(), s.timeProvider)
	c.Assert(err, IsNil)
}

func (s *CassandraBackendSuite) TestBackendGetSet(c *C) {
	if s.shouldSkip {
		c.Skip("Cassandra backend is not activated")
	}

	counter, err := s.backend.getStats("key1", &Rate{Increment: 1, Value: 1, Period: time.Second})
	c.Assert(err, IsNil)
	c.Assert(counter, Equals, int64(0))

	err = s.backend.updateStats("key1", &Rate{Increment: 2, Value: 1, Period: time.Second})
	c.Assert(err, IsNil)

	counter, err = s.backend.getStats("key1", &Rate{Increment: 2, Value: 1, Period: time.Second})
	c.Assert(err, IsNil)
	c.Assert(counter, Equals, int64(2))
}

func (s *CassandraBackendSuite) TestBackendCleanup(c *C) {

	if s.shouldSkip {
		c.Skip("Cassandra backend is not activated")
	}

	err := s.backend.updateStats("key1", &Rate{Increment: 2, Value: 1, Period: time.Second})
	c.Assert(err, IsNil)
	s.backend.cleanup()
}
