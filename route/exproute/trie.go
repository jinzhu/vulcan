package exproute

import (
	"fmt"
	"github.com/mailgun/vulcan/location"
	"github.com/mailgun/vulcan/request"
	"regexp"
	"strings"
)

var reParam *regexp.Regexp

func init() {
	reParam = regexp.MustCompile("^<([^/]+)>")
}

type trie struct {
	root *trieNode
}

func parseTrie(pattern string, matchNode node) (*trie, error) {
	t := &trie{
		root: &trieNode{},
	}
	if len(pattern) == 0 {
		return nil, fmt.Errorf("Empty pattern")
	}
	err := t.root.parseExpression(-1, pattern, matchNode)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (t *trie) canMerge(n node) bool {
	_, ok := n.(*trie)
	return ok
}

func (p *trie) merge(n node) (node, error) {
	other, ok := n.(*trie)
	if !ok {
		return nil, fmt.Errorf("Can't merge %T and %T")
	}
	root, err := p.root.merge(other.root)
	if err != nil {
		return nil, err
	}
	return &trie{root: root}, nil
}

func (p *trie) match(r request.Request) location.Location {
	if p.root == nil {
		return nil
	}

	path := r.GetHttpRequest().URL.Path
	if len(path) == 0 {
		path = "/"
	}
	return p.root.match(-1, path, r)
}

type trieNode struct {
	// Matching character, can be empty
	char byte
	// Optional children of this node
	children []*trieNode
	// If present, means that this node is a pattern matcher
	matcher patternMatcher
	// If present it means this node contains potential match
	result node
}

func (e *trieNode) isLeaf() bool {
	return e.result != nil
}

func (e *trieNode) isRoot() bool {
	return e.char == byte(0) && e.matcher == nil
}

func (e *trieNode) isPatternMatcher() bool {
	return e.matcher != nil
}

func (e *trieNode) String() string {
	self := ""
	if e.matcher != nil {
		self = e.matcher.String()
	} else {
		self = fmt.Sprintf("%c", e.char)
	}
	if e.isLeaf() {
		return fmt.Sprintf("leaf(%s)", self)
	} else if e.isRoot() {
		return fmt.Sprintf("root")
	} else {
		return fmt.Sprintf("node(%s)", self)
	}
}

func (e *trieNode) equals(o *trieNode) bool {
	return (e.char == o.char) &&
		(e.matcher == nil && o.matcher == nil) || // both nodes have no matchers
		((e.matcher != nil && o.matcher != nil) && e.matcher.equals(o.matcher)) // both nodes have equal matchers
}

func (e *trieNode) merge(o *trieNode) (*trieNode, error) {
	if e.char != o.char {
		return nil, fmt.Errorf("Can't merge nodes with different keys: %s and %s", e.char, o.char)
	}

	if e.isLeaf() && o.isLeaf() {
		return nil, fmt.Errorf("Can't merge two leaf nodes: %s and %s", e.String(), o.String())
	}

	if e.isLeaf() {
		return mergeWithLeaf(o, e)
	}

	if o.isLeaf() {
		return mergeWithLeaf(e, o)
	}

	children := make([]*trieNode, 0, len(e.children))
	merged := make(map[byte]bool)

	// First, find the nodes with similar keys and merge them
	for _, c := range e.children {
		for _, c2 := range o.children {
			// The nodes are equivalent, so we can merge them
			if c.equals(c2) {
				m, err := c.merge(c2)
				if err != nil {
					return nil, err
				}
				merged[c.char] = true
				children = append(children, m)
			}
		}
	}

	// Next, append the keys that haven't been merged
	for _, c := range e.children {
		if !merged[c.char] {
			children = append(children, c)
		}
	}

	for _, c := range o.children {
		if !merged[c.char] {
			children = append(children, c)
		}
	}

	return &trieNode{char: e.char, children: children, matcher: e.matcher}, nil
}

func (p *trieNode) parseExpression(offset int, pattern string, result node) error {
	// We are the last element, so we are the matching node
	if offset >= len(pattern)-1 {
		p.result = result
		return nil
	}

	// There's a next character that exists
	matcher, newOffset, err := parsePatternMatcher(offset+1, pattern)
	// We have found the matcher, but the syntax or parameters are wrong
	if err != nil {
		return err
	}
	// Matcher was found
	if matcher != nil {
		node := &trieNode{matcher: matcher}
		p.children = []*trieNode{node}
		return node.parseExpression(newOffset-1, pattern, result)
	} else {
		// Matcher was not found, next node is just a character
		node := &trieNode{char: pattern[offset+1]}
		p.children = []*trieNode{node}
		return node.parseExpression(offset+1, pattern, result)
	}
}

func mergeWithLeaf(base *trieNode, leaf *trieNode) (*trieNode, error) {
	n := &trieNode{
		char:     base.char,
		children: make([]*trieNode, len(base.children)),
		matcher:  base.matcher,
	}
	copy(n.children, base.children)
	n.result = leaf.result
	return n, nil
}

func parsePatternMatcher(offset int, pattern string) (patternMatcher, int, error) {
	if pattern[offset] != '<' {
		return nil, -1, nil
	}
	rest := pattern[offset:]
	match := reParam.FindStringSubmatchIndex(rest)
	if len(match) == 0 {
		return nil, -1, nil
	}
	// Split parsed matcher parameters separated by :
	values := strings.Split(rest[match[2]:match[3]], ":")

	// The common syntax is <matcherType:matcherArg1:matcherArg2>
	matcherType := values[0]
	matcherArgs := values[1:]

	// In case if there's only one  <param> is implicitly converted to <string:param>
	if len(values) == 1 {
		matcherType = "string"
		matcherArgs = values
	}

	matcher, err := makePathMatcher(matcherType, matcherArgs)
	if err != nil {
		return nil, offset, err
	}
	return matcher, offset + match[1], nil
}

type matchResult struct {
	matcher patternMatcher
	value   interface{}
}

type patternMatcher interface {
	getName() string
	match(offset int, path string) (*matchResult, int)
	equals(other patternMatcher) bool
	String() string
}

func makePathMatcher(matcherType string, matcherArgs []string) (patternMatcher, error) {
	switch matcherType {
	case "string":
		return newStringMatcher(matcherArgs)
	}
	return nil, fmt.Errorf("Unsupported matcher: %s", matcherType)
}

func newStringMatcher(args []string) (patternMatcher, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("Expected only one parameter - variable name, got %s", args)
	}
	return &stringMatcher{name: args[0]}, nil
}

type stringMatcher struct {
	name string
}

func (s *stringMatcher) String() string {
	return fmt.Sprintf("<string:%s>", s.name)
}

func (s *stringMatcher) getName() string {
	return s.name
}

func (s *stringMatcher) match(offset int, path string) (*matchResult, int) {
	value, offset := grabValue(offset, path)
	return &matchResult{matcher: s, value: value}, offset
}

func (s *stringMatcher) equals(other patternMatcher) bool {
	_, ok := other.(*stringMatcher)
	return ok && other.getName() == s.getName()
}

// Grabs value until separator or next string
func grabValue(offset int, path string) (string, int) {
	index := strings.Index(path[offset:], "/")
	if index == -1 {
		return path, offset + len(path)
	}
	return path[offset:index], offset + index
}

func (e *trieNode) match(offset int, path string, r request.Request) location.Location {
	// We are the root or the current key matches
	if offset == -1 || path[offset] == e.char {
		// This is a leaf node and we are at the last character of the pattern
		if e.result != nil && offset == len(path)-1 {
			return e.result.match(r)
		}
		// Check for the match in child nodes
		for _, c := range e.children {
			if loc := c.match(offset+1, path, r); loc != nil {
				return loc
			}
		}
	}
	return nil
}
