package command

import (
	"encoding/json"
	. "launchpad.net/gocheck"
	"net/http"
)

type HeadersSuite struct{}

var _ = Suite(&HeadersSuite{})

func (s *HeadersSuite) TestHeadersParsing(c *C) {
	rates := []struct {
		Headers http.Header
		Parse   string
	}{
		{
			Headers: http.Header{
				"A": []string{"b"},
			},
			Parse: `{"a": "b"}`,
		},
		{
			Headers: http.Header{
				"A": []string{"b", "c"},
			},
			Parse: `{"a": ["b", "c"]}`,
		},
		{
			Headers: http.Header{
				"A": []string{"b", "c"},
				"B": []string{"z"},
			},
			Parse: `{"a": ["b", "c"], "b": "z"}`,
		},
	}
	for _, u := range rates {
		var value interface{}
		err := json.Unmarshal([]byte(u.Parse), &value)
		c.Assert(err, IsNil)
		parsed, err := NewHeadersFromObj(value)
		c.Assert(err, IsNil)
		c.Assert(u.Headers, DeepEquals, parsed)
	}
}

func (s *HeadersSuite) TestHeadersParsingFailure(c *C) {
	rates := []struct {
		Parse string
	}{
		{
			Parse: `{"a": -1}`,
		},
		{
			Parse: `[]`,
		},
		{
			Parse: `"type"`,
		},
	}
	for _, u := range rates {
		var value interface{}
		err := json.Unmarshal([]byte(u.Parse), &value)
		c.Assert(err, IsNil)
		_, err = NewHeadersFromObj(value)
		c.Assert(err, Not(IsNil))
	}
}

func (s *HeadersSuite) TestAddRemoveHeaders(c *C) {
	rates := []struct {
		AddHeaders    http.Header
		RemoveHeaders []string
		Parse         string
	}{
		{
			AddHeaders: http.Header{
				"A": []string{"b"},
			},
			RemoveHeaders: []string{"F"},
			Parse:         `{"add-headers": {"a": "b"}, "remove-headers": ["F"]}`,
		},
		{
			AddHeaders: http.Header{
				"A": []string{"b"},
			},
			RemoveHeaders: nil,
			Parse:         `{"add-headers": {"a": "b"}}`,
		},
		{
			RemoveHeaders: []string{"A"},
			AddHeaders:    nil,
			Parse:         `{"remove-headers": ["A"]}`,
		},
		{
			RemoveHeaders: nil,
			AddHeaders:    nil,
			Parse:         `{}`,
		},
	}
	for _, u := range rates {
		var value map[string]interface{}
		err := json.Unmarshal([]byte(u.Parse), &value)
		c.Assert(err, IsNil)
		add, remove, err := AddRemoveHeadersFromDict(value)
		c.Assert(err, IsNil)
		c.Assert(u.AddHeaders, DeepEquals, add)
		c.Assert(u.RemoveHeaders, DeepEquals, remove)
	}
}

func (s *HeadersSuite) TestAddRemoveHeadersFailure(c *C) {
	rates := []struct {
		Parse string
	}{
		{
			Parse: `{"add-headers": "invalid-format", "remove-headers": {"f": "k"}}`,
		},
		{
			Parse: `{"add-headers": {}, "add-headers": ["invalid", "format"]}`,
		},
	}
	for _, u := range rates {
		var value map[string]interface{}
		err := json.Unmarshal([]byte(u.Parse), &value)
		c.Assert(err, IsNil)
		_, _, err = AddRemoveHeadersFromDict(value)
		c.Assert(err, Not(IsNil))
	}
}
