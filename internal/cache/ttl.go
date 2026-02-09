package cache

import "time"

func (c *OrgCache) SetTTL(ttl time.Duration) {
	c.ttl = ttl
}
