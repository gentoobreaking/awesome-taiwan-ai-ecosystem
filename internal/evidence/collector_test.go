package evidence

import (
	"testing"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

func TestCollectorAdd(t *testing.T) {
	c := New()
	ev := models.Evidence{
		Type:        "official_domain",
		Source:      "repository",
		Location:    "twse.com.tw",
		MatchedText: "twse.com.tw",
		Rule:        "official_domain_match",
		Score:       40,
		Confidence:  1.0,
	}
	c.Add(ev)
	if c.Len() != 1 {
		t.Errorf("Expected 1 evidence item, got %d", c.Len())
	}
	items := c.All()
	if items[0].Type != "official_domain" {
		t.Errorf("Expected type=official_domain, got %s", items[0].Type)
	}
}

func TestCollectorByType(t *testing.T) {
	c := New()
	c.Add(models.Evidence{Type: "official_domain", Score: 40})
	c.Add(models.Evidence{Type: "repository_keyword", Score: 20})

	domainEv := c.ByType("official_domain")
	if len(domainEv) != 1 {
		t.Errorf("Expected 1 domain evidence, got %d", len(domainEv))
	}

	keywordEv := c.ByType("repository_keyword")
	if len(keywordEv) != 1 {
		t.Errorf("Expected 1 keyword evidence, got %d", len(keywordEv))
	}
}

func TestCollectorAutoTimestamp(t *testing.T) {
	c := New()
	ev := models.Evidence{Type: "test", Score: 10}
	before := time.Now().UTC()
	c.Add(ev)
	after := time.Now().UTC()

	items := c.All()
	ts := items[0].Timestamp
	if ts.Before(before) || ts.After(after) {
		t.Error("Expected auto-set timestamp to be within expected window")
	}
}
