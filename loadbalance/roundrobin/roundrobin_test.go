package roundrobin

import (
	. "github.com/mailgun/vulcan/route"
	. "launchpad.net/gocheck"
	"testing"
)

func Test(t *testing.T) { TestingT(t) }

type RoundRobinSuite struct {
}

var _ = Suite(&RoundRobinSuite{})

func (s *RoundRobinSuite) SetUpSuite(c *C) {
}

func (s *RoundRobinSuite) TestNoUpstreams(c *C) {
	r := NewRoundRobin(&MatchAll{Group: "*"})
	_, err := r.NextUpstream(nil)
	c.Assert(err, NotNil)
}
